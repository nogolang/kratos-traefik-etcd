package etcdUtils

import (
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/pkg/errors"
	EtcdClientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"golang.org/x/net/context"
)

const (
	TraefikRuleName = "traefik/http/services/ServiceNameReplace/loadbalancer/servers/NumReplace/url"
)

type EtcdTraefik struct {
	client      *EtcdClientv3.Client
	ServiceName string

	//服务的序号
	ServiceNum int

	//kratos服务的租约id
	LeaseId EtcdClientv3.LeaseID
}

func NewEtcdTraefik(client *EtcdClientv3.Client, serviceName string) *EtcdTraefik {
	return &EtcdTraefik{client: client, ServiceName: serviceName}
}

func (receiver *EtcdTraefik) RegisterTraefik(app kratos.AppInfo) error {
	//每个协程都需要创建一个lock对象
	session, _ := concurrency.NewSession(receiver.client,
		concurrency.WithTTL(15),
	)
	defer session.Close()

	//创建锁
	locker := concurrency.NewMutex(session, "/lockRegisterNum/"+receiver.ServiceName)
	timeout, _ := context.WithTimeout(context.Background(), time.Second*60)
	err := locker.Lock(timeout)
	if err != nil {
		log.Fatal("超时启动，请重新启动\n")
		return err
	}

	//序号
	numPrefix := "/register/num"

	res, err := receiver.client.Get(context.Background(), numPrefix, EtcdClientv3.WithPrefix())
	if err != nil {
		return errors.Wrap(err, "获取序号失败")
	}

	var num int
	if len(res.Kvs) == 0 {
		receiver.ServiceNum = 1
	} else {
		num, err = strconv.Atoi(string(res.Kvs[0].Value))
		if err != nil {
			return errors.Wrap(err, "转换序号失败")
		}
	}

	//如果已经到了1023，那么就重置
	if num == 1023 {
		receiver.ServiceNum = 1
	} else {
		receiver.ServiceNum = num + 1
	}

	//设置回去
	_, err = receiver.client.Put(timeout, numPrefix, strconv.Itoa(receiver.ServiceNum))
	if err != nil {
		return errors.Wrap(err, "设置序号失败")
	}
	err = locker.Unlock(timeout)
	if err != nil {
		log.Fatal("解锁失败，程序退出异常\n")
		return err
	}

	//获取租约id，获取当前服务的租约ID，所以我们要把当前服务的id传递过来
	//这是kratos的服务，并不是我们自己的
	//获取kratos服务的数量
	kratosPrefix := "/microservices/"
	prefix := kratosPrefix + receiver.ServiceName
	getLeaseId, err := receiver.client.Get(context.Background(), prefix+"/"+app.ID(), EtcdClientv3.WithPrefix())
	if err != nil {
		return err
	}
	receiver.LeaseId = EtcdClientv3.LeaseID(getLeaseId.Kvs[0].Lease)

	//获取endpoint端口地址，有可能只注册了一个http或者一个grpc
	//如果只有1个，那么里面需要判断是grpc还是http
	//如果有多个，第1个是grpc，第2个是http
	endpoint := app.Endpoint()
	if len(endpoint) == 1 {
		err := receiver.registerOneProtocol(endpoint)
		if err != nil {
			return err
		}
	} else {
		err := receiver.registerManyProtocol(endpoint)
		if err != nil {
			return err
		}
	}
	return nil
}

func (receiver *EtcdTraefik) registerOneProtocol(endpoints []string) error {

	address := endpoints[0]

	//grpc开头的，则只注册grpc，http则只注册http
	//这里是只注册一个服务的情况
	var k, v string
	if strings.HasPrefix(address, "grpc://") {
		k, v = receiver.getGrpcRegisterKv(endpoints[0])

	} else {
		k, v = receiver.getHttpRegisterKv(endpoints[0])
	}

	_, err := receiver.client.Put(context.Background(),
		k, v,
		EtcdClientv3.WithLease(receiver.LeaseId))
	if err != nil {
		return err
	}
	return nil
}

func (receiver *EtcdTraefik) registerManyProtocol(endpoints []string) error {
	//注册多个Protocol，则endpoints第1个为grpc，第2个是 http
	grpcAddress := endpoints[0]
	httpAddress := endpoints[1]

	httpK, httpV := receiver.getHttpRegisterKv(httpAddress)
	_, err := receiver.client.Put(context.Background(),
		httpK, httpV,
		EtcdClientv3.WithLease(receiver.LeaseId))

	if err != nil {
		return err
	}
	grpcK, grpcV := receiver.getGrpcRegisterKv(grpcAddress)
	_, err = receiver.client.Put(context.Background(), grpcK, grpcV, EtcdClientv3.WithLease(receiver.LeaseId))
	if err != nil {
		return err
	}
	return nil
}

// 获取http注册key和value
func (receiver *EtcdTraefik) getHttpRegisterKv(address string) (string, string) {
	//替换掉数量,这里要-1，从0开始
	replace := strings.Replace(TraefikRuleName, "NumReplace", strconv.Itoa(receiver.ServiceNum), -1)

	//替换掉名称
	replace = strings.Replace(replace, "ServiceNameReplace", receiver.ServiceName, -1)

	return replace, address
}

func (receiver *EtcdTraefik) getGrpcRegisterKv(address string) (string, string) {
	//替换掉数量,这里要-1，从0开始
	replace := strings.Replace(TraefikRuleName, "NumReplace", strconv.Itoa(receiver.ServiceNum), -1)

	//替换掉名称,我们的服务名称是固定形式，如果是grpc的服务，则会加上grpc，比如 user-service-grpc
	//这个是注册到traefik里的，用于区分
	replace = strings.Replace(replace, "ServiceNameReplace", receiver.ServiceName+"-grpc", -1)

	//要把kratos里的内容替换掉
	newAddress := strings.Replace(address, "grpc://", "h2c://", -1)

	return replace, newAddress
}

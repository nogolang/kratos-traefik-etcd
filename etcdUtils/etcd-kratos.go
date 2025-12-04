package etcdUtils

import (
	"strconv"
	"strings"

	"github.com/go-kratos/kratos/v2"
	EtcdClientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/net/context"
)

const (
	TraefikRuleName = "traefik/http/services/ServiceNameReplace/loadbalancer/servers/NumReplace/url"
)

type EtcdTraefik struct {
	client      *EtcdClientv3.Client
	ServiceName string

	//服务的序号
	NodeNum int

	//kratos服务的租约id
	LeaseId EtcdClientv3.LeaseID
}

func NewEtcdTraefik(client *EtcdClientv3.Client, serviceName string, nodeNum int) *EtcdTraefik {
	return &EtcdTraefik{client: client, ServiceName: serviceName, NodeNum: nodeNum}
}

func (receiver *EtcdTraefik) RegisterTraefik(app kratos.AppInfo) error {
	//获取租约id，给traefik kv 也用一样的租约，这样就能一起删除
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
	//替换掉数量
	replace := strings.Replace(TraefikRuleName, "NumReplace", strconv.Itoa(receiver.NodeNum), -1)

	//替换掉名称
	replace = strings.Replace(replace, "ServiceNameReplace", receiver.ServiceName, -1)

	return replace, address
}

func (receiver *EtcdTraefik) getGrpcRegisterKv(address string) (string, string) {
	//替换掉数量
	replace := strings.Replace(TraefikRuleName, "NumReplace", strconv.Itoa(receiver.NodeNum), -1)

	//替换掉名称,我们的服务名称是固定形式，如果是grpc的服务，则会加上grpc，比如 user-service-grpc
	//这个是注册到traefik里的，用于区分
	replace = strings.Replace(replace, "ServiceNameReplace", receiver.ServiceName+"-grpc", -1)

	//要把kratos里的内容替换掉
	newAddress := strings.Replace(address, "grpc://", "h2c://", -1)

	return replace, newAddress
}

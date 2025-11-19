

**引入包**
import "github.com/nogolang/kratos-traefik-etcd/etcdUtils"

在kratos的AfterStart钩子里执行即可
此时会向etcd同时注册一个可以被traefik识别的kv，并且租约和kratos注册的kv保持一致，这样程序结束后会自动从etcd里取消kv

```
//创建kratos app
app := kratos.New(
    kratos.Name(receiver.AllConfig.Server.ServerName),
    kratos.Registrar(receiver.KratosEtcdClient),
    //consul
    //kratos.Registrar(receiver.kratosConsulClient),

    //只使用http
    kratos.Server(receiver.HttpServer),
    kratos.AfterStart(func(ctx context.Context) error {
        appInfo, ok := kratos.FromContext(ctx)
        if !ok {
            receiver.Logger.Fatal("获取kratos app失败")
            return fmt.Errorf("获取kratos app失败")
        }
        //让traefik识别到
        traefikEtcd := etcdUtils.NewEtcdTraefik(receiver.EtcdClient, receiver.AllConfig.Server.ServerName)
        err := traefikEtcd.RegisterTraefik(appInfo)
        if err != nil {
            receiver.Logger.Fatal("注册到etcd-traefik里失败：", zap.Error(err))
            return err
        }
        return nil
    }),
)
```


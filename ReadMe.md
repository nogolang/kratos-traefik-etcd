

**引入包**
import "github.com/nogolang/kratos-traefik-etcd/etcdUtils"


在执行完kratos的的run方法之后(注意用协程)使用etcdUtils
此时会向etcd同时注册一个可以被traefik识别的kv，并且租约和kratos注册的kv保持一致，这样程序结束后会自动从etcd里取消kv

```
go func() {
		err := app.Run()
		if err != nil {
			receiver.Logger.Fatal("kratos服务启动失败：", zap.Error(err))
			return
		}
}()

traefikEtcd := etcdUtils.NewEtcdTraefik(receiver.EtcdClient, receiver.AllConfig.Server.ServerName)
go func(a *kratos.App) {
    for {
        time.Sleep(500 * time.Millisecond)
        if a.ID() != "" {
            err := traefikEtcd.RegisterTraefik(a)
            if err != nil {
                receiver.Logger.Fatal("注册到etcd-traefik里失败：", zap.Error(err))
            }
            //注册成功后直接结束这个协程即可
            return
        }
    }
}(app)
```


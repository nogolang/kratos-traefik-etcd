

**引入包**

go get -u "github.com/nogolang/kratos-traefik-etcd/etcdUtils"



在执行完kratos的的run方法之后(注意用协程)使用etcdUtils

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


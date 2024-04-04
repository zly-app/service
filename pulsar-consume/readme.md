
# pulsar消费服务

> 提供用于 https://github.com/zly-app/zapp 的服务

# 说明

> 此服务基于模块 [github.com/apache/pulsar-client-go](https://github.com/apache/pulsar-client-go)

# 示例

1. 添加配置文件 `configs/default.yml`. 更多配置参考[这里](./config.go)

```yaml
services:
  pulsar-consume:
    t1: # 注册名
      config:
        url: pulsar://localhost:6650
        ListenerName: external # 监听器名称, 这里采用 external 以实现内外网分流. 参考 https://pulsar.apache.org/docs/3.2.x/concepts-multiple-advertised-listeners/
        topics: persistent://public/default/test # 消费topic, 多个topic用英文逗号连接
        subscriptionName: test # 订阅名
        subscriptionType: shared # 订阅类型, 支持 exclusive,failover,shared,keyshared. 默认 shared
```

2. 添加代码

```go
package main

import (
	pulsar_consume "github.com/zly-app/service/pulsar-consume"

	"github.com/zly-app/zapp"
)

func main() {
	app := zapp.NewApp("test",
		pulsar_consume.WithService(), // 启用pulsar消费服务
	)
	defer app.Exit()

	pulsar_consume.RegistryHandler("t1", // 注册handler, 这里的注册名要和配置文件中的一样
		func(ctx *pulsar_consume.Context) error {
			ctx.Info("Payload: ", string(ctx.Msg.Payload()))
			return nil
		})

	app.Run()
}
```

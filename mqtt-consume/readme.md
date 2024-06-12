
# mqtt消费服务

> 提供用于 https://github.com/zly-app/zapp 的服务

# 说明

> 此服务基于模块 [github.com/eclipse/paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang)

# 示例

1. 添加配置文件 `configs/default.yml`. 更多配置参考[这里](./config.go)

```yaml
services:
  mqtt-consume:
    t1: # 注册名
      config:
        Server: localhost:1883 # mqtt服务地址
        WaitConnectedTimeMs: 5000 # 等待连接超时时间, 单位毫秒
        
        User: '' # mqtt用户名
        Password: '' # mqtt密码
        
        topics: test # 消费topic, 多个topic用英文逗号连接. 示例: test/topic/a,test/topic/b,test/topic/+
        Qos: 1 # qos级别, 0最多交付一次, 1至少交付一次, 2仅交付一次
        ClientID: '' # clientID 如果为空则设为 实例id. 如果客户端使用一个重复的 Client ID 连接至服务器，将会把已使用该 Client ID 连接成功的客户端踢下线。
        CleanSession: true # 清除会话, 设为false时, 服务端会为同一个会话的客户端保留一定数量的离线消息, 通常是1000条. 不会保存Qos=0的消息. 部分mqtt服务器不支持这个功能
        
        ConsumeThreadCount: 0 # 消费者协程数, 0表示使用逻辑处理器数量*2
```

2. 添加代码

```go
package main

import (
	"context"

	mqtt_consume "github.com/zly-app/service/mqtt-consume"

	"github.com/zly-app/zapp"
)

func main() {
	app := zapp.NewApp("test",
		mqtt_consume.WithService(), // 启用mqtt消费服务
	)
	defer app.Exit()

	mqtt_consume.RegistryHandler("t1", // 注册handler, 这里的注册名要和配置文件中的一样
		func(ctx context.Context, msg mqtt_consume.Message) error {
			app.Info(ctx, "Payload: ", string(msg.Payload()))
			return nil
		})

	app.Run()
}
```

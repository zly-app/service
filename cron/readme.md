
# cron服务

> 提供用于 https://github.com/zly-app/zapp 的服务

# 说明

```text
cron.WithService()              # 启用服务
cron.RegistryHandler(...)       # 注册handler
cron.RegistryOnceHandler(...)   # 注册一次性handler
cron.RegistryTask(...)          # 注册自定义task
```

# 示例
```go
package main

import (
	"github.com/zly-app/service/cron"
	"github.com/zly-app/zapp"
)

func main() {
	// 启用cron服务
	app := zapp.NewApp("test", cron.WithService())
	// 注册handler
	cron.RegistryHandler("c1", "@every 1s", true, func(ctx cron.IContext) error {
		ctx.Info("触发")
		return nil
	})
	// 运行
	app.Run()
}
```

# 配置

> 这个服务可以不需要配置, 默认服务类型为 `cron`

```yaml
services:
  cron:
    # 线程数, 默认为-1
    ThreadCount = -1
    # 最大任务队列大小, 默认为10000
    MaxTaskQueueSize = 0
```

# 通过配置修改task默认行为

```yaml
services:
  cron:
    # 任务列表
    tasks:
      Name: '' # 任务名, 将会替换代码中相同任务名的默认行为
      Expression: '' # cron表达式, https://en.wikipedia.org/wiki/Cron
      IsOnceTrigger: false # 是否为一次性触发, 如果设为true, 则 Expression 的格式为 YYYY-MM-dd hh:mm:ss
      Disable: false # 是否禁用
      RetryCount: 0 # 任务失败重试次数, 0表示不重试
      RetrySleepMs: 0 # 失败重试等待时间, 单位秒, 0表示不等待
      MaxConcurrentExecuteCount: 1 # 最大并发执行任务数, 如果为-1则不限制. 表示在执行过程中又被调度器触发执行时, 能同时运行同一个任务的数量. 默认1
      TimeoutMs: 0 # 超时时间, 单位秒, 0表示永不超时
```

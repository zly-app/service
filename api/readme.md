
# api服务

> 提供用于 https://github.com/zly-app/zapp 的服务

# 说明

> 此组件基于模块 [github.com/kataras/iris/v12](https://github.com/kataras/iris)

```text
api.RegistryService()           # 注册服务
api.WithApiService()            # 启用服务
api.RegistryApiRouter(...)      # 服务注入(注册路由)
```

# 示例

```go
package main

import (
	"github.com/zly-app/service/api"

	"github.com/zly-app/zapp"
	"github.com/zly-app/zapp/core"
)

func main() {
	// 注册api服务
	api.RegistryService()
	// 启用api服务
	app := zapp.NewApp("test", api.WithApiService())
	// 注册路由
	api.RegistryApiRouter(app, func(c core.IComponent, router api.Party) {
		router.Get("/", api.Wrap(func(ctx *api.Context) interface{} {
			return "hello"
		}))
	})
	// 运行
	app.Run()
}
```

# 配置

> 默认服务类型为 `api`

```toml
[services.api]
# bind地址
Bind=":8080"
# 适配nginx的Forwarded获取ip, 优先级高于nginx的Real
IPWithNginxForwarded=true
# 适配nginx的Real获取ip, 优先级高于sock连接的ip
IPWithNginxReal=true
# 在开发环境中显示api结果
ShowApiResultInDevelop=true
# 在生产环境显示详细的错误
ShowDetailedErrorInProduction=false
```

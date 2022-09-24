
# api服务

> 提供用于 https://github.com/zly-app/zapp 的服务

> 此组件基于模块 [github.com/kataras/iris/v12](https://github.com/kataras/iris)

<!-- TOC -->

- [示例](#%E7%A4%BA%E4%BE%8B)
- [配置](#%E9%85%8D%E7%BD%AE)
- [校验器](#%E6%A0%A1%E9%AA%8C%E5%99%A8)
- [包装处理程序Wrap](#%E5%8C%85%E8%A3%85%E5%A4%84%E7%90%86%E7%A8%8B%E5%BA%8Fwrap)
    - [api.Wrap支持的函数指纹](#apiwrap%E6%94%AF%E6%8C%81%E7%9A%84%E5%87%BD%E6%95%B0%E6%8C%87%E7%BA%B9)

<!-- /TOC -->

---

# 示例

```go
package main

import (
	"github.com/zly-app/service/api"
	"github.com/zly-app/zapp"
	"github.com/zly-app/zapp/core"
)

func main() {
	// 启用api服务
	app := zapp.NewApp("test", api.WithService())
	// 注册路由
	api.RegistryRouter(func(c core.IComponent, router api.Party) {
		router.Get("/", api.Wrap(func(ctx *api.Context) interface{} {
			return "hello"
		}))
	})
	// 运行
	app.Run()
}
```

# 配置

下面所有配置字段都是可选的, 默认服务类型为`api`.

```yml
services:
  api:
    # bind地址
    Bind: ':8080'
    # 适配nginx的Forwarded获取ip, 优先级高于nginx的Real
    IPWithNginxForwarded: true
    # 适配nginx的Real获取ip, 优先级高于sock连接的ip
    IPWithNginxReal: true
    # post允许客户端传输最大数据大小, 单位字节, 默认32M
    PostMaxMemory: 33554432
    # 同时处理请求的goroutine数, 设为0时取逻辑cpu数*2, 设为负数时不作任何限制, 每个请求由独立的线程执行
    ThreadCount: 0
    # 最大请求等待队列大小
    # 
    # 只有 ThreadCount >= 0 时生效.
    # 启动时创建一个指定大小的任务队列, 触发产生的请求会放入这个队列, 队列已满时新触发的请求会返回错误
    MaxReqWaitQueueSize: 10000
    # 请求日志等级设为info
    ReqLogLevelIsInfo: true
    # 响应日志等级设为info
    RspLogLevelIsInfo: true
    # bind日志等级设为info
    BindLogLevelIsInfo: true
    # 在开发环境中输出api结果
    LogApiResultInDevelop: true
    # 在生产环境中输出api结果
    LogApiResultInProd: true
    # 在生产环境发送详细的错误到客户端
    SendDetailedErrorInProduction: false
    # 总是输出headers日志, 如果设为false, 只会在出现错误时才会输出headers日志
    AlwaysLogHeaders: true
    # 总是输出body日志, 如果设为false, 只会在出现错误时才会输出body日志
    AlwaysLogBody: true
    # 日志输出结果最大大小，默认64k
    LogApiResultMaxSize: 65536
    # 日志输出body最大大小，默认64k
    LogBodyMaxSize: 65536
```

# 校验器

+ 使用 [github.com/go-playground/validator/v10](https://github.com/go-playground/validator) 校验器
+ 校验器tag由`validate`改为`bind`
+ 添加了`regex`,`time`,`date`校验方法

# 包装处理程序(api.Wrap)

router传入的处理程序最好经过`api.Wrap`包装, `api.Wrap`实现了很多功能让开发者能专注于业务

## 函数指纹说明

+ 入参
  > 第一个入参必须是 *api.Context 类型<br>
  > 如果有第二个入参必须是 struct<br>
  > 第二个入参可以是指针<br>
  > 第二个入参会智能选择从url或body中读取参数并校验

+ 出参
  > 第一个出参可以是任何类型<br>
  > 如果有第二个出参必须是error类型
  
+ 示例

    ```go
    func (ctx *api.Context) interface{}
    func (ctx *api.Context) error
    func (ctx *api.Context, req *AnyReqStruct) interface{}
    func (ctx *api.Context, req *AnyReqStruct) error
    func (ctx *api.Context, req *AnyReqStruct) (interface{}, error)
    func (ctx *api.Context, req *AnyReqStruct) (*AnyOutStruct, error)
    ```

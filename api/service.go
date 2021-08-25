/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2021/1/21
   Description :
-------------------------------------------------
*/

package api

import (
	"context"

	"github.com/iris-contrib/middleware/cors"
	"github.com/kataras/iris/v12"
	"github.com/zly-app/zapp/core"
	"go.uber.org/zap"

	"github.com/zly-app/service/api/config"
	"github.com/zly-app/service/api/middleware"
)

type Party = iris.Party

// api注入函数定义
type RegisterApiRouterFunc = func(c core.IComponent, router Party)

type ApiService struct {
	app core.IApp
	*iris.Application
}

func NewHttpService(app core.IApp, opts ...Option) core.IService {
	// 处理选项
	o := newOptions(opts...)

	// irisApp
	irisApp := iris.New()
	irisApp.Logger().SetLevel("disable") // 关闭默认日志
	irisApp.Use(
		middleware.LoggerMiddleware(app),
		cors.AllowAll(),
		middleware.Recover(),
	)
	irisApp.AllowMethods(iris.MethodOptions)

	// 配置项
	irisApp.Configure(o.Configurator...)

	// 中间件
	for _, fn := range o.Middlewares {
		irisApp.Use(WrapMiddleware(fn))
	}

	return &ApiService{app: app, Application: irisApp}
}

func (a *ApiService) Inject(sc ...interface{}) {
	for _, h := range sc {
		fn, ok := h.(RegisterApiRouterFunc)
		if !ok {
			a.app.Fatal("api服务注入类型错误, 它必须能转为 api.RegisterApiRouterFunc")
		}

		fn(a.app.GetComponent(), a.Party("/"))
	}
}

func (a *ApiService) Start() error {
	conf := config.NewConfig()
	err := a.app.GetConfig().ParseServiceConfig(nowServiceType, conf)
	if err != nil {
		return err
	}
	conf.Check()
	config.Conf = *conf

	a.app.Debug("正在启动api服务", zap.String("bind", conf.Bind))
	opts := []iris.Configurator{
		iris.WithoutBodyConsumptionOnUnmarshal, // 重复消费
		iris.WithoutPathCorrection,             // 不自动补全斜杠
		iris.WithOptimizations,                 // 启用性能优化
		iris.WithoutStartupLog,                 // 不要打印iris启动信息
		iris.WithPathEscape,                    // 解析path转义
		iris.WithFireMethodNotAllowed,          // 路由未找到时返回405而不是404
		iris.WithConfiguration(iris.Configuration{RemoteAddrHeadersForce: conf.UseFirstValidRemoteAddrOfHeaders}),
		iris.WithPostMaxMemory(conf.PostMaxMemory), // post允许客户端传输最大数据大小
	}
	if conf.IPWithNginxForwarded {
		opts = append(opts, iris.WithRemoteAddrHeader("X-Forwarded-For"))
	}
	if conf.IPWithNginxReal {
		opts = append(opts, iris.WithRemoteAddrHeader("X-Real-IP"))
	}
	return a.Run(iris.Addr(conf.Bind), opts...)
}

func (a *ApiService) Close() error {
	err := a.Shutdown(context.Background())
	a.app.Debug("api服务已关闭")
	return err
}

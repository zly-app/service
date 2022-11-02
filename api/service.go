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
	"errors"

	"github.com/iris-contrib/middleware/cors"
	"github.com/kataras/iris/v12"
	"github.com/zly-app/zapp"
	"github.com/zly-app/zapp/component/gpool"
	"github.com/zly-app/zapp/core"
	"go.uber.org/zap"

	"github.com/zly-app/service/api/config"
	"github.com/zly-app/service/api/middleware"
)

type Party = iris.Party

// api注入函数定义
type RegisterApiRouterFunc = func(c core.IComponent, router Party)

type ApiService struct {
	app  core.IApp
	conf *config.Config
	*iris.Application
}

// 协程池限制
func GPoolLimitMiddleware(app core.IApp, conf *config.Config) func(ctx *Context) error {
	pool := gpool.NewGPool(&gpool.GPoolConfig{
		JobQueueSize: conf.MaxReqWaitQueueSize,
		ThreadCount:  conf.ThreadCount,
	})
	return func(ctx *Context) error {
		err, ok := pool.TryGoSync(func() error {
			ctx.Next()
			return nil
		})
		if !ok {
			return errors.New("gPool Limit")
		}
		return err
	}
}

func NewApiService(app core.IApp, conf *config.Config, opts ...Option) *ApiService {
	// 处理选项
	o := newOptions(opts...)

	// irisApp
	irisApp := iris.New()
	irisApp.Logger().SetLevel("disable") // 关闭默认日志
	irisApp.Use(
		middleware.BaseMiddleware(app, conf),
		middleware.LoggerMiddleware(app, conf),          // 日志
		WrapMiddleware(GPoolLimitMiddleware(app, conf)), // 协程池限制
		cors.AllowAll(),
		middleware.Recover(), // panic恢复
	)
	irisApp.AllowMethods(iris.MethodOptions)

	// 配置项
	irisApp.Configure(o.Configurator...)

	// 中间件
	for _, fn := range o.Middlewares {
		irisApp.Use(WrapMiddleware(fn))
	}

	// 在app关闭前优雅的关闭服务
	zapp.AddHandler(zapp.BeforeExitHandler, func(app core.IApp, handlerType zapp.HandlerType) {
		err := irisApp.Shutdown(context.Background())
		if err != nil {
			app.Error("irisApp关闭失败", zap.Error(err))
			return
		}
		app.Warn("api服务已关闭")
	})

	return &ApiService{
		app:         app,
		conf:        conf,
		Application: irisApp,
	}
}

func (a *ApiService) Start() error {
	a.app.Info("正在启动api服务", zap.String("bind", a.conf.Bind))
	opts := []iris.Configurator{
		iris.WithoutBodyConsumptionOnUnmarshal,       // 重复消费
		iris.WithoutPathCorrection,                   // 不自动补全斜杠
		iris.WithOptimizations,                       // 启用性能优化
		iris.WithoutStartupLog,                       // 不要打印iris启动信息
		iris.WithPathEscape,                          // 解析path转义
		iris.WithFireMethodNotAllowed,                // 路由未找到时返回405而不是404
		iris.WithPostMaxMemory(a.conf.PostMaxMemory), // post允许客户端传输最大数据大小
	}
	if a.conf.IPWithIngressForwarded {
		opts = append(opts, iris.WithRemoteAddrHeader("X-Original-Forwarded-For"))
	}
	if a.conf.IPWithProxyForwarded {
		opts = append(opts, iris.WithRemoteAddrHeader("X-Forwarded-For"))
	}
	if a.conf.IPWithProxyReal {
		opts = append(opts, iris.WithRemoteAddrHeader("X-Real-IP"))
	}
	return a.Run(iris.Addr(a.conf.Bind), opts...)
}

// 注册路由
func (a *ApiService) RegistryRouter(fn ...RegisterApiRouterFunc) {
	for _, h := range fn {
		h(a.app.GetComponent(), a.Party("/"))
	}
}

func (a *ApiService) Close() error {
	return nil
}

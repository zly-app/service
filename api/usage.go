/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2021/1/21
   Description :
-------------------------------------------------
*/

package api

import (
	"github.com/zly-app/zapp"
	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/service"
	"go.uber.org/zap"

	"github.com/zly-app/service/api/config"
)

// DefaultServiceType 默认服务类型
const DefaultServiceType core.ServiceType = "api"

// WithService 启用 api 服务，必须在 zapp 初始化前调用。
func WithService(opts ...Option) zapp.Option {
	service.RegisterCreatorFunc(DefaultServiceType, func(app core.IApp) core.IService {
		return newService(app, opts...)
	})
	return zapp.WithService(DefaultServiceType)
}

// Install 返回服务注册器，必须在 zapp 初始化后调用（例如 app.Run 之前）。
func Install(app core.IApp) *Registrar {
	return &Registrar{app: app}
}

// Registrar 是 api 服务注册器：拥有「向 api 服务注入路由」这条规则。
type Registrar struct {
	app core.IApp
}

// RegistryRouter 注册路由
func (r *Registrar) RegistryRouter(fn ...RegisterApiRouterFunc) {
	a := make([]interface{}, len(fn))
	for i, h := range fn {
		a[i] = h
	}
	r.app.InjectService(DefaultServiceType, a...)
}

type Service struct {
	app core.IApp
	api *ApiService
}

func (s *Service) Inject(a ...interface{}) {
	for _, h := range a {
		fn, ok := h.(RegisterApiRouterFunc)
		if !ok {
			s.app.Fatal("api服务注入类型错误, 它必须能转为 api.RegisterApiRouterFunc")
		}

		s.api.RegistryRouter(fn)
	}
}

func (s *Service) Start() error { return s.api.Start() }

func (s *Service) Close() error { return s.api.Close() }

func newService(app core.IApp, opts ...Option) *Service {
	conf := config.NewConfig()
	err := app.GetConfig().ParseServiceConfig(DefaultServiceType, conf, true)
	if err != nil {
		app.Fatal("获取api服务配置失败", zap.Error(err))
	}
	conf.Check()

	api := NewApiService(app, conf, opts...)
	return &Service{
		app: app,
		api: api,
	}
}

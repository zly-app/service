package mqtt_consume

import (
	"sync"

	"github.com/zly-app/zapp"
	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/logger"
	"github.com/zly-app/zapp/service"
	"go.uber.org/zap"
)

const DefaultServiceType = "mqtt-consume"

func init() {
	service.RegisterCreatorFunc(DefaultServiceType, func(app core.IApp) core.IService {
		return NewServiceAdapter(app)
	})
}

func WithService() zapp.Option {
	return zapp.WithService(DefaultServiceType)
}

// 注册handler
func RegistryHandler(consumeName string, handlers ...ConsumerHandler) {
	zapp.App().InjectService(DefaultServiceType, serviceAdapterInjectData{
		ConsumeName: consumeName,
		Handlers:    handlers,
	})
}

// 服务适配器注入数据
type serviceAdapterInjectData struct {
	ConsumeName string
	Handlers    []ConsumerHandler
}

type serviceAdapterConfig struct {
	Config
	Disable bool // 是否关闭
}

type ServiceAdapter struct {
	app      core.IApp
	services map[string]*MQTTConsumeService
}

func (s *ServiceAdapter) Inject(a ...interface{}) {
	for _, v := range a {
		data, ok := v.(serviceAdapterInjectData)
		if !ok {
			s.app.Fatal("mqtt消费服务注入类型错误, 它必须能转为 *mqtt_consume.serviceAdapterInjectData")
		}

		ss, ok := s.services[data.ConsumeName]
		if !ok {
			s.app.Fatal("mqtt消费服务注入参数错误, 未定义的消费者名", zap.String("ConsumeName", data.ConsumeName))
		}
		if ss == nil {
			continue
		}
		ss.RegistryHandler(data.Handlers...)
	}
}

func (s *ServiceAdapter) Start() error {
	var wg sync.WaitGroup
	wg.Add(len(s.services))
	for name, ss := range s.services {
		if ss == nil {
			wg.Done()
			continue
		}
		go func(name string, ss *MQTTConsumeService) {
			err := ss.Start()
			if err != nil {
				s.app.Fatal("mqtt消费者启动失败", zap.String("name", name), zap.Error(err))
			}
			wg.Done()
		}(name, ss)
	}
	wg.Wait()
	return nil
}

func (s *ServiceAdapter) Close() error {
	var wg sync.WaitGroup
	wg.Add(len(s.services))
	for _, ss := range s.services {
		if ss == nil {
			wg.Done()
			continue
		}
		go func(ss *MQTTConsumeService) {
			ss.Close()
			wg.Done()
		}(ss)
	}
	wg.Wait()
	return nil
}

func NewServiceAdapter(app core.IApp) core.IService {
	consumersConf := make(map[string]interface{})
	err := app.GetConfig().ParseServiceConfig(DefaultServiceType, &consumersConf)
	if err != nil {
		logger.Log.Panic("服务配置错误", zap.String("serviceType", string(DefaultServiceType)), zap.Error(err))
	}

	services := make(map[string]*MQTTConsumeService, len(consumersConf))
	for name := range consumersConf {
		conf := &serviceAdapterConfig{
			Config: *NewConfig(),
		}
		err = app.GetConfig().ParseServiceConfig(DefaultServiceType+"."+core.ServiceType(name), conf)
		if err != nil {
			logger.Log.Panic("服务配置错误", zap.String("serviceType", string(DefaultServiceType)), zap.String("name", name), zap.Error(err))
		}
		if conf.Disable {
			services[name] = nil
			continue
		}
		s, err := NewConsumeService(name, app, &conf.Config)
		if err != nil {
			logger.Log.Panic("创建服务失败", zap.String("serviceType", string(DefaultServiceType)), zap.String("name", name), zap.Error(err))
		}
		services[name] = s
		logger.Log.Info("启动mqtt消费服务ok", zap.String("name", name))
	}

	return &ServiceAdapter{
		app:      app,
		services: services,
	}
}

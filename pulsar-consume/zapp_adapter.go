package pulsar_consume

import (
	"sync"

	"github.com/zly-app/zapp"
	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/logger"
	"github.com/zly-app/zapp/service"
	"go.uber.org/zap"
)

// 默认服务类型
const DefaultServiceType core.ServiceType = "pulsar-consume"

// 当前服务类型
var nowServiceType = DefaultServiceType

// 设置服务类型, 这个函数应该在 zapp.NewApp 之前调用
func SetServiceType(t core.ServiceType) {
	nowServiceType = t
}

// 启用pulsar-consume服务
func WithService() zapp.Option {
	service.RegisterCreatorFunc(nowServiceType, func(app core.IApp) core.IService {
		return NewServiceAdapter(app)
	})
	return zapp.WithService(nowServiceType)
}

// 注册handler
func RegistryHandler(consumeName string, handlers ...ConsumerHandler) {
	zapp.App().InjectService(nowServiceType, serviceAdapterInjectData{
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
	*Config
	Disable bool // 是否关闭
}

type ServiceAdapter struct {
	app      core.IApp
	services map[string]*PulsarConsumeService
}

func (s *ServiceAdapter) Inject(a ...interface{}) {
	for _, v := range a {
		data, ok := v.(serviceAdapterInjectData)
		if !ok {
			s.app.Fatal("pulsar消费服务注入类型错误, 它必须能转为 *pulsar_consume.serviceAdapterInjectData")
		}

		ss, ok := s.services[data.ConsumeName]
		if !ok {
			s.app.Fatal("pulsar消费服务注入参数错误, 未定义的消费者名", zap.String("ConsumeName", data.ConsumeName))
		}
		ss.RegistryHandler(data.Handlers...)
	}
}

func (s *ServiceAdapter) Start() error {
	var wg sync.WaitGroup
	wg.Add(len(s.services))
	for name, ss := range s.services {
		go func(name string, ss *PulsarConsumeService) {
			err := ss.Start()
			if err != nil {
				s.app.Fatal("pulsar消费者启动失败", zap.String("name", name), zap.Error(err))
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
		go func(ss *PulsarConsumeService) {
			ss.Close()
			wg.Done()
		}(ss)
	}
	wg.Wait()
	return nil
}

func NewServiceAdapter(app core.IApp) core.IService {
	consumersConf := make(map[string]interface{})
	err := app.GetConfig().ParseServiceConfig(nowServiceType, consumersConf)
	if err != nil {
		logger.Log.Panic("服务配置错误", zap.String("serviceType", string(nowServiceType)), zap.Error(err))
	}

	services := make(map[string]*PulsarConsumeService, len(consumersConf))
	for name := range consumersConf {
		conf := &serviceAdapterConfig{
			Config: NewConfig(),
		}
		err = app.GetConfig().ParseServiceConfig(nowServiceType+"."+core.ServiceType(name), conf)
		if err != nil {
			logger.Log.Panic("服务配置错误", zap.String("serviceType", string(nowServiceType)), zap.String("name", name), zap.Error(err))
		}
		if conf.Disable {
			continue
		}
		s, err := NewConsumeService(app, conf.Config)
		if err != nil {
			logger.Log.Panic("创建服务失败", zap.String("serviceType", string(nowServiceType)), zap.String("name", name), zap.Error(err))
		}
		services[name] = s
	}

	return &ServiceAdapter{
		app:      app,
		services: services,
	}
}

package kafka_consume

import (
	"github.com/zly-app/zapp"
	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/service"
)

// DefaultServiceType 默认服务类型
const DefaultServiceType core.ServiceType = "kafka-consume"

// WithService 启用 kafka-consume 服务，必须在 zapp 初始化前调用。
func WithService() zapp.Option {
	service.RegisterCreatorFunc(DefaultServiceType, func(app core.IApp) core.IService {
		return NewKafkaConsumeService(app)
	})
	return zapp.WithService(DefaultServiceType)
}

// Install 返回消费者注册器，必须在 zapp 初始化后调用（例如 app.Run 之前）。
func Install(app core.IApp) *Registrar {
	return &Registrar{app: app}
}

// Registrar 是 kafka 消费者注册器。
type Registrar struct {
	app core.IApp
}

// RegistryHandler 注册handler
func (r *Registrar) RegistryHandler(topics []string, groupID string, handler RegistryKafkaConsumerHandlerFunc, opts ...ConsumerOption) {
	r.app.InjectService(DefaultServiceType, &ConsumerConfig{
		Topics:  topics,
		GroupID: groupID,
		Handler: handler,
		Opts:    opts,
	})
}

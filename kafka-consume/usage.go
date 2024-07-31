package kafka_consume

import (
	"github.com/zly-app/zapp"
	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/service"
)

// 默认服务类型
const DefaultServiceType core.ServiceType = "kafka-consume"

// 当前服务类型
var nowServiceType = DefaultServiceType

// 设置服务类型, 这个函数应该在 zapp.NewApp 之前调用
func SetServiceType(t core.ServiceType) {
	nowServiceType = t
}

func init() {
	service.RegisterCreatorFunc(nowServiceType, func(app core.IApp) core.IService {
		return NewKafkaConsumeService(app)
	})
}

// 启用kafka-consume服务
func WithService() zapp.Option {
	return zapp.WithService(nowServiceType)
}

// 注册handler
func RegistryHandler(topics []string, groupID string, handler RegistryKafkaConsumerHandlerFunc, opts ...ConsumerOption) {
	zapp.App().InjectService(nowServiceType, &ConsumerConfig{
		Topics:  topics,
		GroupID: groupID,
		Handler: handler,
		Opts:    opts,
	})
}

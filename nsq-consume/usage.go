/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2021/1/23
   Description :
-------------------------------------------------
*/

package nsq_consume

import (
	"github.com/zly-app/zapp"
	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/service"
)

// 默认服务类型
const DefaultServiceType core.ServiceType = "nsq-consume"

// 当前服务类型
var nowServiceType = DefaultServiceType

// 设置服务类型, 这个函数应该在 zapp.NewApp 之前调用
func SetServiceType(t core.ServiceType) {
	nowServiceType = t
}

func init() {
	service.RegisterCreatorFunc(nowServiceType, func(app core.IApp) core.IService {
		return NewNsqConsumeService(app)
	})
}

// 启用nsq-consume服务
func WithService() zapp.Option {
	return zapp.WithService(nowServiceType)
}

// 注册handler
func RegistryHandler(topic, channel string, handler RegistryNsqConsumerHandlerFunc, opts ...ConsumerOption) {
	zapp.App().InjectService(nowServiceType, &ConsumerConfig{
		Topic:   topic,
		Channel: channel,
		Handler: handler,
		Opts:    opts,
	})
}

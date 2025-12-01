/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2021/1/25
   Description :
-------------------------------------------------
*/

package nsq_consume

import (
	"sync"

	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/log"
	"go.uber.org/zap"
)

type NsqConsumeService struct {
	app       core.IApp
	conf      *ServiceConfig
	consumers []*consumerCli
}

func (n *NsqConsumeService) Inject(a ...interface{}) {
	for _, v := range a {
		conf, ok := v.(*ConsumerConfig)
		if !ok {
			n.app.Fatal("nsq消费服务注入类型错误, 它必须能转为 *nsq_consume.ConsumerConfig")
		}
		conf.ServiceConfig = n.conf

		consumer := newConsumer(n.app, conf)
		n.consumers = append(n.consumers, consumer)
	}
}

func (n *NsqConsumeService) Start() error {
	// 开始消费
	for _, consumer := range n.consumers {
		if err := consumer.Start(); err != nil {
			return err
		}
	}

	return nil
}

func (n *NsqConsumeService) Close() error {
	var wg sync.WaitGroup
	wg.Add(len(n.consumers))
	for _, consumer := range n.consumers {
		go func(consumer *consumerCli) {
			defer wg.Done()
			_ = consumer.Close()
		}(consumer)
	}
	wg.Wait()
	return nil
}

func NewNsqConsumeService(app core.IApp) core.IService {
	// 加载配置
	conf := newConfig()
	err := app.GetConfig().ParseServiceConfig(nowServiceType, conf)
	if err == nil {
		err = conf.Check()
	}
	if err != nil {
		log.Panic("服务配置错误", zap.String("serviceType", string(nowServiceType)), zap.Error(err))
	}

	return &NsqConsumeService{
		app:  app,
		conf: conf,
	}
}

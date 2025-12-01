package kafka_consume

import (
	"sync"

	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/log"
	"go.uber.org/zap"
)

type KafkaConsumeService struct {
	app       core.IApp
	conf      *ServiceConfig
	consumers []*consumerCli
}

func (k *KafkaConsumeService) Inject(a ...interface{}) {
	for _, v := range a {
		conf, ok := v.(*ConsumerConfig)
		if !ok {
			k.app.Fatal("kafka消费服务注入类型错误, 它必须能转为 *kafka_consume.ConsumerConfig")
		}
		conf.ServiceConfig = k.conf

		consumer := newConsumer(k.app, conf)
		k.consumers = append(k.consumers, consumer)
	}
}

func (k *KafkaConsumeService) Start() error {
	// 开始消费
	for _, consumer := range k.consumers {
		if err := consumer.Start(); err != nil {
			return err
		}
	}

	return nil
}

func (k *KafkaConsumeService) Close() error {
	var wg sync.WaitGroup
	wg.Add(len(k.consumers))
	for _, consumer := range k.consumers {
		go func(consumer *consumerCli) {
			defer wg.Done()
			_ = consumer.Close()
		}(consumer)
	}
	wg.Wait()
	return nil
}

func NewKafkaConsumeService(app core.IApp) core.IService {
	// 加载配置
	conf := newConfig()
	err := app.GetConfig().ParseServiceConfig(nowServiceType, conf)
	if err == nil {
		err = conf.Check()
	}
	if err != nil {
		log.Panic("服务配置错误", zap.String("serviceType", string(nowServiceType)), zap.Error(err))
	}

	return &KafkaConsumeService{
		app:  app,
		conf: conf,
	}
}

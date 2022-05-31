package pulsar_consume

import (
	"fmt"
	"sync"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/apache/pulsar-client-go/pulsar/log"
	"github.com/zly-app/zapp/core"
)

type PulsarConsumeService struct {
	app      core.IApp
	client   pulsar.Client
	consumes []*Consume
	handler  []ConsumerHandler
}

func (p *PulsarConsumeService) Start() {
	for _, consume := range p.consumes {
		go consume.Start()
	}
}

func (p *PulsarConsumeService) Close() {
	var wg sync.WaitGroup
	wg.Add(len(p.consumes))
	for _, consume := range p.consumes {
		go func(consume *Consume) {
			consume.Close()
			wg.Done()
		}(consume)
	}
	wg.Wait()
}

// 注册消费函数, 应该在Start之前调用
func (p *PulsarConsumeService) RegistryHandler(handler ...ConsumerHandler) {
	h := make([]ConsumerHandler, 0, len(handler))
	h = append(h, handler...)
	p.handler = append(p.handler, h...)
}

func (p *PulsarConsumeService) consumeHandler(msg Message) bool {
	panic("未实现")
}

func NewConsumeService(app core.IApp, conf *Config) (*PulsarConsumeService, error) {
	if err := conf.Check(); err != nil {
		return nil, fmt.Errorf("配置检查失败: %v", err)
	}

	p := &PulsarConsumeService{
		app: app,
	}

	co := pulsar.ClientOptions{
		URL:                     conf.Url,
		ConnectionTimeout:       time.Duration(conf.ConnectionTimeout) * time.Millisecond,
		OperationTimeout:        time.Duration(conf.OperationTimeout) * time.Millisecond,
		ListenerName:            conf.ListenerName,
		MaxConnectionsPerBroker: 1,
		Logger:                  log.DefaultNopLogger(),
	}

	client, err := pulsar.NewClient(co)
	if err != nil {
		return nil, fmt.Errorf("创建pulsar客户端失败: %v", err)
	}

	consumes := make([]*Consume, conf.ConsumeCount)
	for i := 0; i < conf.ConsumeCount; i++ {
		consumer, err := NewConsume(app, client, conf, p.consumeHandler)
		if err != nil {
			return nil, fmt.Errorf("创建pulsar消费者失败: %v", err)
		}
		consumes[i] = consumer
	}

	p.client = client
	p.consumes = consumes
	return p, nil
}
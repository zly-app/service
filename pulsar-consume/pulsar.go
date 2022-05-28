package pulsar_consume

import (
	"fmt"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/apache/pulsar-client-go/pulsar/log"
	"github.com/zly-app/zapp/core"
)

type PulsarConsumeService struct {
	app      core.IApp
	client   pulsar.Client
	consumes []*Consume
}

func (p *PulsarConsumeService) Start() error {
	panic("未实现")
}

func (p *PulsarConsumeService) Close() error {
	panic("未实现")
}

func (p *PulsarConsumeService) Consume(msg Message) error {
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
		ConnectionTimeout:       time.Duration(conf.ConnectionTimeout) * time.Second,
		OperationTimeout:        time.Duration(conf.OperationTimeout) * time.Second,
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
		consumer, err := NewConsume(app, client, conf, p.Consume)
		if err != nil {
			return nil, fmt.Errorf("创建pulsar消费者失败: %v", err)
		}
		consumes[i] = consumer
	}

	p.client = client
	p.consumes = consumes
	return p, nil
}

package pulsar_consume

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/apache/pulsar-client-go/pulsar/log"
	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/filter"
	"github.com/zly-app/zapp/pkg/utils"
)

type PulsarConsumeService struct {
	name     string
	app      core.IApp
	client   pulsar.Client
	conf     *Config
	consumes []*Consumer
	handler  []ConsumerHandler
}

func (p *PulsarConsumeService) Start() error {
	if len(p.handler) == 0 {
		return fmt.Errorf("未设置handler")
	}

	for _, consume := range p.consumes {
		go consume.Start()
	}
	return nil
}

func (p *PulsarConsumeService) Close() {
	var wg sync.WaitGroup
	wg.Add(len(p.consumes))
	for _, consume := range p.consumes {
		go func(consume *Consumer) {
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

type consumeReq struct {
	MID             string
	Topic           string
	Payload         string
	PublishTime     time.Time         `json:"PublishTime,omitempty"`
	Key             string            `json:"Key,omitempty"`
	OrderingKey     string            `json:"OrderingKey,omitempty"`
	Properties      map[string]string `json:"Properties,omitempty"`
	RedeliveryCount uint32            `json:"RedeliveryCount,omitempty"`
	msg             Message
}

func (p *PulsarConsumeService) consumeHandler(msg Message) bool {
	ctx, chain := filter.GetServiceFilter(p.app.BaseContext(), string(DefaultServiceType)+"/"+p.name, "Consume")
	r := &consumeReq{
		MID:             msg.ID().String(),
		Topic:           msg.Topic(),
		Payload:         string(msg.Payload()),
		PublishTime:     msg.PublishTime(),
		Key:             msg.Key(),
		OrderingKey:     msg.OrderingKey(),
		Properties:      msg.Properties(),
		RedeliveryCount: msg.RedeliveryCount(),
		msg:             msg,
	}
	_, err := chain.Handle(ctx, r, func(ctx context.Context, req interface{}) (interface{}, error) {
		msg := req.(*consumeReq).msg
		err := utils.Recover.WrapCall(func() error {
			for _, fn := range p.handler {
				if err := fn(ctx, msg); err != nil {
					return err
				}
			}
			return nil
		})
		return nil, err
	})
	if err != nil {
		return false
	}
	return true
}

func NewConsumeService(name string, app core.IApp, conf *Config) (*PulsarConsumeService, error) {
	if err := conf.Check(); err != nil {
		return nil, fmt.Errorf("配置检查失败: %v", err)
	}

	p := &PulsarConsumeService{
		name: name,
		app:  app,
		conf: conf,
	}

	co := pulsar.ClientOptions{
		URL:                     conf.Url,
		ConnectionTimeout:       time.Duration(conf.ConnectionTimeout) * time.Millisecond,
		OperationTimeout:        time.Duration(conf.OperationTimeout) * time.Millisecond,
		ListenerName:            conf.ListenerName,
		MaxConnectionsPerBroker: 1,
		Logger:                  log.DefaultNopLogger(),
	}
	if conf.AuthBasicUser != "" {
		auth, err := pulsar.NewAuthenticationBasic(conf.AuthBasicUser, conf.AuthBasicPassword)
		if err != nil {
			return nil, fmt.Errorf("创建pulsar认证失败: %v", err)
		}
		co.Authentication = auth
	}

	client, err := pulsar.NewClient(co)
	if err != nil {
		return nil, fmt.Errorf("创建pulsar客户端失败: %v", err)
	}

	consumes := make([]*Consumer, conf.ConsumeCount)
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

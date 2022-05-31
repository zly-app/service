package pulsar_consume

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/apache/pulsar-client-go/pulsar/log"
	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/pkg/utils"
	"go.uber.org/zap"
)

type PulsarConsumeService struct {
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

func (p *PulsarConsumeService) consumeHandler(msg Message) bool {
	ctx, cancel := context.WithCancel(p.app.BaseContext())
	defer cancel()

	logger := p.app.NewTraceLogger(ctx,
		zap.String("Topic", msg.Topic()),
		zap.String("SubscriptionName", p.conf.SubscriptionName),
		zap.String("SubscriptionType", p.conf.SubscriptionType),
		zap.Int64("LedgerID", msg.ID().LedgerID()),
		zap.Int64("EntryID", msg.ID().EntryID()),
		zap.String("ProducerName", msg.ProducerName()),
		zap.String("PublishTime", msg.PublishTime().Format(time.RFC3339Nano)),
	)
	cCtx := &Context{
		ILogger:          logger,
		Ctx:              ctx,
		Msg:              msg,
		SubscriptionName: p.conf.SubscriptionName,
	}

	if p.conf.ConsumeLogLevelIsInfo {
		cCtx.Info("pulsarConsume.receive")
	} else {
		cCtx.Debug("pulsarConsume.receive")
	}

	err := utils.Recover.WrapCall(func() error {
		for _, fn := range p.handler {
			if err := fn(cCtx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		errDetail := utils.Recover.GetRecoverErrorDetail(err)
		cCtx.Error("pulsarConsumer.error!", zap.String("error", errDetail))
		return false
	}

	if p.conf.ConsumeLogLevelIsInfo {
		cCtx.Info("pulsarConsumer.success")
	} else {
		cCtx.Debug("pulsarConsumer.success")
	}
	return true
}

func NewConsumeService(app core.IApp, conf *Config) (*PulsarConsumeService, error) {
	if err := conf.Check(); err != nil {
		return nil, fmt.Errorf("配置检查失败: %v", err)
	}

	p := &PulsarConsumeService{
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

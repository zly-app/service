package pulsar_consume

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/zly-app/zapp/core"
	"go.uber.org/zap"
)

type ConsumerHandler func(ctx context.Context, msg Message) error

type Consumer struct {
	app       core.IApp
	conf      *Config
	consume   pulsar.Consumer
	handle    func(Message) bool
	workers   *Workers
	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (c *Consumer) Start() {
	c.workers.Start()
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			msg, err := c.consume.Receive(c.ctx)
			if err == context.Canceled {
				return
			}
			if err != nil {
				c.app.Error("consume.Receive err", zap.Error(err))
				time.Sleep(time.Duration(c.conf.ReceiveMsgRetryTime) * time.Millisecond)
				continue
			}
			c.workers.Go(func() {
				if c.handle(msg) { // 成功
					c.consume.Ack(msg)
				} else if c.conf.EnableRetryTopic {
					c.consume.ReconsumeLater(msg, time.Duration(c.conf.ReconsumeTime)*time.Millisecond)
				} else {
					c.consume.Nack(msg)
				}
			})
		}
	}
}

func (c *Consumer) Close() {
	c.ctxCancel()
	c.workers.Stop()
}

func NewConsume(app core.IApp, client pulsar.Client, conf *Config, handle func(Message) bool) (*Consumer, error) {
	co := pulsar.ConsumerOptions{
		AutoDiscoveryPeriod: time.Duration(conf.AutoDiscoveryPeriod) * time.Millisecond,
		SubscriptionName:    conf.SubscriptionName,
		//Properties:                     nil,
		//SubscriptionProperties:         nil,
		//KeySharedPolicy:                nil,
		RetryEnable:         conf.EnableRetryTopic,
		ReceiverQueueSize:   conf.ReceiverQueueSize,
		NackRedeliveryDelay: time.Duration(conf.ReconsumeTime) * time.Millisecond,
		ReadCompacted:       conf.ReadCompacted,
		//Interceptors:                   nil,
		//Schema:                         nil,
		//Decryption:                     nil,
		EnableDefaultNackBackoffPolicy: conf.EnableDefaultNackBackoffPolicy,
		//NackBackoffPolicy:              nil,
	}
	if conf.Topics != "" {
		topics := strings.Split(conf.Topics, ",")
		if len(topics) == 1 {
			co.Topic = topics[0]
		} else {
			co.Topics = topics
		}
	} else {
		co.TopicsPattern = conf.TopicsPattern
	}
	switch strings.ToLower(conf.SubscriptionType) {
	case "exclusive":
		co.Type = pulsar.Exclusive
	case "failover":
		co.Type = pulsar.Failover
	case "shared":
		co.Type = pulsar.Shared
	case "keyshared":
		co.Type = pulsar.KeyShared
	}
	switch strings.ToLower(conf.SubscriptionInitialPosition) {
	case "latest":
		co.SubscriptionInitialPosition = pulsar.SubscriptionPositionLatest
	case "earliest":
		co.SubscriptionInitialPosition = pulsar.SubscriptionPositionEarliest
	}
	if conf.DLQMaxDeliveries > 0 {
		co.DLQ = &pulsar.DLQPolicy{
			MaxDeliveries:    uint32(conf.DLQMaxDeliveries),
			DeadLetterTopic:  conf.DLQDeadLetterTopic,
			RetryLetterTopic: conf.DLQRetryLetterTopic,
		}
	}
	if conf.MaxReconnectToBroker > -1 {
		v := uint(conf.MaxReconnectToBroker)
		co.MaxReconnectToBroker = &v
	}

	consume, err := client.Subscribe(co)
	if err != nil {
		return nil, fmt.Errorf("订阅失败: %v", err)
	}

	ctx, ctxCancel := context.WithCancel(app.BaseContext())
	c := &Consumer{
		app:       app,
		conf:      conf,
		consume:   consume,
		handle:    handle,
		workers:   NewWorkers(conf.ConsumeThreadCount),
		ctx:       ctx,
		ctxCancel: ctxCancel,
	}
	return c, nil
}

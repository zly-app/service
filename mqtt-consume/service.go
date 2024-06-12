package mqtt_consume

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/filter"
	"github.com/zly-app/zapp/logger"
	"github.com/zly-app/zapp/pkg/utils"
	"go.uber.org/zap"
)

type ConsumerHandler func(ctx context.Context, msg Message) error

type MQTTConsumeService struct {
	name    string
	app     core.IApp
	client  mqtt.Client
	conf    *Config
	handler []ConsumerHandler
	workers *Workers
}

func NewConsumeService(name string, app core.IApp, conf *Config) (*MQTTConsumeService, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(conf.Server)
	opts.SetClientID(conf.ClientID) //设备唯一id，正常应该是设备拿自己的设备id注册到服务器上。
	if conf.User != "" {
		opts.SetUsername(conf.User)     //账号
		opts.SetPassword(conf.Password) //密码
	}
	client := mqtt.NewClient(opts)
	token := client.Connect()
	waitOk := token.WaitTimeout(time.Duration(conf.WaitConnectedTimeMs) * time.Millisecond)
	if !waitOk {
		return nil, errors.New("连接mqtt超时")
	}
	if token.Error() != nil {
		return nil, fmt.Errorf("连接mqtt失败: %s", token.Error())
	}

	s := &MQTTConsumeService{
		name:    name,
		app:     app,
		client:  client,
		conf:    conf,
		handler: make([]ConsumerHandler, 0),
	}

	return s, nil
}

func (s *MQTTConsumeService) Close() {
	if s.client != nil {
		s.client.Disconnect(uint(s.conf.WaitConnectedTimeMs))
	}
	if s.workers != nil {
		s.workers.Stop()
	}
}

// 注册消费函数, 应该在Start之前调用
func (s *MQTTConsumeService) RegistryHandler(handler ...ConsumerHandler) {
	h := make([]ConsumerHandler, 0, len(handler))
	h = append(h, handler...)
	s.handler = append(s.handler, h...)
}

func (s *MQTTConsumeService) Start() error {
	if len(s.handler) == 0 {
		return fmt.Errorf("未设置handler")
	}

	s.workers = NewWorkers(s.conf.ConsumeThreadCount)

	topics := strings.Split(s.conf.Topics, ",")
	for i := range topics {
		err := s.subscribe(topics[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *MQTTConsumeService) subscribe(topic string) error {
	token := s.client.Subscribe(topic, s.conf.Qos, s.process)
	waitOk := token.WaitTimeout(time.Duration(s.conf.WaitConnectedTimeMs) * time.Millisecond)
	if !waitOk {
		return fmt.Errorf("subscribe mqtt topic: %v timeout", topic)
	}
	if token.Error() != nil {
		return fmt.Errorf("subscribe mqtt topic: %v err: %v", topic, token.Error())
	}
	logger.Log.Info("subscribe mqtt topic", zap.String("mqtt_name", s.name), zap.String("topic", topic))
	return nil
}

func (s *MQTTConsumeService) process(_ mqtt.Client, msg mqtt.Message) {
	s.workers.Go(func() {
		s.consumeHandler(msg)
	})
}

type consumeReq struct {
	Duplicate bool `json:"Duplicate,omitempty"`
	Qos       byte
	Retained  bool `json:"Retained,omitempty"`
	MID       uint16
	Topic     string
	Payload   string
	msg       Message
}

func (s *MQTTConsumeService) consumeHandler(msg Message) {
	ctx, chain := filter.GetServiceFilter(context.Background(), string(DefaultServiceType)+"/"+s.name, "Consume")
	r := &consumeReq{
		Duplicate: msg.Duplicate(),
		Qos:       msg.Qos(),
		Retained:  msg.Retained(),
		Topic:     msg.Topic(),
		MID:       msg.MessageID(),
		Payload:   string(msg.Payload()),
		msg:       msg,
	}
	_, err := chain.Handle(ctx, r, func(ctx context.Context, req interface{}) (interface{}, error) {
		r := req.(*consumeReq)
		msg := r.msg
		err := utils.Recover.WrapCall(func() error {
			for _, fn := range s.handler {
				if err := fn(ctx, msg); err != nil {
					return err
				}
			}
			return nil
		})
		return nil, err
	})
	if err != nil {
		logger.Log.Error("mqtt msg consume err", zap.String("mqtt_name", s.name), zap.String("topic", msg.Topic()), zap.Error(err))
	}

	msg.Ack()
}

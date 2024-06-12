package mqtt_consume

import (
	"github.com/zly-app/zapp/core"
)

const (
	defServer              = "localhost:1883"
	defWaitConnectedTimeMs = 5000

	defTopics       = "test"
	defQos          = 1
	defCleanSession = true

	defConsumeThreadCount = 0
)

type Config struct {
	Server              string // mqtt服务地址. 如 localhost:1883
	WaitConnectedTimeMs int    // 等待连接超时时间, 单位毫秒

	User     string // mqtt用户名
	Password string // mqtt密码

	Topics       string // 消费topic, 多个topic用英文逗号连接. 示例: test/topic/a,test/topic/b,test/topic/+
	Qos          byte   // qos级别, 0最多交付一次, 1至少交付一次, 2仅交付一次
	ClientID     string // clientID 如果为空则设为 实例id. 如果客户端使用一个重复的 Client ID 连接至服务器，将会把已使用该 Client ID 连接成功的客户端踢下线。
	CleanSession bool   // 清除会话, 设为false时, 服务端会为同一个会话的客户端保留一定数量的离线消息, 通常是1000条. 不会保存Qos=0的消息. 部分mqtt服务器不支持这个功能

	ConsumeThreadCount int // 消费者协程数, 0表示使用逻辑处理器数量*2
}

func NewConfig() *Config {
	return &Config{
		Qos:          defQos,
		CleanSession: defCleanSession,

		ConsumeThreadCount: defConsumeThreadCount,
	}
}

func (conf *Config) Check(app core.IApp) error {
	if conf.Server == "" {
		conf.Server = defServer
	}
	if conf.WaitConnectedTimeMs < 1 {
		conf.WaitConnectedTimeMs = defWaitConnectedTimeMs
	}

	if conf.Topics == "" {
		conf.Topics = defTopics
	}
	if conf.ClientID == "" {
		conf.ClientID = app.GetConfig().Config().Frame.Instance
	}

	if conf.ConsumeThreadCount < 0 {
		conf.ConsumeThreadCount = defConsumeThreadCount
	}
	return nil
}

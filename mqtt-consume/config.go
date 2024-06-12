package mqtt_consume

import (
	"github.com/zly-app/zapp/core"
)

const (
	defServer              = "localhost:1883"
	defWaitConnectedTimeMs = 5000

	defTopics = "test"
	defQos    = 1

	defConsumeThreadCount = 0
)

type Config struct {
	Server              string // mqtt服务地址. 如 localhost:1883
	WaitConnectedTimeMs int    // 等待连接超时时间, 单位毫秒

	User     string // mqtt用户名
	Password string // mqtt密码

	Topics   string // 消费topic, 多个topic用英文逗号连接. 示例: test/topic/a,test/topic/b,test/topic/+
	Qos      byte   // qos级别, 0最多交付一次, 1至少交付一次, 2仅交付一次
	ClientID string // clientID 如果为空则设为 实例id

	ConsumeThreadCount int // 每个消费者协程数, 0表示使用逻辑处理器数量*2
}

func NewConfig() *Config {
	return &Config{
		Qos:                defQos,
		ConsumeThreadCount: defConsumeThreadCount,
	}
}

func (conf Config) Check(app core.IApp) error {
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

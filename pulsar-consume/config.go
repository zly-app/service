package pulsar_consume

import (
	"fmt"
	"strings"
)

const (
	// pulsar地址
	defUrl = "pulsar://localhost:6650"
	// 默认监听器名称
	defListenerName = "external"
	// 连接超时
	defConnectionTimeout = 5000
	// 操作超时
	defOperationTimeout = 30000

	// 消费topic
	defTopics = "persistent://public/default/test"
	// 指定轮询新分区或新主题的时间间隔
	defAutoDiscoveryPeriod = 60000
	// 订阅名
	defSubscriptionName = "test"
	// 订阅类型
	defSubscriptionType = "shared"
	// 订阅初始化游标位置
	defSubscriptionInitialPosition = "latest"
	// dlq策略最大交付次数
	defDLQMaxDeliveries = 0
	// dlq策略死信topic默认后缀
	defDLQDeadLetterTopicSuffix = "-DEAD"
	// dlq策略重试topic默认后缀
	defDLQRetryLetterTopicSuffix = "-RETRY"
	// 启用重试topic的重试时间
	defReconsumeTime = 5000
	// 接收器队列大小
	defReceiverQueueSize = 1000
	// 重新连接到broker的最大次数
	defMaxReconnectToBroker = -1

	// 消费者数量
	defConsumeCount = 1
	// 每个消费者协程数
	defConsumeThreadCount = 1
	// 消费者接收消息失败重试时间
	defReceiveMsgRetryTime = 5000
)

type Config struct {
	Url               string // pulsar地址, 示例: pulsar://localhost:6600,localhost:6650
	ListenerName      string // pulsar使用的监听器名, 示例: external
	ConnectionTimeout int    // 连接超时, 单位毫秒
	OperationTimeout  int    // 操作超时, 单位毫秒

	AuthBasicUser     string // 基础认证用户名, 可用于 AuthenticationProviderBasic 的认证扩展
	AuthBasicPassword string // 基础认证密码

	Topics                         string // 消费topic, 多个topic用英文逗号连接. 示例: persistent://public/default/test1,persistent://public/default/test2
	TopicsPattern                  string // 支持正则匹配的消费topic, 要求所有topic都在同一个命名空间. 示例: persistent://public/default/test-.*
	AutoDiscoveryPeriod            int    // 指定轮询新分区或新主题的时间间隔, 单位毫秒. 只有 TopicsPattern 生效时才会启用
	SubscriptionName               string // 订阅名
	SubscriptionType               string // 订阅类型, 支持 exclusive,failover,shared,keyshared. 默认 shared
	SubscriptionInitialPosition    string // 订阅初始化游标位置, 支持 latest,earliest
	DLQMaxDeliveries               int    // dlq策略最大交付次数, 0表示禁用
	DLQDeadLetterTopic             string // dlq策略死信队列topic
	DLQRetryLetterTopic            string // dlq策略重试队列topic
	EnableRetryTopic               bool   // 启用重试topic, 当消息消费失败时扔到重试队列, 如果设为false, 会在内存中保存消息然后重试. 只有设置了DLQ策略才生效
	ReconsumeTime                  int    // 重新消费时间, 单位毫秒
	ReceiverQueueSize              int    // 接收器队列大小
	ReadCompacted                  bool   // 是否允许从压缩topic中读取. 只有持久topic并且在exclusive或failover订阅模式才可用, 否则会产生错误
	MaxReconnectToBroker           int    // 重新连接到broker的最大次数, -1表示不限
	EnableDefaultNackBackoffPolicy bool   // 是否启用重试补偿时间策略, 只有在不使用重试队列topic时生效

	ConsumeLogLevelIsInfo bool // 将消费日志设为info
	ConsumeCount          int  // 消费者数量
	ConsumeThreadCount    int  // 每个消费者协程数, keyShard模式建议增加ConsumeCount而不是通过ConsumeThreadCount提高速度
	ReceiveMsgRetryTime   int  // 消费者接收消息失败重试时间, 单位毫秒
}

func NewConfig() *Config {
	return &Config{
		ListenerName:         defListenerName,
		MaxReconnectToBroker: defMaxReconnectToBroker,
	}
}

func (conf *Config) Check() error {
	if conf.Url == "" {
		conf.Url = defUrl
	}
	if conf.ConnectionTimeout < 1 {
		conf.ConnectionTimeout = defConnectionTimeout
	}
	if conf.OperationTimeout < 1 {
		conf.OperationTimeout = defOperationTimeout
	}

	if conf.Topics == "" && conf.TopicsPattern == "" {
		conf.Topics = defTopics
	}
	if conf.AutoDiscoveryPeriod < 1 {
		conf.AutoDiscoveryPeriod = defAutoDiscoveryPeriod
	}
	if conf.SubscriptionName == "" {
		conf.SubscriptionName = defSubscriptionName
	}
	switch strings.ToLower(conf.SubscriptionType) {
	case "":
		conf.SubscriptionType = defSubscriptionType
	case "exclusive", "failover", "shared", "keyshared":
	default:
		return fmt.Errorf("不支持的订阅类型: %v", conf.SubscriptionType)
	}
	switch strings.ToLower(conf.SubscriptionInitialPosition) {
	case "":
		conf.SubscriptionInitialPosition = defSubscriptionInitialPosition
	case "latest", "earliest":
	default:
		return fmt.Errorf("不支持的订阅初始化游标位置类型: %v", conf.SubscriptionType)
	}
	if conf.DLQMaxDeliveries < 1 {
		conf.DLQMaxDeliveries = defDLQMaxDeliveries
	}
	if conf.DLQMaxDeliveries > 0 && conf.DLQDeadLetterTopic == "" {
		conf.DLQDeadLetterTopic = conf.SubscriptionName + defDLQDeadLetterTopicSuffix
	}
	if conf.DLQMaxDeliveries > 0 && conf.DLQRetryLetterTopic == "" {
		conf.DLQRetryLetterTopic = conf.SubscriptionName + defDLQRetryLetterTopicSuffix
	}
	if conf.ReconsumeTime < 1 {
		conf.ReconsumeTime = defReconsumeTime
	}
	if conf.ReceiverQueueSize < 1 {
		conf.ReceiverQueueSize = defReceiverQueueSize
	}
	if conf.MaxReconnectToBroker < 0 {
		conf.MaxReconnectToBroker = defMaxReconnectToBroker
	}

	if conf.ConsumeCount < 1 {
		conf.ConsumeCount = defConsumeCount
	}
	if conf.ConsumeThreadCount < 1 {
		conf.ConsumeThreadCount = defConsumeThreadCount
	}
	if conf.ReceiveMsgRetryTime < 1 {
		conf.ReceiveMsgRetryTime = defReceiveMsgRetryTime
	}
	return nil
}

/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2021/1/21
   Description :
-------------------------------------------------
*/

package config

import (
	"runtime"
)

const (
	// 默认bind
	defaultBind = ":8080"
	// 适配ingress的X-Original-Forwarded-For获取ip, 优先级高于X-Forwarded-For
	defaultIPWithIngressForwarded = true
	// 适配proxy的X-Forwarded-For获取ip, 优先级高于X-Real-Ip
	defaultIPWithProxyForwarded = true
	// 适配proxy的X-Real-Ip获取ip, 优先级高于sock连接的ip
	defaultIPWithProxyReal = true
	// 在开发环境中输出api结果
	defLogApiResultInDevelop = true
	// 默认post允许最大数据大小(128M)
	defaultPostMaxMemory = 128 << 20

	// 同时处理请求的goroutine数
	defThreadCount = 0
	// 最大请求等待队列大小
	defMaxReqWaitQueueSize = 10000

	// 请求日志等级设为info
	defReqLogLevelIsInfo = true
	// 响应日志等级设为info
	defRspLogLevelIsInfo = true
	// bind日志等级设为info
	defBindLogLevelIsInfo = true
	// 在生产环境中输出api结果
	defLogApiResultInProd = true
	// 在生产环境发送详细的错误到客户端
	defSendDetailedErrorInProduction = false
	// 总是输出headers日志
	defAlwaysLogHeaders = true
	// 总是输出body日志
	defAlwaysLogBody = true
	// 默认输出结果最大大小(128K)
	defaultLogApiResultMaxSize = 128 << 10
	// 输出body最大大小(128K)
	defaultLogBodyMaxSize = 128 << 10
)

// api服务配置
type Config struct {
	Bind                   string // bind地址
	IPWithIngressForwarded bool   // 适配ingress的X-Original-Forwarded-For获取ip, 优先级高于X-Forwarded-For
	IPWithProxyForwarded   bool   // 适配proxy的X-Forwarded-For获取ip, 优先级高于X-Real-Ip
	IPWithProxyReal        bool   // 适配proxy的X-Real-Ip获取ip, 优先级高于sock连接的ip
	PostMaxMemory          int64  // post允许客户端传输最大数据大小, 单位字节

	// 同时处理请求的goroutine数, 设为0时取逻辑cpu数*2, 设为负数时不作任何限制, 每个请求由独立的线程执行
	ThreadCount int
	// 最大请求等待队列大小
	//
	// 只有 ThreadCount >= 0 时生效.
	// 启动时创建一个指定大小的任务队列, 触发产生的请求会放入这个队列, 队列已满时新触发的请求会返回错误
	MaxReqWaitQueueSize int

	ReqLogLevelIsInfo             bool  // 请求日志等级设为info
	RspLogLevelIsInfo             bool  // 响应日志等级设为info
	BindLogLevelIsInfo            bool  // bind日志等级设为info
	LogApiResultInDevelop         bool  // 在开发环境中输出api结果
	LogApiResultInProd            bool  // 在生产环境中输出api结果
	SendDetailedErrorInProduction bool  // 在生产环境发送详细的错误到客户端
	AlwaysLogHeaders              bool  // 总是输出headers日志, 如果设为false, 只会在出现错误时才会输出headers日志
	AlwaysLogBody                 bool  // 总是输出body日志, 如果设为false, 只会在出现错误时才会输出body日志
	LogApiResultMaxSize           int   // 日志输出结果最大大小
	LogBodyMaxSize                int64 // 日志输出请求body最大大小
}

func NewConfig() *Config {
	return &Config{
		Bind:                   defaultBind,
		IPWithIngressForwarded: defaultIPWithIngressForwarded,
		IPWithProxyForwarded:   defaultIPWithProxyForwarded,
		IPWithProxyReal:        defaultIPWithProxyReal,

		ThreadCount: defThreadCount,

		ReqLogLevelIsInfo:             defReqLogLevelIsInfo,
		RspLogLevelIsInfo:             defRspLogLevelIsInfo,
		BindLogLevelIsInfo:            defBindLogLevelIsInfo,
		LogApiResultInDevelop:         defLogApiResultInDevelop,
		LogApiResultInProd:            defLogApiResultInProd,
		SendDetailedErrorInProduction: defSendDetailedErrorInProduction,
		AlwaysLogHeaders:              defAlwaysLogHeaders,
		AlwaysLogBody:                 defAlwaysLogBody,
	}
}

func (conf *Config) Check() {
	if conf.Bind == "" {
		conf.Bind = defaultBind
	}
	if conf.PostMaxMemory < 1 {
		conf.PostMaxMemory = defaultPostMaxMemory
	}

	if conf.ThreadCount == 0 {
		conf.ThreadCount = runtime.NumCPU() * 2
	}
	if conf.ThreadCount < 0 {
		conf.ThreadCount = -1
	}
	if conf.MaxReqWaitQueueSize < 1 {
		conf.MaxReqWaitQueueSize = defMaxReqWaitQueueSize
	}

	if conf.LogApiResultMaxSize < 1 {
		conf.LogApiResultMaxSize = defaultLogApiResultMaxSize
	}
	if conf.LogBodyMaxSize < 1 {
		conf.LogBodyMaxSize = defaultLogBodyMaxSize
	}
}

/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2021/1/21
   Description :
-------------------------------------------------
*/

package config

const (
	// 默认bind
	defaultBind = ":8080"
	// 默认适配nginx的Forwarded获取ip
	defaultIPWithNginxForwarded = true
	// 默认适配nginx的Real获取ip
	defaultIPWithNginxReal = true
	// 默认输出结果最大大小
	defaultLogApiResultMaxSize = 10240
	// 输出body最大大小
	defaultLogBodyMaxSize = 10240
	// 默认post允许最大数据大小(32M)
	defaultPostMaxMemory = 32 << 20
)

// api服务配置
type Config struct {
	Bind                          string // bind地址
	ReqLogLevelIsInfo             bool   // 请求日志等级设为info
	RspLogLevelIsInfo             bool   // 响应日志等级设为info
	BindLogLevelIsInfo            bool   // bind日志等级设为info
	IPWithNginxForwarded          bool   // 适配nginx的Forwarded获取ip, 优先级高于nginx的Real
	IPWithNginxReal               bool   // 适配nginx的Real获取ip, 优先级高于sock连接的ip
	LogApiResultInDevelop         bool   // 在开发环境中输出api结果
	LogApiResultInProd            bool   // 在生产环境中输出api结果
	LogApiResultMaxSize           int    // 输出结果最大大小
	SendDetailedErrorInProduction bool   // 在生产环境发送详细的错误到客户端
	AlwaysLogHeaders              bool   // 总是输出headers日志, 如果设为false, 只会在出现错误时才会输出headers日志
	AlwaysLogBody                 bool   // 总是输出body日志, 如果设为false, 只会在出现错误时才会输出body日志
	LogBodyMaxSize                int64  // 输出body最大大小
	PostMaxMemory                 int64  // post允许客户端传输最大数据大小, 单位字节
}

func NewConfig() *Config {
	return &Config{
		IPWithNginxForwarded: defaultIPWithNginxForwarded,
		IPWithNginxReal:      defaultIPWithNginxReal,
	}
}

func (conf *Config) Check() {
	if conf.Bind == "" {
		conf.Bind = defaultBind
	}
	if conf.LogApiResultMaxSize < 1 {
		conf.LogApiResultMaxSize = defaultLogApiResultMaxSize
	}
	if conf.LogBodyMaxSize < 1 {
		conf.LogBodyMaxSize = defaultLogBodyMaxSize
	}
	if conf.PostMaxMemory < 1 {
		conf.PostMaxMemory = defaultPostMaxMemory
	}
}

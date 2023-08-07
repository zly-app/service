/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2021/1/26
   Description :
-------------------------------------------------
*/

package cron

import (
	"runtime"
)

const (
	// 默认线程数
	defaultThreadCount = -1
	// 默认最大任务队列大小
	defaultMaxTaskQueueSize = 10000
)

// 任务配置
type TaskFileConfig struct {
	Name                      string // 任务名
	Expression                string // cron表达式, https://en.wikipedia.org/wiki/Cron
	IsOnceTrigger             bool   // 是否为一次性触发, 如果设为true, 则 Expression 的格式为 YYYY-MM-dd hh:mm:ss
	Disable                   bool   // 是否禁用
	RetryCount                int64  // 任务失败重试次数, 0表示不重试
	RetrySleepMs              int64  // 失败重试等待时间, 单位秒, 0表示不等待
	MaxConcurrentExecuteCount int64  // 最大并发执行任务数, 如果为-1则不限制. 表示在执行过程中又被调度器触发执行时, 能同时运行同一个任务的数量. 默认1
	TimeoutMs                 int64  // 超时时间, 单位秒, 0表示永不超时
}

// CronService配置
type Config struct {
	/*
		线程数, 默认为-1
		  同时处理任务的全局最大goroutine数
		  如果为0, 所有触发的任务都会新开启一个goroutine
		  如果为-1, 使用逻辑cpu数量的4倍
	*/
	ThreadCount int
	/*
		最大任务队列大小, 默认为10000
		  只有 ThreadCount > 0 || ThreadCount == -1 时生效
		  启动时创建一个指定大小的任务队列, 触发产生的任务会放入这个队列, 队列已满时新触发的任务会被抛弃
	*/
	MaxTaskQueueSize int
	// 任务列表
	Tasks []TaskFileConfig
}

func newConfig() *Config {
	return &Config{
		ThreadCount:      defaultThreadCount,
		MaxTaskQueueSize: defaultMaxTaskQueueSize,
	}
}

func (c *Config) check() {
	if c.ThreadCount == -1 {
		c.ThreadCount = runtime.NumCPU() * 4
	}
	if c.MaxTaskQueueSize <= 0 {
		c.MaxTaskQueueSize = defaultMaxTaskQueueSize
	}
}

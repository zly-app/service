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

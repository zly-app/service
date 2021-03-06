/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2021/3/1
   Description :
-------------------------------------------------
*/

package nsq_consume

type consumerOptions struct {
	ThreadCount            int
	MaxAutoRequeueAttempts uint16
}

type ConsumerOption func(opts *consumerOptions)

func newConsumerOptions() *consumerOptions {
	return &consumerOptions{
		ThreadCount: 0,
	}
}

// 线程数, 默认为0表示使用配置的默认线程数
//
// 同时处理信息的goroutine数
func WithConsumerThreadCount(threadCount int) ConsumerOption {
	return func(opts *consumerOptions) {
		if threadCount < 0 {
			threadCount = 0
		}
		opts.ThreadCount = threadCount
	}
}

// 最大自动重入队次数, 默认为0表示使用配置的次数
func WithConsumerMaxAutoRequeueAttempts(attempts uint16) ConsumerOption {
	return func(opts *consumerOptions) {
		opts.MaxAutoRequeueAttempts = attempts
	}
}

package cron

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zly-app/zapp/pkg/utils"
)

var OutOfMaxConcurrentExecuteCount = errors.New("超出最大并发执行数")

// 错误回调, 只有会被重试时才会调用
type ErrCallback func(ctx IContext, err error)

type IExecutor interface {
	// 执行
	Do(onDo func(retryNums int) (IContext, error), errCallback ErrCallback) error
	// 等待任务执行完毕
	Wait()
	// 返回是否正在执行任务
	IsRunning() bool
}

type Executor struct {
	maxConcurrentExecuteCount int64         // 最大并发执行数, -1表示不限制
	concurrentExecuteCount    int64         // 当前并发执行数
	maxRetryCount             int64         // 重试次数
	retryInterval             time.Duration // 重试间隔
	wg                        sync.WaitGroup
}

// 创建一个执行器, 任务失败会重试
//
// retryCount: 任务失败重试次数
// retryInterval: 失败重试间隔时间
// maxConcurrentExecuteCount: 最大并发执行任务数, 如果为-1则不限制, 默认为1
func NewExecutor(retryCount int64, retryInterval time.Duration, maxConcurrentExecuteCount int64) IExecutor {
	if maxConcurrentExecuteCount == 0 {
		maxConcurrentExecuteCount = 1
	}
	return &Executor{
		maxConcurrentExecuteCount: maxConcurrentExecuteCount,
		concurrentExecuteCount:    0,
		maxRetryCount:             retryCount,
		retryInterval:             retryInterval,
	}
}

// 执行, 如果已经达到最大并发执行任务数则会返回错误
func (w *Executor) Do(onDo func(retryNums int) (IContext, error), errCallback ErrCallback) error {
	if w.maxConcurrentExecuteCount > 0 && atomic.AddInt64(&w.concurrentExecuteCount, 1) > w.maxConcurrentExecuteCount {
		atomic.AddInt64(&w.concurrentExecuteCount, -1) // 增加了要退回去
		return OutOfMaxConcurrentExecuteCount
	}

	w.wg.Add(1)

	err := w.doRetry(w.retryInterval, w.maxRetryCount, onDo, errCallback)

	if w.maxConcurrentExecuteCount > 0 {
		atomic.AddInt64(&w.concurrentExecuteCount, -1) // 执行完毕也要退回去
	}
	w.wg.Done()
	return err
}

// 等待所有任务执行完毕
func (w *Executor) Wait() {
	w.wg.Wait()
}

// 返回是否正在执行任务
func (w *Executor) IsRunning() bool {
	return atomic.LoadInt64(&w.concurrentExecuteCount) > 0
}

// 执行一个函数
func (w *Executor) doRetry(interval time.Duration, retryCount int64,
	onDo func(retryNums int) (IContext, error), errCallback ErrCallback) (err error) {
	var ctx IContext
	for {
		err = utils.Recover.WrapCall(func() error {
			ctx, err = onDo(int(retryCount))
			return err
		})
		if err == nil || retryCount == 0 {
			// 这里不需要错误回调, 如果有err交给调用者处理
			return
		}

		retryCount--

		if errCallback != nil {
			errCallback(ctx, err)
		}

		if interval > 0 {
			time.Sleep(interval)
		}
	}
}

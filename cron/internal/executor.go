package internal

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/zly-app/zapp/pkg/utils"

	"github.com/zly-app/service/cron/types"
)

// Executor 执行器实现
type Executor struct {
	maxConcurrentExecuteCount int64
	concurrentExecuteCount    int64
	maxRetryCount             int64
	retryInterval             time.Duration
	wg                        sync.WaitGroup
}

func NewExecutor(retryCount int64, retryInterval time.Duration, maxConcurrentExecuteCount int64) types.IExecutor {
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

func (w *Executor) Do(onDo func(remainAttempts int) (types.IContext, error), errCallback types.ErrCallback) error {
	if w.maxConcurrentExecuteCount > 0 && atomic.AddInt64(&w.concurrentExecuteCount, 1) > w.maxConcurrentExecuteCount {
		atomic.AddInt64(&w.concurrentExecuteCount, -1)
		return types.OutOfMaxConcurrentExecuteCount
	}

	if w.maxConcurrentExecuteCount <= 0 {
		atomic.AddInt64(&w.concurrentExecuteCount, 1)
	}

	w.wg.Add(1)

	err := w.doRetry(w.retryInterval, w.maxRetryCount, onDo, errCallback)

	atomic.AddInt64(&w.concurrentExecuteCount, -1)
	w.wg.Done()
	return err
}

func (w *Executor) Wait() {
	w.wg.Wait()
}

func (w *Executor) IsRunning() bool {
	return atomic.LoadInt64(&w.concurrentExecuteCount) > 0
}

func (w *Executor) doRetry(interval time.Duration, remainAttempts int64,
	onDo func(retryNums int) (types.IContext, error), errCallback types.ErrCallback) (err error) {
	var ctx types.IContext
	for {
		err = utils.Recover.WrapCall(func() error {
			ctx, err = onDo(int(remainAttempts))
			return err
		})
		if err == nil || remainAttempts == 0 {
			return
		}

		remainAttempts--

		if errCallback != nil {
			errCallback(ctx, err)
		}

		if interval > 0 {
			time.Sleep(interval)
		}
	}
}

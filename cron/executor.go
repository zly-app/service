package cron

import (
	"time"

	"github.com/zly-app/service/cron/internal"
	"github.com/zly-app/service/cron/types"
)

// IExecutor 执行器接口
type IExecutor = types.IExecutor

// ErrCallback 错误回调
type ErrCallback = types.ErrCallback

// OutOfMaxConcurrentExecuteCount 超出最大并发执行数错误
var OutOfMaxConcurrentExecuteCount = types.OutOfMaxConcurrentExecuteCount

// NewExecutor 创建执行器
func NewExecutor(retryCount int64, retryInterval time.Duration, maxConcurrentExecuteCount int64) IExecutor {
	return internal.NewExecutor(retryCount, retryInterval, maxConcurrentExecuteCount)
}

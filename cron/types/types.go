package types

import (
	"context"
	"errors"
	"time"

	"github.com/zly-app/zapp/core"
)

// OutOfMaxConcurrentExecuteCount 超出最大并发执行数错误
var OutOfMaxConcurrentExecuteCount = errors.New("超出最大并发执行数")

// Handler 任务处理函数
type Handler func(ctx IContext) error

// IContext 任务执行上下文接口
type IContext interface {
	// 获取task
	Task() ITask
	// 获取元数据
	Meta() interface{}
	// 设置元数据
	SetMeta(meta interface{})

	ILogger
	context.Context
}

// ILogger 日志接口，与 core.ILogger 兼容
type ILogger core.ILogger

// ITask 定时任务接口
type ITask interface {
	Name() string
	Handler() Handler
	IsEnable() bool
	Timeout() time.Duration
	TriggerTime() time.Time
	MakeNextTriggerTime(t time.Time) (time.Time, bool)
	Trigger(ctx context.Context) error
}

// ITrigger 触发器接口
type ITrigger interface {
	TriggerType() TriggerType
	Expression() string
	ResetClock()
	MakeNextTriggerTime(t time.Time) (time.Time, bool)
}

// TriggerType 触发器类型
type TriggerType int

const (
	// CronTriggerType Cron触发器
	CronTriggerType TriggerType = iota
	// OnceTriggerType 一次性触发器
	OnceTriggerType
)

// OnceTriggerTimeLayout 一次性触发器时间样式
const OnceTriggerTimeLayout = "2006-01-02 15:04:05"

// IExecutor 执行器接口
type IExecutor interface {
	Do(onDo func(retryNums int) (IContext, error), errCallback ErrCallback) error
	Wait()
	IsRunning() bool
}

// ErrCallback 错误回调
type ErrCallback func(ctx IContext, err error)

// TaskConfig 任务配置
type TaskConfig struct {
	Trigger  ITrigger
	Executor IExecutor
	Handler  Handler
	Timeout  time.Duration
	Enable   bool
}

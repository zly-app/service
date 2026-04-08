package internal

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zly-app/zapp/pkg/utils"

	"github.com/zly-app/service/cron/types"
)

// Task 定时任务实现
type Task struct {
	name    string
	handler types.Handler

	trigger   types.ITrigger
	executor  types.IExecutor
	timeout   time.Duration

	triggerTime time.Time
	mu          sync.Mutex

	enable atomic.Int32
}

// NewTaskOfConfig 创建任务
func NewTaskOfConfig(name string, config types.TaskConfig) types.ITask {
	t := &Task{
		name:     name,
		trigger:  config.Trigger,
		executor: config.Executor,
		handler:  config.Handler,
		timeout:  config.Timeout,
	}
	t.SetEnable(config.Enable)
	return t
}

func (t *Task) Name() string {
	return t.name
}

func (t *Task) Handler() types.Handler {
	return t.handler
}

func (t *Task) IsEnable() bool {
	return t.enable.Load() == 1
}

func (t *Task) Timeout() time.Duration {
	return t.timeout
}

func (t *Task) TriggerTime() time.Time {
	t.mu.Lock()
	tt := t.triggerTime
	t.mu.Unlock()
	return tt
}

func (t *Task) MakeNextTriggerTime(tt time.Time) (time.Time, bool) {
	if !t.IsEnable() {
		return tt, false
	}

	next, ok := t.trigger.MakeNextTriggerTime(tt)
	if ok {
		t.mu.Lock()
		t.triggerTime = next
		t.mu.Unlock()
	}
	return next, ok
}

func (t *Task) Trigger(ctx context.Context) error {
	return t.execute(ctx)
}

func (t *Task) execute(ctx context.Context) error {
	doCtx, span := utils.Trace.StartSpan(ctx, "cron/"+t.Name())
	defer utils.Trace.EndSpan(span)

	if t.timeout > 0 {
		var cancel context.CancelFunc
		doCtx, cancel = context.WithTimeout(doCtx, t.timeout)
		defer cancel()
	}

	onDo := func(retryNums int) (types.IContext, error) {
		iCtx := NewContext(doCtx, t)
		return iCtx, t.handler(iCtx)
	}
	return t.executor.Do(onDo, t.errCallback)
}

func (t *Task) errCallback(ctx types.IContext, err error) {
	// 简化实现，实际应该使用 logger
	_ = ctx
	_ = err
}

// ResetClock 重置触发器时钟
func (t *Task) ResetClock() {
	t.trigger.ResetClock()
}

// SetEnable 设置启用状态
func (t *Task) SetEnable(enable bool) {
	if enable {
		t.enable.Store(1)
	} else {
		t.enable.Store(0)
	}
}

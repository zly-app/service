package cron

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zly-app/zapp/pkg/utils"
	"go.uber.org/zap"
)

// ITask 定时任务接口
type ITask interface {
	// 返回任务名
	Name() string
	// 获取 handler
	Handler() Handler
	// 返回启用状态
	IsEnable() bool
	// 执行超时时间
	Timeout() time.Duration

	// 获取触发时间
	TriggerTime() time.Time
	// 生成下次触发时间, 如果返回了 false 表示没有下一次了, 返回的时间一定>t
	MakeNextTriggerTime(t time.Time) (time.Time, bool)
	// 立即触发执行, 阻塞等待执行结束
	Trigger(ctx context.Context) error
}

// heapTask 是任务堆的内部包装, 仅在 task_heap.go 中使用
// 它嵌入 ITask 并维护堆索引, 不实现 ITask 接口
type heapTask struct {
	task   ITask
	heapId int // -1 表示不在堆中
}

func (h *heapTask) TriggerTime() time.Time {
	return h.task.TriggerTime()
}

func (h *heapTask) IsEnable() bool {
	return h.task.IsEnable()
}

func (h *heapTask) Name() string {
	return h.task.Name()
}

// Task 是 ITask 的标准实现
type Task struct {
	name    string
	handler Handler

	trigger   ITrigger
	executor  IExecutor
	timeout   time.Duration // 执行超时

	triggerTime time.Time      // 下次触发时间
	mu          sync.Mutex     // 保护 triggerTime

	enable atomic.Int32 // 1=启用, 0=禁用
}

type TaskConfig struct {
	Trigger  ITrigger
	Executor IExecutor
	Handler  Handler
	Timeout  time.Duration // 执行超时
	Enable   bool
}

// NewTask 创建一个基于 cron 表达式的任务
func NewTask(name string, expression string, enable bool, handler Handler) ITask {
	return NewTaskOfConfig(name, TaskConfig{
		Trigger:  NewCronTrigger(expression),
		Executor: NewExecutor(0, 0, 1),
		Handler:  handler,
		Timeout:  0,
		Enable:   enable,
	})
}

// NewTaskOfConfig 根据配置创建一个任务
func NewTaskOfConfig(name string, config TaskConfig) ITask {
	t := &Task{
		name:     name,
		trigger:  config.Trigger,
		executor: config.Executor,
		handler:  config.Handler,
		timeout:  config.Timeout,
	}
	t.setEnable(config.Enable)
	return t
}

func (t *Task) Name() string {
	return t.name
}

func (t *Task) Handler() Handler {
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
	doCtx, span := utils.Trace.StartSpan(ctx, string(DefaultServiceType)+"/"+t.Name())
	defer utils.Trace.EndSpan(span)

	if t.timeout > 0 {
		var cancel context.CancelFunc
		doCtx, cancel = context.WithTimeout(doCtx, t.timeout)
		defer cancel()
	}

	onDo := func(retryNums int) (IContext, error) {
		iCtx := newContext(doCtx, t)
		return iCtx, t.handler(iCtx)
	}
	return t.executor.Do(onDo, t.errCallback)
}

func (t *Task) errCallback(ctx IContext, err error) {
	ctx.Warn("cron error, will retry", zap.Error(err))
}

// resetClock 重置触发器的时钟(内部方法)
func (t *Task) resetClock() {
	t.trigger.ResetClock()
}

// setEnable 设置启用状态(内部方法)
func (t *Task) setEnable(enable bool) {
	if enable {
		t.enable.Store(1)
	} else {
		t.enable.Store(0)
	}
}

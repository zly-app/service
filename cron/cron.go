/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2021/1/26
   Description :
-------------------------------------------------
*/

package cron

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zly-app/zapp/component/gpool"
	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/logger"
	"github.com/zly-app/zapp/pkg/utils"
	"github.com/zlyuancn/zutils"
	"go.uber.org/zap"
)

type ICron interface {
	// 运行状态
	RunState() RunState
	// 输出
	String() string

	// 启动
	Start()
	// 结束
	Stop()
	// 暂停所有任务
	Pause()
	// 恢复所有任务
	Resume()

	// 添加任务, 如果任务名重复返回 false
	AddTask(task ITask) bool
	// 移除任务
	RemoveTask(name string)
	// 启用任务
	EnableTask(task ITask, enable bool)
	// 获取任务名列表, 按名称排序
	TaskNames() []string
	// 获取任务列表, 按名称排序
	Tasks() []ITask
	// 获取任务, 如果不存在返回nil
	GetTask(name string) ITask
}

// 运行状态
type RunState int32

const (
	// 已停止
	StoppedState RunState = iota
	// 启动中
	StartingState
	// 暂停
	PausedState
	// 恢复中
	ResumingState
	// 已启动
	StartedState
	// 停止中
	StoppingState
)

func (r RunState) String() string {
	switch r {
	case StoppedState:
		return "stopped"
	case StartingState:
		return "starting"
	case PausedState:
		return "paused"
	case ResumingState:
		return "resuming"
	case StartedState:
		return "started"
	case StoppingState:
		return "stopping"
	}
	return fmt.Sprintf("undefined state: %d", r)
}

// =================================

const heapsCount = 64 // 任务堆数量

type CronService struct {
	app core.IApp

	tasks           map[string]ITask // 任务
	taskFileConfigs map[string]*TaskFileConfig
	heaps           []ITaskHeap // 任务堆列表, 根据触发时间取模将任务分配到不同的任务堆

	runState  RunState
	closeChan chan struct{}

	gpool core.IGPool // 协程池

	mx sync.Mutex // 锁 tasks, heaps
}

func NewCronService(app core.IApp) core.IService {
	conf := newConfig()
	vi := app.GetConfig().GetViper()
	confKey := "services." + string(nowServiceType)
	if vi.IsSet(confKey) {
		if err := vi.UnmarshalKey(confKey, conf); err != nil {
			app.Fatal(fmt.Errorf("无法解析<%s>服务配置: %s", nowServiceType, err))
		}
	}
	conf.check()

	c := &CronService{
		app:       app,
		tasks:     make(map[string]ITask),
		runState:  StoppedState,
		closeChan: make(chan struct{}),
	}
	c.remakeHeaps()
	if conf.ThreadCount > 0 {
		c.gpool = gpool.NewGPool(&gpool.GPoolConfig{
			JobQueueSize: conf.MaxTaskQueueSize,
			ThreadCount:  conf.ThreadCount,
		})
	}

	c.taskFileConfigs = make(map[string]*TaskFileConfig, len(conf.Tasks))
	for _, t := range conf.Tasks {
		c.taskFileConfigs[t.Name] = &t
	}
	return c
}

func (c *CronService) Inject(a ...interface{}) {
	for _, v := range a {
		task, ok := v.(ITask)
		if !ok {
			c.app.Fatal("Cron服务注入类型错误, 它必须能转为 cron.ITask")
		}

		if ok := c.AddTask(task); !ok {
			c.app.Fatal("添加Cron任务失败, 可能是名称重复", zap.String("name", task.Name()))
		}
	}
}

func (c *CronService) Start() error {
	if !atomic.CompareAndSwapInt32((*int32)(&c.runState), int32(StoppedState), int32(StartingState)) {
		return nil
	}
	c.app.Debug("cron服务正在启动")

	c.resetClock()
	go c.start()

	atomic.StoreInt32((*int32)(&c.runState), int32(StartedState))
	c.app.Debug("cron服务已启动")
	return nil
}

func (c *CronService) Close() error {
	state := atomic.LoadInt32((*int32)(&c.runState))
	if RunState(state) == StoppingState || RunState(state) == StoppedState {
		return nil
	}

	if !atomic.CompareAndSwapInt32((*int32)(&c.runState), state, int32(StoppingState)) {
		return nil
	}

	c.app.Debug("cron服务正在关闭")
	c.closeChan <- struct{}{}
	<-c.closeChan

	atomic.StoreInt32((*int32)(&c.runState), int32(StoppedState))
	c.app.Warn("cron服务已关闭")
	return nil
}

func (c *CronService) RunState() RunState {
	return RunState(atomic.LoadInt32((*int32)(&c.runState)))
}

func (c *CronService) Pause() {
	if c.RunState() != StartedState {
		return
	}

	if atomic.CompareAndSwapInt32((*int32)(&c.runState), int32(StartedState), int32(PausedState)) {
		c.app.Warn("暂停定时器")
	}
}

func (c *CronService) Resume() {
	if c.RunState() != StartedState {
		return
	}

	if atomic.CompareAndSwapInt32((*int32)(&c.runState), int32(PausedState), int32(ResumingState)) {
		c.resetClock()

		// 设为启动状态(恢复完成)
		// 这里使用cas是为了防止这个时候用户调用了Stop()+Start()后状态被更改
		if atomic.CompareAndSwapInt32((*int32)(&c.runState), int32(ResumingState), int32(StartedState)) {
			c.app.Info("恢复定时器")
		}
	}
}

func (c *CronService) AddTask(task ITask) bool {
	task = c.rebuildTask(task)
	c.mx.Lock()
	if _, ok := c.tasks[task.Name()]; ok { // 已存在
		c.mx.Unlock()
		return false
	}

	c.tasks[task.Name()] = task

	if task.IsEnable() && c.RunState() == StartedState {
		task.resetClock()
		_, ok := task.MakeNextTriggerTime(time.Now())
		if ok {
			c.pushTaskToHeap(task)
		}
	}

	c.mx.Unlock()
	return true
}

func (c *CronService) RemoveTask(name string) {
	c.mx.Lock()
	task, ok := c.tasks[name]
	if !ok {
		c.mx.Unlock()
		return
	}

	delete(c.tasks, name)

	heap := c.getHeapOfTime(task.TriggerTime().Unix())
	heap.Remove(task)

	c.mx.Unlock()
}

func (c *CronService) EnableTask(task ITask, enable bool) {
	c.mx.Lock()
	rawTask, ok := c.tasks[task.Name()]
	if !ok || rawTask != task {
		c.mx.Unlock()
		return
	}

	heap := c.getHeapOfTime(task.TriggerTime().Unix())
	heap.Remove(task)

	task.setEnable(enable)
	if enable && c.RunState() == StartedState {
		task.resetClock()
		_, ok := task.MakeNextTriggerTime(time.Now())
		if ok {
			c.pushTaskToHeap(task)
		}
	}
	c.mx.Unlock()
}

func (c *CronService) TaskNames() []string {
	c.mx.Lock()
	names := make([]string, len(c.tasks))
	var index int
	for name := range c.tasks {
		names[index] = name
		index++
	}
	c.mx.Unlock()

	sort.Strings(names)
	return names
}

func (c *CronService) Tasks() []ITask {
	c.mx.Lock()
	tasks := make([]ITask, len(c.tasks))
	var index int
	for _, task := range c.tasks {
		tasks[index] = task
		index++
	}
	c.mx.Unlock()

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Name() < tasks[j].Name()
	})
	return tasks
}

func (c *CronService) GetTask(name string) ITask {
	c.mx.Lock()
	task := c.tasks[name]
	c.mx.Unlock()
	return task
}

// 开始
func (c *CronService) start() {
	timer := time.NewTicker(time.Second)
	for {
		select {
		case t := <-timer.C:
			if c.isStarted() {
				go c.heartBeat(t)
			}
		case <-c.closeChan:
			timer.Stop()
			c.closeChan <- struct{}{}
			return
		}
	}
}

// 是否已开始
func (c *CronService) isStarted() bool {
	return atomic.LoadInt32((*int32)(&c.runState)) == int32(StartedState)
}

// 构建时间堆
func (c *CronService) remakeHeaps() {
	heaps := make([]ITaskHeap, heapsCount)
	for i := 0; i < heapsCount; i++ {
		heaps[i] = NewTaskHeap()
	}
	c.heaps = heaps
}

// 根据时间获取任务堆
func (c *CronService) getHeapOfTime(sec int64) ITaskHeap {
	bucket := sec & (heapsCount - 1)
	return c.heaps[bucket]
}

// 将任务放入任务堆中
func (c *CronService) pushTaskToHeap(task ITask) {
	heap := c.getHeapOfTime(task.TriggerTime().Unix())
	heap.Push(task)
}

// 心跳
func (c *CronService) heartBeat(t time.Time) {
	heap := c.getHeapOfTime(t.Unix())

	c.mx.Lock()
	defer c.mx.Unlock()

	for {
		if len(heap.Tasks()) == 0 { // 没有任务
			return
		}

		task := heap.Tasks()[0]
		if task.TriggerTime().After(t) { // 时间未到
			return
		}

		task = heap.Pop()
		c.triggerTask(task) // 触发

		// 获取下一次触发时间
		_, ok := task.MakeNextTriggerTime(t)
		if ok {
			c.pushTaskToHeap(task)
		}
	}
}

// 触发一个任务
func (c *CronService) triggerTask(t ITask) {
	if c.gpool == nil {
		go c.execute(t)
		return
	}

	ok := c.gpool.TryGo(func() error {
		c.execute(t)
		return nil
	}, nil)
	if !ok {
		c.app.Warn("cron.error", zap.String("task_name", t.Name()), zap.String("err", "tasks queue is full"))
	}
}

// 执行一个任务
func (c *CronService) execute(task ITask) {
	if !task.IsEnable() {
		return
	}

	baseCtx, span := utils.Otel.StartSpan(context.Background(), task.Name()+" trigger")
	defer utils.Otel.EndSpan(span)

	log := c.app.NewTraceLogger(baseCtx, zap.String("task_name", task.Name()))

	log.Debug("cron.start")
	err := task.Trigger(baseCtx)
	if err != nil {
		log.Error("cron.error!\n" + utils.Recover.GetRecoverErrorDetail(err))
	} else {
		log.Debug("cron.success")
	}
}

// 重置定时器
//
// 会重新创建任务堆列表并重新将所有任务加入堆中.
// 这里不要做任何耗时操作, 否则可能会错过下一秒的时间导致任务会延迟64秒后执行
func (c *CronService) resetClock() {
	c.mx.Lock()
	c.remakeHeaps()

	now := time.Now()
	for _, task := range c.tasks {
		if !task.IsEnable() {
			continue
		}

		task.resetClock()
		_, ok := task.MakeNextTriggerTime(now)
		if ok {
			c.pushTaskToHeap(task)
		}
	}
	c.mx.Unlock()
}

// 根据配置文件重建task
func (c *CronService) rebuildTask(task ITask) ITask {
	conf, ok := c.taskFileConfigs[task.Name()]
	if !ok {
		return task
	}

	var trigger ITrigger
	if conf.IsOnceTrigger {
		t, err := zutils.Time.TextToTimeOfLayout(conf.Expression, zutils.Time.Layout)
		if err != nil {
			logger.Log.Fatal("解析一次性任务失败",
				zap.String("task", task.Name()),
				zap.String("expression", conf.Expression),
				zap.Error(err),
			)
		}
		trigger = NewOnceTrigger(t)
	} else {
		trigger = NewCronTrigger(conf.Expression)
	}

	executor := NewExecutor(conf.RetryCount, time.Duration(conf.RetrySleepMs)*time.Second, conf.MaxConcurrentExecuteCount)
	return NewTaskOfConfig(task.Name(), TaskConfig{
		Trigger:  trigger,
		Executor: executor,
		Handler:  task.Handler(),
		TimeOut:  time.Duration(conf.TimeoutMs) * time.Second,
		Enable:   !conf.Disable,
	})
}

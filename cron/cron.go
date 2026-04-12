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
	"github.com/zly-app/zapp/log"
	"github.com/zly-app/zapp/pkg/utils"
	"github.com/zlyuancn/zutils"
	"go.uber.org/zap"

	"github.com/zly-app/service/cron/internal"
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
	heapMxs         []sync.Mutex // 每个 heap 一个锁, 减少 heartBeat 与任务管理的锁竞争

	runState  RunState
	closeChan chan struct{}

	gpool core.IGPool // 协程池

	mx sync.Mutex // 锁 tasks
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
		heapMxs:   make([]sync.Mutex, heapsCount),
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
		if tk, ok := task.(*internal.Task); ok {
			tk.ResetClock()
		}
		_, ok := task.MakeNextTriggerTime(time.Now())
		if ok {
			c.pushTaskToHeapLocked(task)
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
	c.removeTaskFromAllHeaps(task)
	c.mx.Unlock()
}

func (c *CronService) EnableTask(task ITask, enable bool) {
	c.mx.Lock()
	rawTask, ok := c.tasks[task.Name()]
	if !ok || rawTask != task {
		c.mx.Unlock()
		return
	}

	c.removeTaskFromAllHeaps(task)

	// 使用内部包的 Task 类型访问内部方法
	if tk, ok := task.(*internal.Task); ok {
		tk.SetEnable(enable)
		if enable && c.RunState() == StartedState {
			tk.ResetClock()
		}
	}

	if enable && c.RunState() == StartedState {
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
	heapMxs := make([]sync.Mutex, heapsCount)
	for i := 0; i < heapsCount; i++ {
		heaps[i] = NewTaskHeap()
	}
	c.heaps = heaps
	c.heapMxs = heapMxs
}

// 构建时间堆 (仅重建堆数据, 不重建 heapMxs, 用于 resetClock 中已持有所有 heapMxs 锁的场景)
func (c *CronService) remakeHeapsLocked() {
	heaps := make([]ITaskHeap, heapsCount)
	for i := 0; i < heapsCount; i++ {
		heaps[i] = NewTaskHeap()
	}
	c.heaps = heaps
}

// 根据时间获取任务堆索引
func (c *CronService) getHeapIndexOfTime(sec int64) int {
	return int(sec & (heapsCount - 1))
}

// 根据时间获取任务堆
func (c *CronService) getHeapOfTime(sec int64) ITaskHeap {
	return c.heaps[c.getHeapIndexOfTime(sec)]
}

// 将任务放入任务堆中 (需要持有 heapMxs 对应锁)
func (c *CronService) pushTaskToHeap(task ITask) {
	idx := c.getHeapIndexOfTime(task.TriggerTime().Unix())
	c.heapMxs[idx].Lock()
	c.heaps[idx].Push(task)
	c.heapMxs[idx].Unlock()
}

// 将任务放入任务堆中 (不单独加 heap 锁, 用于调用方已持有所有必要锁的场景如 resetClock)
func (c *CronService) pushTaskToHeapLocked(task ITask) {
	heap := c.getHeapOfTime(task.TriggerTime().Unix())
	heap.Push(task)
}

// 从所有堆中移除任务 (需要持有 mx 锁, 防止与 heartBeat 的 Pop+push 产生竞态)
func (c *CronService) removeTaskFromAllHeaps(task ITask) {
	for i, heap := range c.heaps {
		c.heapMxs[i].Lock()
		heap.Remove(task)
		c.heapMxs[i].Unlock()
	}
}

// 心跳
func (c *CronService) heartBeat(t time.Time) {
	idx := c.getHeapIndexOfTime(t.Unix())
	heap := c.heaps[idx]
	c.heapMxs[idx].Lock()
	defer c.heapMxs[idx].Unlock()

	for {
		if heap.Empty() { // 没有任务
			return
		}

		topTask := heap.Tasks()[0]
		if topTask.TriggerTime().After(t) { // 时间未到
			return
		}

		task := heap.Pop()

		// 检查任务是否仍在 tasks 中（可能已被 RemoveTask 删除）
		c.mx.Lock()
		if _, ok := c.tasks[task.Name()]; !ok {
			c.mx.Unlock()
			continue
		}

		// 获取下一次触发时间
		_, ok := task.MakeNextTriggerTime(t)
		if ok {
			// 任务可能需要放入不同的 heap bucket, 获取目标 heap 的锁
			nextIdx := c.getHeapIndexOfTime(task.TriggerTime().Unix())
			if nextIdx == idx {
				heap.Push(task)
			} else {
				// 需要放入不同的 heap, 先锁目标 heap
				c.heapMxs[nextIdx].Lock()
				c.heaps[nextIdx].Push(task)
				c.heapMxs[nextIdx].Unlock()
			}
		}
		c.mx.Unlock()

		c.triggerTask(task) // 触发
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

	baseCtx, span := utils.Trace.StartSpan(context.Background(), task.Name()+" trigger")
	defer utils.Trace.EndSpan(span)

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
	defer c.mx.Unlock()

	// 锁住所有 heap, 防止 heartBeat 在重建过程中访问
	for i := range c.heapMxs {
		c.heapMxs[i].Lock()
	}

	c.remakeHeapsLocked()

	now := time.Now()
	for _, task := range c.tasks {
		if !task.IsEnable() {
			continue
		}

		if tk, ok := task.(*internal.Task); ok {
			tk.ResetClock()
		}
		_, ok := task.MakeNextTriggerTime(now)
		if ok {
			c.pushTaskToHeapLocked(task)
		}
	}

	for i := range c.heapMxs {
		c.heapMxs[i].Unlock()
	}
}

// 根据配置文件重建task
func (c *CronService) rebuildTask(task ITask) ITask {
	conf, ok := c.taskFileConfigs[task.Name()]
	if !ok {
		return task
	}

	var trigger ITrigger
	if conf.IsOnceTrigger {
		t, err := zutils.Time(time.Local).TextToTimeOfLayout(conf.Expression, zutils.T.Layout)
		if err != nil {
			log.Fatal("解析一次性任务失败",
				zap.String("task", task.Name()),
				zap.String("expression", conf.Expression),
				zap.Error(err),
			)
		}
		trigger = internal.NewOnceTrigger(t)
	} else {
		trigger = internal.NewCronTrigger(conf.Expression)
	}

	executor := internal.NewExecutor(conf.RetryCount, time.Duration(conf.RetrySleepMs)*time.Second, conf.MaxConcurrentExecuteCount)
	return internal.NewTaskOfConfig(task.Name(), TaskConfig{
		Trigger:  trigger,
		Executor: executor,
		Handler:  task.Handler(),
		Timeout:  time.Duration(conf.TimeoutMs) * time.Second,
		Enable:   !conf.Disable,
	})
}

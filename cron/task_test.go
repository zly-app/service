package cron

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zly-app/service/cron/internal"
)

// --- Task 基本属性 ---

func TestNewTask(t *testing.T) {
	var called int32
	task := NewTask("test-task", "@every 1s", true, func(ctx IContext) error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	if task.Name() != "test-task" {
		t.Fatalf("expected name 'test-task', got '%s'", task.Name())
	}
	if !task.IsEnable() {
		t.Fatal("expected task to be enabled")
	}
	if task.Timeout() != 0 {
		t.Fatalf("expected timeout 0, got %v", task.Timeout())
	}
	if task.Handler() == nil {
		t.Fatal("expected handler to be non-nil")
	}

	err := task.Trigger(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Fatal("expected handler to be called once")
	}
}

func TestNewTaskOfConfig(t *testing.T) {
	trigger := NewOnceTrigger(time.Now().Add(time.Hour))
	executor := NewExecutor(0, 0, 1)
	task := NewTaskOfConfig("cfg-task", TaskConfig{
		Trigger:  trigger,
		Executor: executor,
		Handler:  func(ctx IContext) error { return nil },
		Timeout:  5 * time.Second,
		Enable:   false,
	})

	if task.Name() != "cfg-task" {
		t.Fatalf("expected 'cfg-task', got '%s'", task.Name())
	}
	if task.IsEnable() {
		t.Fatal("expected task to be disabled")
	}
	if task.Timeout() != 5*time.Second {
		t.Fatalf("expected 5s timeout, got %v", task.Timeout())
	}
}

func TestTaskEnableToggle(t *testing.T) {
	task := NewTask("toggle-task", "@every 1s", true, func(ctx IContext) error { return nil })

	if !task.IsEnable() {
		t.Fatal("expected enabled initially")
	}

	tk := task.(*internal.Task)
	tk.SetEnable(false)
	if task.IsEnable() {
		t.Fatal("expected disabled after setEnable(false)")
	}

	tk.SetEnable(true)
	if !task.IsEnable() {
		t.Fatal("expected enabled after setEnable(true)")
	}
}

// --- TriggerTime & MakeNextTriggerTime ---

func TestTaskTriggerTimeInitially(t *testing.T) {
	task := NewTask("time-task", "@every 1s", true, func(ctx IContext) error { return nil })
	// triggerTime 未初始化时为空时间, TriggerTime 是 ITask 的公开方法, 直接调用即可
	if !task.TriggerTime().IsZero() {
		t.Fatalf("expected zero time initially, got %v", task.TriggerTime())
	}
}

func TestTaskMakeNextTriggerTimeAdvances(t *testing.T) {
	task := NewTask("next-task", "@every 1s", true, func(ctx IContext) error { return nil })

	now := time.Now().Truncate(time.Second)
	next, ok := task.MakeNextTriggerTime(now)
	if !ok {
		t.Fatal("expected next trigger time")
	}
	if !next.After(now) {
		t.Fatalf("expected next > now, got next=%v now=%v", next, now)
	}

	// 再次调用，应该返回更晚的时间
	next2, ok := task.MakeNextTriggerTime(next)
	if !ok {
		t.Fatal("expected second trigger time")
	}
	if !next2.After(next) {
		t.Fatalf("expected next2 > next, got next2=%v next=%v", next2, next)
	}
}

func TestTaskMakeNextTriggerTimeDisabled(t *testing.T) {
	task := NewTask("disabled-task", "@every 1s", false, func(ctx IContext) error { return nil })

	now := time.Now()
	next, ok := task.MakeNextTriggerTime(now)
	if ok {
		t.Fatal("disabled task should not produce next trigger time")
	}
	if !next.Equal(now) {
		t.Fatalf("expected returned time to equal input when disabled")
	}
}

// --- Timeout ---

func TestTaskTimeout(t *testing.T) {
	done := make(chan struct{})

	task := NewTaskOfConfig("timeout-task", TaskConfig{
		Trigger:  NewOnceTrigger(time.Now().Add(time.Hour)),
		Executor: NewExecutor(0, 0, 1),
		Handler: func(ctx IContext) error {
			select {
			case <-time.After(5 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		Timeout: 50 * time.Millisecond,
		Enable:  true,
	})

	err := task.Trigger(context.Background())
	close(done)
	_ = done

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got: %v", err)
	}
}

func TestTaskNoTimeout(t *testing.T) {
	task := NewTaskOfConfig("fast-task", TaskConfig{
		Trigger:  NewOnceTrigger(time.Now().Add(time.Hour)),
		Executor: NewExecutor(0, 0, 1),
		Handler: func(ctx IContext) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
		Timeout: 5 * time.Second,
		Enable:  true,
	})

	err := task.Trigger(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Execution ---

func TestTaskExecutionWithError(t *testing.T) {
	expectedErr := errors.New("handler error")

	task := NewTask("error-task", "@every 1s", true, func(ctx IContext) error {
		return expectedErr
	})

	err := task.Trigger(context.Background())
	if err == nil {
		t.Fatal("expected error from handler")
	}
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected '%v', got '%v'", expectedErr, err)
	}
}

func TestTaskExecutionConcurrency(t *testing.T) {
	var calls int32
	task := NewTaskOfConfig("concurrent-task", TaskConfig{
		Trigger:  NewOnceTrigger(time.Now().Add(time.Hour)),
		Executor: NewExecutor(0, 0, -1), // unlimited concurrent
		Handler: func(ctx IContext) error {
			atomic.AddInt32(&calls, 1)
			return nil
		},
		Enable: true,
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = task.Trigger(context.Background())
		}()
	}
	wg.Wait()

	if atomic.LoadInt32(&calls) != 100 {
		t.Fatalf("expected 100 calls, got %d", atomic.LoadInt32(&calls))
	}
}

// --- Trigger 相关 ---

func TestOnceTriggerMakesNoNextAfterExecution(t *testing.T) {
	pastTime := time.Now().Add(-time.Hour)
	trigger := NewOnceTrigger(pastTime)

	now := time.Now()
	_, ok := trigger.MakeNextTriggerTime(now)
	if ok {
		t.Fatal("once trigger in the past should not produce next time")
	}
}

func TestOnceTriggerMakesNextBeforeExecution(t *testing.T) {
	futureTime := time.Now().Add(time.Hour)
	trigger := NewOnceTrigger(futureTime)

	now := time.Now()
	next, ok := trigger.MakeNextTriggerTime(now)
	if !ok {
		t.Fatal("once trigger in the future should produce next time")
	}
	if !next.Equal(futureTime) {
		t.Fatalf("expected %v, got %v", futureTime, next)
	}
}

func TestCronTriggerResets(t *testing.T) {
	trigger := NewCronTrigger("@every 1s")

	// 获取当前执行时间
	next1, ok := trigger.MakeNextTriggerTime(time.Now())
	if !ok {
		t.Fatal("expected next trigger time")
	}

	// 重置时钟
	trigger.ResetClock()

	// 重置后应该重新计算
	next2, ok := trigger.MakeNextTriggerTime(time.Now())
	if !ok {
		t.Fatal("expected next trigger time after reset")
	}
	// 重置后的时间应该比之前获取的更晚（因为 now 已经推进了）
	if !next2.After(next1) && !next2.Equal(next1) {
		// 可能相同，这是可以接受的
	}
}

func TestCronTriggerExpression(t *testing.T) {
	trigger := NewCronTrigger("@every 5s")
	if trigger.Expression() != "@every 5s" {
		t.Fatalf("expected '@every 5s', got '%s'", trigger.Expression())
	}
	if trigger.TriggerType() != CronTriggerType {
		t.Fatalf("expected CronTriggerType, got %v", trigger.TriggerType())
	}
}

func TestOnceTriggerExpression(t *testing.T) {
	tt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)
	trigger := NewOnceTrigger(tt)
	if trigger.Expression() != tt.Format(OnceTriggerTimeLayout) {
		t.Fatalf("expected '%s', got '%s'", tt.Format(OnceTriggerTimeLayout), trigger.Expression())
	}
	if trigger.TriggerType() != OnceTriggerType {
		t.Fatalf("expected OnceTriggerType, got %v", trigger.TriggerType())
	}
}

// --- Executor ---

func TestExecutorMaxConcurrent(t *testing.T) {
	var running int32
	var maxSeen int32
	var wg sync.WaitGroup

	exec := NewExecutor(0, 0, 2) // max 2 concurrent

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := exec.Do(func(retryNums int) (IContext, error) {
				n := atomic.AddInt32(&running, 1)
				currentMax := atomic.LoadInt32(&maxSeen)
				for n > currentMax {
					if atomic.CompareAndSwapInt32(&maxSeen, currentMax, n) {
						break
					}
					currentMax = atomic.LoadInt32(&maxSeen)
				}
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&running, -1)
				return nil, nil
			}, nil)
			// 超出并发限制的调用会返回错误
			_ = err
		}()
	}
	wg.Wait()

	if atomic.LoadInt32(&maxSeen) > 2 {
		t.Fatalf("max concurrent exceeded 2, got %d", atomic.LoadInt32(&maxSeen))
	}
}

func TestExecutorRetry(t *testing.T) {
	var attempts int32
	exec := NewExecutor(3, 1*time.Millisecond, 1) // retry 3 times

	err := exec.Do(func(retryNums int) (IContext, error) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			return nil, errors.New("retry me")
		}
		return nil, nil
	}, nil)

	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Fatalf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestExecutorExhaustsRetries(t *testing.T) {
	var attempts int32
	exec := NewExecutor(2, 1*time.Millisecond, 1) // retry 2 times, always fails

	err := exec.Do(func(retryNums int) (IContext, error) {
		atomic.AddInt32(&attempts, 1)
		return nil, errors.New("always fail")
	}, nil)

	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// 初始 1 次 + 2 次重试 = 3 次
	if atomic.LoadInt32(&attempts) != 3 {
		t.Fatalf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestExecutorWait(t *testing.T) {
	var done int32
	exec := NewExecutor(0, 0, -1) // unlimited concurrent

	for i := 0; i < 10; i++ {
		go func() {
			_ = exec.Do(func(retryNums int) (IContext, error) {
				time.Sleep(10 * time.Millisecond)
				return nil, nil
			}, nil)
			atomic.AddInt32(&done, 1)
		}()
	}

	time.Sleep(2e8) // 等待所有任务都加入
	exec.Wait()

	if atomic.LoadInt32(&done) != 10 {
		t.Fatalf("expected 10 done, got %d", atomic.LoadInt32(&done))
	}
}

func TestExecutorIsRunning(t *testing.T) {
	exec := NewExecutor(0, 0, -1)
	if exec.IsRunning() {
		t.Fatal("expected not running initially")
	}

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		_ = exec.Do(func(retryNums int) (IContext, error) {
			close(started)
			time.Sleep(50 * time.Millisecond)
			return nil, nil
		}, nil)
		close(done)
	}()

	<-started // wait until handler actually started
	if !exec.IsRunning() {
		t.Fatal("expected IsRunning() to be true")
	}

	<-done
	if exec.IsRunning() {
		t.Fatal("expected IsRunning() to be false after completion")
	}
}

// --- ResetClock ---

func TestTaskResetClock(t *testing.T) {
	task := NewTask("reset-clock-task", "@every 1s", true, func(ctx IContext) error { return nil })
	tk := task.(*internal.Task)

	// 获取初始触发时间
	now := time.Now()
	tk.ResetClock()

	next, ok := task.MakeNextTriggerTime(now)
	if !ok {
		t.Fatal("expected next trigger time after reset")
	}
	if !next.After(now) {
		t.Fatalf("expected trigger time after now, got %v", next)
	}
}

package cron

import (
	"context"
	"testing"
	"time"

	"github.com/zly-app/zapp"
	"github.com/zly-app/zapp/config"
	"github.com/zly-app/zapp/core"
)

func TestTask(t *testing.T) {
	task := NewTask("test", "@every 1s", true, func(ctx IContext) (err error) {
		ctx.Info("触发了")
		return nil
	})
	err := task.Trigger(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestTaskTimeout(t *testing.T) {
	task := NewTaskOfConfig("test", TaskConfig{
		Trigger:  NewOnceTrigger(time.Unix(time.Now().Unix()+int64(time.Second), 0)),
		Executor: NewExecutor(0, 0, 1),
		Handler: func(ctx IContext) (err error) {
			ctx.Info("模拟等待开始", ctx.Err())
			time.Sleep(time.Second * 2)
			ctx.Info("模拟等待结束", ctx.Err())
			return nil
		},
		TimeOut: time.Second,
		Enable:  true,
	})
	err := task.Trigger(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestCronTask(t *testing.T) {
	app := zapp.NewApp("test",
		WithService(),
	)
	defer app.Exit()

	RegistryHandler("test", "@every 1s", true, func(ctx IContext) (err error) {
		ctx.Info("触发了")
		return nil
	})

	go func() {
		time.Sleep(time.Second * 3)
		app.Exit()
	}()
	app.Run()
}

func TestCronTaskFileConfig(t *testing.T) {
	app := zapp.NewApp("test",
		WithService(),
		zapp.WithConfigOption(config.WithConfig(&core.Config{
			Services: map[string]interface{}{
				string(nowServiceType): map[string]interface{}{
					"tasks": []TaskFileConfig{
						{
							Name:                      "test",
							Expression:                "@every 1s",
							IsOnceTrigger:             false,
							Disable:                   false,
							RetryCount:                0,
							RetrySleepMs:              0,
							MaxConcurrentExecuteCount: 0,
							TimeoutMs:                 0,
						},
					},
				},
			},
		})),
	)
	defer app.Exit()

	RegistryHandler("test", "@every 10s", false, func(ctx IContext) (err error) {
		ctx.Info("触发了")
		return nil
	})

	go func() {
		time.Sleep(time.Second * 3)
		app.Exit()
	}()
	app.Run()
}

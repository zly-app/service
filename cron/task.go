package cron

import (
	"github.com/zly-app/service/cron/internal"
	"github.com/zly-app/service/cron/types"
)

// ITask 定时任务接口
type ITask = types.ITask

// TaskConfig 任务配置
type TaskConfig = types.TaskConfig

// NewTask 创建一个基于 cron 表达式的任务
func NewTask(name string, expression string, enable bool, handler types.Handler) ITask {
	return NewTaskOfConfig(name, TaskConfig{
		Trigger:  internal.NewCronTrigger(expression),
		Executor: internal.NewExecutor(0, 0, 1),
		Handler:  handler,
		Timeout:  0,
		Enable:   enable,
	})
}

// NewTaskOfConfig 根据配置创建一个任务
func NewTaskOfConfig(name string, config TaskConfig) ITask {
	return internal.NewTaskOfConfig(name, config)
}

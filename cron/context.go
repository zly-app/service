package cron

import (
	"context"

	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/logger"
	"go.uber.org/zap"
)

type Handler func(ctx IContext) (err error)

type IContext interface {
	// 获取task
	Task() ITask
	// 获取元数据
	Meta() interface{}
	// 设置元数据
	SetMeta(meta interface{})

	core.ILogger
	context.Context
}

type Context struct {
	task    ITask
	handler Handler
	meta    interface{}
	core.ILogger
	context.Context
}

func newContext(ctx context.Context, task ITask) IContext {
	log := logger.Log.NewTraceLogger(ctx, zap.String("task_name", task.Name()))
	return &Context{
		task:    task,
		handler: task.Handler(),
		meta:    nil,
		ILogger: log,
		Context: ctx,
	}
}

func (ctx *Context) Task() ITask {
	return ctx.task
}

func (ctx *Context) Meta() interface{} {
	return ctx.meta
}
func (ctx *Context) SetMeta(meta interface{}) {
	ctx.meta = meta
}

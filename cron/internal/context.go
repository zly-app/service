package internal

import (
	"context"

	"github.com/zly-app/zapp/log"
	"go.uber.org/zap"

	"github.com/zly-app/service/cron/types"
)

// Context 任务执行上下文实现
type Context struct {
	task    types.ITask
	handler types.Handler
	meta    interface{}
	types.ILogger
	context.Context
}

func NewContext(ctx context.Context, task types.ITask) types.IContext {
	// 直接使用 log.Log.NewTraceLogger，它返回的是 core.ILogger
	zlogger := log.Log.NewTraceLogger(ctx, zap.String("task_name", task.Name()))
	//logger := &loggerAdapter{ILogger: zlog.GetLogCore(zlogger)}
	return &Context{
		task:    task,
		handler: task.Handler(),
		meta:    nil,
		ILogger: zlogger,
		Context: ctx,
	}
}

func (ctx *Context) Task() types.ITask {
	return ctx.task
}

func (ctx *Context) Meta() interface{} {
	return ctx.meta
}

func (ctx *Context) SetMeta(meta interface{}) {
	ctx.meta = meta
}

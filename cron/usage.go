/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2021/1/23
   Description :
-------------------------------------------------
*/

package cron

import (
	"time"

	"github.com/zly-app/zapp"
	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/service"
)

// DefaultServiceType 默认服务类型
const DefaultServiceType core.ServiceType = "cron"

// WithService 启用 cron 服务，必须在 zapp 初始化前调用。
func WithService() zapp.Option {
	service.RegisterCreatorFunc(DefaultServiceType, func(app core.IApp) core.IService {
		return NewCronService(app)
	})
	return zapp.WithService(DefaultServiceType)
}

// Install 返回 cron 注册器，必须在 zapp 初始化后调用（例如 app.Run 之前）。
func Install(app core.IApp) *Registrar {
	return &Registrar{app: app}
}

// Registrar 是 cron 注册器：拥有「向 cron 服务注入任务」这条规则。
type Registrar struct {
	app core.IApp
}

// RegistryHandler 注册cron的Handler
func (r *Registrar) RegistryHandler(name string, expression string, enable bool, handler Handler) {
	task := NewTaskOfConfig(name, TaskConfig{
		Trigger:  NewCronTrigger(expression),
		Executor: NewExecutor(0, 0, 1),
		Handler:  handler,
		Enable:   enable,
	})
	r.app.InjectService(DefaultServiceType, task)
}

// RegistryOnceHandler 注册一次性cron的Handler
func (r *Registrar) RegistryOnceHandler(name string, t time.Time, enable bool, handler Handler) {
	task := NewTaskOfConfig(name, TaskConfig{
		Trigger:  NewOnceTrigger(t),
		Executor: NewExecutor(0, 0, 1),
		Handler:  handler,
		Enable:   enable,
	})
	r.app.InjectService(DefaultServiceType, task)
}

// RegistryTask 注册自定义task
func (r *Registrar) RegistryTask(task ITask) {
	r.app.InjectService(DefaultServiceType, task)
}

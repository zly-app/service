/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2021/1/23
   Description :
-------------------------------------------------
*/

package mysql_binlog

import (
	"github.com/zly-app/zapp"
	"github.com/zly-app/zapp/core"
	"github.com/zly-app/zapp/service"
)

// DefaultServiceType 默认服务类型
const DefaultServiceType core.ServiceType = "mysql-binlog"

// WithService 启用 mysql-binlog 服务，必须在 zapp 初始化前调用。
func WithService() zapp.Option {
	service.RegisterCreatorFunc(DefaultServiceType, func(app core.IApp) core.IService {
		return NewMysqlBinlogService(app)
	})
	return zapp.WithService(DefaultServiceType)
}

// Install 返回事件注册器，必须在 zapp 初始化后调用（例如 app.Run 之前）。
func Install(app core.IApp) *Registrar {
	return &Registrar{app: app}
}

// Registrar 是 binlog 事件注册器。
type Registrar struct {
	app core.IApp
}

// RegistryHandler 注册handler
func (r *Registrar) RegistryHandler(handler IEventHandler) {
	r.app.InjectService(DefaultServiceType, handler)
}

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

// 默认服务类型
const DefaultServiceType core.ServiceType = "mysql-binlog"

// 当前服务类型
var nowServiceType = DefaultServiceType

// 设置服务类型, 这个函数应该在 zapp.NewApp 之前调用
func SetServiceType(t core.ServiceType) {
	nowServiceType = t
}

func init() {
	service.RegisterCreatorFunc(nowServiceType, func(app core.IApp) core.IService {
		return NewMysqlBinlogService(app)
	})
}

// 启用mysql-binlog服务
func WithService() zapp.Option {
	return zapp.WithService(nowServiceType)
}

// 注册handler
func RegistryHandler(handler IEventHandler) {
	zapp.App().InjectService(nowServiceType, handler)
}

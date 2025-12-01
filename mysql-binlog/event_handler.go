/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2020/12/3
   Description :
-------------------------------------------------
*/

package mysql_binlog

import (
	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/zly-app/zapp/log"
	"go.uber.org/zap"
)

const (
	UpdateAction = canal.UpdateAction // 更新操作
	InsertAction = canal.InsertAction // 插入操作
	DeleteAction = canal.DeleteAction // 删除操作
)
const (
	// 最旧的位置, 表示从头开始消费
	//
	// 注意, 如果表曾经被改变过且表在改变之前产生了row事件, 那么可能导致意想不到的错误.
	//      因为表的结构是以最新结构为准, 但是表改变之前的row事件的数据是以表被改变之前的结构为准.
	OldestPos = "oldest"
	// 最新的位置, 表示忽略之前的记录, 只会mysql-binlog服务启动后的记录
	LatestPos = "latest"
)

type IEventHandler interface {
	// 获取开始处理的位置, 可以使用 OldestPos 或 LatestPos 表示从最旧或最新的记录开始分析
	GetStartPos() (binlogName string, pos uint32, err error)
	// 事件解析错误回调, 如果不允许skip则结束服务
	OnEventParseErr(event *canal.RowsEvent, err error) (skip bool)
	// 表改变
	OnTableChanged(schema, table, sql string)
	// ROW 事件
	OnRow(records []*Record)
	// 要求记录位置同步
	//
	// force会在表改变时设为true, 此时要求必须同步成功, 如果同步失败会结束服务.
	// 服务结束之前会自动再调用一次这个方法.
	OnPosSynced(binlogName string, pos uint32, force bool) error
}

func (m *MysqlBinlogService) OnRotate(*replication.RotateEvent) error { return nil }
func (m *MysqlBinlogService) OnTableChanged(schema string, table string) error {
	m.oldSchema, m.oldTable = schema, table
	return nil
}
func (m *MysqlBinlogService) OnDDL(_ mysql.Position, queryEvent *replication.QueryEvent) error {
	m.handler.OnTableChanged(m.oldSchema, m.oldTable, string(queryEvent.Query))
	return nil
}
func (m *MysqlBinlogService) OnXID(mysql.Position) error { return nil }
func (m *MysqlBinlogService) OnGTID(mysql.GTIDSet) error { return nil }
func (m *MysqlBinlogService) OnRow(event *canal.RowsEvent) error {
	records, err := m.analyzer.MakeRecords(event)
	if err != nil {
		skip := m.handler.OnEventParseErr(event, err)
		if !skip {
			m.app.Fatal("解析event失败", zap.Any("event", event), zap.Error(err))
		}
		return nil
	}

	m.handler.OnRow(records)
	return nil
}
func (m *MysqlBinlogService) OnPosSynced(pos mysql.Position, _ mysql.GTIDSet, force bool) error {
	err := m.handler.OnPosSynced(pos.Name, pos.Pos, force)
	if err == nil {
		return nil
	}
	if force {
		m.app.Fatal("mysql-binlog pos synced error", zap.Error(err))
	}
	m.app.Warn("mysql-binlog pos synced error", zap.Error(err))
	return nil
}
func (m *MysqlBinlogService) String() string { return "CanalEventHandler" }

var _ IEventHandler = (*BaseEventHandler)(nil)

type BaseEventHandler struct{}

func (b *BaseEventHandler) GetStartPos() (binlogName string, pos uint32, err error) {
	return LatestPos, 0, nil
}
func (b *BaseEventHandler) OnEventParseErr(event *canal.RowsEvent, err error) (skip bool) {
	log.Warn("OnEventParseErr", zap.Any("event", event), zap.Error(err))
	return true
}
func (b *BaseEventHandler) OnTableChanged(schema, table, sql string) {
	log.Debug("OnTableChanged", zap.String("schema", schema), zap.String("table", table), zap.String("sql", sql))
}
func (b *BaseEventHandler) OnRow(records []*Record) {
	for i, r := range records {
		log.Debug("OnRow", i, r.String())
	}
}
func (b *BaseEventHandler) OnPosSynced(binlogName string, pos uint32, force bool) error {
	log.Debug("OnPosSynced", zap.String("binlogName", binlogName), zap.Uint32("pos", pos), zap.Bool("force", force))
	return nil
}

/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2020/12/3
   Description :
-------------------------------------------------
*/

package mysql_binlog

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/mysql"
	"go.uber.org/zap"

	"github.com/zly-app/zapp/core"
)

type MysqlBinlogService struct {
	app core.IApp

	canal               *canal.Canal
	analyzer            *analyzer
	oldSchema, oldTable string

	handler IEventHandler
}

func NewMysqlBinlogService(app core.IApp) core.IService {
	return &MysqlBinlogService{
		app: app,
	}
}

func (m *MysqlBinlogService) Inject(a ...interface{}) {
	if m.handler != nil {
		m.app.Fatal("mysql-binlog服务重复注入")
	}

	if len(a) != 1 {
		m.app.Fatal("mysql-binlog服务注入数量必须为1个")
	}

	var ok bool
	m.handler, ok = a[0].(IEventHandler)
	if !ok {
		m.app.Fatal("mysql-binlog服务注入类型错误, 它必须能转为 mysql_binlog.RegistryMysqlBinlogHandlerFunc")
	}
}

func (m *MysqlBinlogService) Start() error {
	if m.handler == nil {
		return errors.New("未注入handler")
	}

	conf := newConfig()
	err := m.app.GetConfig().ParseServiceConfig(DefaultServiceType, conf)
	if err == nil {
		err = conf.Check()
	}
	if err != nil {
		return fmt.Errorf("服务配置错误: %v", err)
	}

	// 构建配置
	cfg := &canal.Config{
		Addr:                  conf.Host,
		User:                  conf.UserName,
		Password:              conf.Password,
		Charset:               "utf8mb4",
		ServerID:              uint32(rand.New(rand.NewSource(time.Now().Unix())).Intn(1000)) + 1001,
		Flavor:                "mysql",
		DiscardNoMetaRowEvent: conf.DiscardNoMetaRowEvent,
		Dump: canal.DumpConfig{
			ExecutionPath:  conf.DumpExecutionPath,
			DiscardErr:     true,
			SkipMasterData: false,
		},
	}
	cfg.Charset = conf.Charset
	if len(conf.IncludeTableRegex) > 0 {
		cfg.IncludeTableRegex = append([]string{}, conf.IncludeTableRegex...)
	}
	if len(conf.ExcludeTableRegex) > 0 {
		cfg.ExcludeTableRegex = append([]string{}, conf.ExcludeTableRegex...)
	}

	// 创建canal组件
	ca, err := canal.NewCanal(cfg)
	if err != nil {
		return fmt.Errorf("创建canal组件失败: %v", err)
	}

	ca.SetEventHandler(m)
	m.canal = ca
	m.analyzer = newAnalyzer(m.app, conf.IgnoreWKBDataParseError) // 分析器

	// 获取位置
	binlogName, pos, err := m.handler.GetStartPos()
	if err != nil {
		return err
	}

	// 处理位置
	switch binlogName {
	case OldestPos: // 最旧的位置
		m.app.Debug("mysql-bing服务启动中, 将从最旧位置开始处理")
		return m.canal.Run()
	case LatestPos: // 最新的位置
		pos, err := m.canal.GetMasterPos()
		if err != nil {
			return fmt.Errorf("获取bing最新位置失败: %v", err)
		}
		m.app.Debug("mysql-bing服务启动中, 将从最新位置开始处理", zap.String("binlogName", pos.Name), zap.Uint32("pos", pos.Pos))
		_ = m.OnPosSynced(pos, nil, true)
		return m.canal.RunFrom(pos)
	default: // 指定位置
		p := mysql.Position{Name: binlogName, Pos: pos}
		m.app.Debug("mysql-bing服务启动中, 将从指定位置开始处理", zap.String("binlogName", binlogName), zap.Uint32("pos", pos))
		_ = m.OnPosSynced(p, nil, false)
		return m.canal.RunFrom(p)
	}
}

func (m *MysqlBinlogService) Close() error {
	m.canal.Close()
	return nil
}

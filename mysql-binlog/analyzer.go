/*
-------------------------------------------------
   Author :       zlyuancn
   date：         2020/12/3
   Description :
-------------------------------------------------
*/

package mysql_binlog

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/schema"
	"github.com/twpayne/go-geom/encoding/geojson"
	"github.com/twpayne/go-geom/encoding/wkb"
	"github.com/zlyuancn/zstr"
	"go.uber.org/zap"

	"github.com/zly-app/zapp/core"
)

// 分析器
type analyzer struct {
	app                     core.IApp
	IgnoreWKBDataParseError bool // 忽略wkb数据解析错误, 一般为POINT, GEOMETRY类型
}

func newAnalyzer(app core.IApp, ignoreWKBDataParseError bool) *analyzer {
	return &analyzer{
		app:                     app,
		IgnoreWKBDataParseError: ignoreWKBDataParseError,
	}
}

// 构建记录
func (a *analyzer) MakeRecords(event *canal.RowsEvent) (records []*Record, err error) {
	if event.Action == UpdateAction {
		return a.makeUpdateRecords(event)
	}

	records = make([]*Record, len(event.Rows))
	for i, row := range event.Rows {
		var m map[string]interface{}
		m, err = a.parseLine(event.Table, row)
		if err != nil {
			return nil, fmt.Errorf("无法解析row: %d: %s", i, err)
		}

		record := &Record{
			Action:    event.Action,
			Old:       nil,
			New:       nil,
			DbName:    event.Table.Schema,
			TableName: event.Table.Name,
			Timestamp: event.Header.Timestamp,
		}
		records[i] = record

		switch event.Action {
		case InsertAction:
			record.New = m
		case DeleteAction:
			record.Old = m
		}

	}
	return records, nil
}

// 构建更新记录
func (a *analyzer) makeUpdateRecords(event *canal.RowsEvent) (records []*Record, err error) {
	if 1&len(event.Rows) == 1 {
		return nil, fmt.Errorf("update的rows数量应该为2的整数, 但收到了%d条row", len(records))
	}

	records = make([]*Record, 0, len(event.Rows)/2)
	for i := 0; i < len(event.Rows); i += 2 {
		old_row, row := event.Rows[i], event.Rows[i+1]

		record := &Record{
			Action:    event.Action,
			Old:       nil,
			New:       nil,
			DbName:    event.Table.Schema,
			TableName: event.Table.Name,
			Timestamp: event.Header.Timestamp,
		}
		records = append(records, record)

		record.Old, err = a.parseLine(event.Table, old_row)
		if err != nil {
			return nil, fmt.Errorf("无法解析row: %d: %s", i, err)
		}

		record.New, err = a.parseLine(event.Table, row)
		if err != nil {
			return nil, fmt.Errorf("无法解析row: %d: %s", i+1, err)
		}
	}
	return records, nil
}

// 解析一条row数据
func (a *analyzer) parseLine(table *schema.Table, row []interface{}) (out map[string]interface{}, err error) {
	if len(row) != len(table.Columns) {
		return nil, fmt.Errorf("column数量<%d>和len(row)=<%d>数量不相等", len(table.Columns), len(row))
	}

	out = make(map[string]interface{}, len(row))
	for i, v := range row {
		column := table.Columns[i]
		out[column.Name], err = a.parseValue(column.Type, column.RawType, v)
		if err != nil {
			return nil, err
		}
	}
	return
}

// 解析一个值
//
// 支持以下类型, 其它类型返回的值可能是各种奇怪类型
// 数字:
//      TINYINT: int8
//      TINYINT UNSIGNED: uint8
//      SMALLINT: int16
//      SMALLINT UNSIGNED: uint16
//      MEDIUMINT: int32
//      MEDIUMINT UNSIGNED: uint32
//      INT: int32
//      INT UNSIGNED: uint32
//      BIGINT: int64
//      BIGINT UNSIGNED: uint64
//      FLOAT: float32
//      DOUBLE: float64
//      DECIMAL: float64
// 字符串: string
//      CHAR, VARCHAR, TINYBLOB, TINYTEXT, BLOB, TEXT, MEDIUMBLOB, MEDIUMTEXT, LONGBLOB, LONGTEXT:
// 时间:
//      DATE: string
//      TIME: string
//      YEAR: int
//      DATETIME: string
//      TIMESTAMP: string
// 字节串: base64string
//      BINARY, VARBINARY
// 其它:
//      JSON: string
//      ENUM: int64
//      SET: int64
//      BIT: int64
//      POINT: []float64{x, y}
//      GEOMETRY: string geojson
func (a *analyzer) parseValue(t int, rawType string, v interface{}) (interface{}, error) {
	if v == nil {
		return nil, nil
	}
	switch t {
	case schema.TYPE_STRING, schema.TYPE_JSON:
		switch raw := v.(type) {
		case string:
			return v, nil
		case []uint8:
			if rawType == "geometry" {
				p, err := a.parseWKB(raw)
				if err != nil && a.IgnoreWKBDataParseError {
					a.app.Warn("parse geometry data error", zap.Error(err))
					return "", nil
				}
				return *zstr.BytesToString(p), err
			}
			return *zstr.BytesToString(raw), nil
		}
	case schema.TYPE_POINT:
		switch raw := v.(type) {
		case []uint8:
			p, err := a.parseWKBOfPoint(raw)
			if err != nil && a.IgnoreWKBDataParseError {
				a.app.Warn("parse point data error", zap.Error(err))
				return []float64{0, 0}, nil
			}
			return p, err
		}
	case schema.TYPE_BINARY:
		src, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("binary data error, %T can't convert to string", v)
		}
		buf := make([]byte, base64.StdEncoding.EncodedLen(len(src)))
		base64.StdEncoding.Encode(buf, zstr.StringToBytes(&src))
		return *zstr.BytesToString(buf), nil
	}
	return v, nil
}

func (a *analyzer) parseWKB(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte("null"), nil
	}

	if len(data) <= 4 {
		return nil, errors.New("wkb data is incomplete")
	}

	temp, err := wkb.Unmarshal(data[4:])
	if err != nil {
		return nil, fmt.Errorf("wkb data parser err: %s", err)
	}

	out, err := geojson.Marshal(temp)
	if err != nil {
		return nil, fmt.Errorf("wkb data cannot be converted to geojson: %s", err)
	}

	return out, nil
}

func (a *analyzer) parseWKBOfPoint(data []byte) ([]float64, error) {
	if len(data) == 0 {
		return []float64{0, 0}, nil
	}

	if len(data) != 25 {
		return nil, errors.New("point data is incomplete")
	}

	p, err := wkb.Unmarshal(data[4:])
	if err != nil {
		return nil, fmt.Errorf("point data parser err: %s", err)
	}
	if len(p.FlatCoords()) != 2 {
		return nil, fmt.Errorf("point data parser ok, but point size need 2, got %d", len(p.FlatCoords()))
	}

	return p.FlatCoords(), nil
}

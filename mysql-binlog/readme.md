<!-- TOC -->

- [mysql-binlog服务](#mysql-binlog服务)
- [说明](#说明)
- [示例](#示例)
- [配置](#配置)
- [`Record`说明](#record说明)
	- [Record 结构](#record-结构)
	- [字段映射](#字段映射)
		- [数字](#数字)
		- [字符串](#字符串)
		- [二进制](#二进制)
		- [时间](#时间)
		- [其它](#其它)
	- [Unmarshal](#unmarshal)
		- [将值定义为指定类型](#将值定义为指定类型)
		- [自定义解析](#自定义解析)
- [将`pos`保存到文件中](#将pos保存到文件中)

<!-- /TOC -->
---

# mysql-binlog服务

> 提供用于 https://github.com/zly-app/zapp 的服务

# 说明

```text
mysql_binlog.WithService()          # 启用服务
mysql_binlog.RegistryHandler(...)   # 服务注入(注册记录事件handler)
```

# 示例

```go
package main

import (
	mysql_binlog "github.com/zly-app/service/mysql-binlog"
	"github.com/zly-app/zapp"
)

func main() {
    // 启用服务
    app := zapp.NewApp("test", mysql_binlog.WithService())
    // 服务注入
    mysql_binlog.RegistryHandler(&mysql_binlog.BaseEventHandler{})
    // 运行
    app.Run()
}
```

# 配置

> 默认服务类型为 `mysql-binlog`

```toml
[services.mysql-binlog]
# mysql 主机地址
Host="localhost:3306"
# 用户名, 最好是root
UserName="root"
# 密码
Password="yourpwd"
# 字符集, 一般为utf8mb4
Charset="utf8mb4"
# 包含的表正则匹配, 匹配的数据为 dbName.tableName
IncludeTableRegex=["^dbname\\.tablename$"]
# 排除的表正则匹配, 匹配的数据为 dbName.tableName
ExcludeTableRegex=[]
# 放弃没有表元数据的row事件
DiscardNoMetaRowEvent=true
# mysqldump执行路径, 如果为空则忽略mysqldump只使用binlog, mysqldump执行路径一般为mysqldump
DumpExecutionPath=""
# 忽略wkb数据解析错误, 一般为POINT, GEOMETRY类型
IgnoreWKBDataParseError=true
```

# `Record`说明

## Record 结构

```go
type Record struct {
	Action    string                 `json:"action"`
	Old       map[string]interface{} `json:"old"`
	New       map[string]interface{} `json:"new"`
	DbName    string                 `json:"db_name"`
	TableName string                 `json:"table_name"`
	Timestamp uint32                 `json:"timestamp"`
}
```

1. `Action`表示动作, 只会是`update`,`insert`,`delete`这些值中的一个.

   ```
   当为 update 时,  Old 字段值为原始数据, New 的值为新数据.
   当为 insert 时,  Old 的值为 nil, New 字段值为新增的数据.
   当为 delete 时,  Old 字段值为原始数据, New 的值为 nil.
   ```

2. `DbName`表示数据库名.
3. `TableName`表示表名.
4. `Timestamp`表示数据发送改变时的时间戳, 单位为秒

## 字段映射

### 数字

| mysql字段          | go类型  |
| ------------------ | ------- |
| TINYINT            | int8    |
| TINYINT UNSIGNED   | uint8   |
| SMALLINT           | int16   |
| SMALLINT UNSIGNED  | uint16  |
| MEDIUMINT          | int32   |
| MEDIUMINT UNSIGNED | uint32  |
| INT                | int32   |
| INT UNSIGNED       | uint32  |
| BIGINT             | int64   |
| BIGINT UNSIGNED    | uint64  |
| FLOAT              | float32 |
| DOUBLE             | float64 |
| DECIMAL            | float64 |


### 字符串

| mysql字段                                                                                 | go类型 |
| ----------------------------------------------------------------------------------------- | ------ |
| CHAR, VARCHAR, TINYBLOB, BLOB, MEDIUMBLOB, LONGBLOB, TINYTEXT, TEXT, MEDIUMTEXT, LONGTEXT | string |


### 二进制

| mysql字段 | go类型               |
| --------- | -------------------- |
| BINARY    | base64编码后的string |
| VARBINARY | base64编码后的string |


### 时间

| mysql字段 | go类型 |
| --------- | ------ |
| DATE      | string |
| TIME      | string |
| YEAR      | int    |
| DATETIME  | string |
| TIMESTAMP | string |

### 其它


| mysql字段 | go类型              |
| --------- | ------------------- |
| JSON      | string              |
| ENUM      | int64               |
| SET       | int64               |
| BIT       | int64               |
| POINT     | []float64{x, y}     |
| GEOMETRY  | geojson格式的string |

## Unmarshal

`UnmarshalOld` 将 `Old` 字段内容解析到自定义结构中.
`UnmarshalNew` 将 `New` 字段内容解析到自定义结构中.

必须传入一个带指针的结构体, 字段对应优先对比 `scan` 标签的值, 无 `scan` 标签时对比 `json` 标签的值, 否则以结构体的字段名匹配. 如果标签值为空或 `-` 则忽略该字段.

### 将值定义为指定类型

这样做可以帮助程序识别你的原始数据类型, 做到精确的转换

```go
type Table struct {
	A string `scan:"a,string"` // 定义为string类型
	B struct{
		B1 string `json:"b1"`
		B2 string `json:"b2"`
	} `scan:"b,json"` // 定义为json类型
	C string `scan:"c,point"`  // 定义为point类型
	D string `scan:"d,binary"` // 定义为binary类型
}
```

1. `string` 要求原始数据的值必须是 `string` 或 `nil`, 解析器会调用 `zstr.Scan` 将数据解析到该字段中.
2. `json` 要求原始数据的值必须是 `string` 或 `nil`, 解析器会调用 `jsoniter.UnmarshalFromString` 方法将数据解析到该字段中.
3. `point` 要求原始数据的值必须是长度为 2 的 `[]float64` 或 `[]interface{}` 或 `nil`, 字段类型必须是切片, 解析器会对原始数据内的每一个值调用 `zstr.ScanAny` 解析后放入切片中.
4. `binary` 要求原始数据必须是 base64编码后的`string` 或 `nil`, 字段类型必须是 `string` 或 `[]byte`.

### 自定义解析

字段类型如果实现了以下接口, 则调用该接口的方法以实现自定义解析功能.

```go
type BinaryUnmarshaler interface {
	UnmarshalBinary(data []byte) error
}

type AnyUnmarshaler interface {
	UnmarshalAny(any interface{}) error
}
```

1. 对于 `string` 和 `json` 和 `binary` 定义. `AnyUnmarshaler` 接口中的 `UnmarshalAny` 方法中 `any` 的值一定是 `string`
2. 对于 `point` 定义. `AnyUnmarshaler` 接口中的 `UnmarshalAny` 方法中 `any` 的值一定是长度为 2 的 `[]float64`. 并且不支持 `BinaryUnmarshaler` 接口.

# 将`pos`保存到文件中

通过`PosFileHandler`自动将`pos`保存到文件

```go
package main

import (
	mysql_binlog "github.com/zly-app/service/mysql-binlog"
	"github.com/zly-app/zapp"
)

func main() {
    // 启用服务
    app := zapp.NewApp("test", mysql_binlog.WithService())
    // 服务注入
    mysql_binlog.RegistryHandler(&mysql_binlog.PosFileHandler{})
    // 运行
    app.Run()
}
```

1. handler启动时从pos文件中获取位置, 如果文件不存在则返回默认位置. 默认位置可通过`PosFileWithDefaultPos`修改.
2. pos变更时自动将pos追加到pos文件末尾, 如果是`force`则立即刷新到磁盘.
3. pos追加时会检查当前写入文件大小. 如果追加后会超过设置的`最大文件大小`则会创建一个新的pos文件写入pos并立即刷新到磁盘, 接着调用`Rename`命令替换之前的pos文件.

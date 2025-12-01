package mysql_binlog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/zly-app/zapp/log"
	"go.uber.org/zap"
)

const (
	defaultPosFileFileName   = "binlog.pos"
	defaultPosFileMaxSize    = 5 << 20
	defaultPosFileBinlogName = LatestPos
)

const posRotateFileNameSuffix = ".new" // pos新文件名后缀

type PosFileOption func(f *PosFileHandler)

// 设置pos文件名
func PosFileWithFilename(filename string) PosFileOption {
	return func(f *PosFileHandler) {
		f.filename = filename
	}
}

// 设置pos文件最大大小, <=0时表示不限制
func PosFileWithMaxSize(size int64) PosFileOption {
	return func(f *PosFileHandler) {
		f.maxFileSize = size
	}
}

// 设置默认binlog文件名和位置, 当pos文件不存在时生效
func PosFileWithDefaultPos(binlogName string, pos uint32) PosFileOption {
	return func(f *PosFileHandler) {
		f.binlogName, f.pos = binlogName, pos
	}
}

// 文件同步pos
type PosFileHandler struct {
	BaseEventHandler
	filename    string // 文件路径
	maxFileSize int64  // 最大文件大小, <=0时表示不限制
	binlogName  string // pos文件不存在时使用这个binlog文件
	pos         uint32 // pos文件不存在时binlog文件使用这个位置

	file *os.File // 当前pos文件
	size int64    // 当前写入大小
}

/*
pos文件, 将pos以追加方式写入到文件中

	filename pos文件路径
	defaultBinlogName pos文件不存在时使用这个binlog文件
	defaultPos pos文件不存在时binlog文件使用这个位置
*/
func NewPosFileHandler(opts ...PosFileOption) IEventHandler {
	f := &PosFileHandler{
		filename:    defaultPosFileFileName,
		maxFileSize: defaultPosFileMaxSize,
		binlogName:  defaultPosFileBinlogName,
		pos:         0,
	}

	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *PosFileHandler) GetStartPos() (binlogName string, pos uint32, err error) {
	// 检查旋转文件
	if err = f.checkPosRotateFileIsNotExists(); err != nil {
		return
	}

	// 获取pos
	binlogName, pos, err = f.getPosOfFile(f.filename)
	if os.IsNotExist(err) { // 文件不存在返回默认值
		return f.binlogName, f.pos, nil
	}
	if err != nil {
		return
	}
	if binlogName == "" {
		err = errors.New("pos文件是空的")
		return
	}
	return binlogName, pos, nil
}

// 检查旋转文件不存在
func (f *PosFileHandler) checkPosRotateFileIsNotExists() error {
	// 检查旋转文件是不存在的
	rotateFilename := f.filename + posRotateFileNameSuffix
	fi, err := os.Stat(rotateFilename)
	if err != nil {
		if os.IsNotExist(err) { // 文件不存在就是正常的
			return nil
		}
		return fmt.Errorf("检查pos的旋转文件错误: %v", err)
	}
	if fi != nil { // 找到了文件
		return errors.New("pos的旋转文件存在, 请检查")
	}
	return nil
}

// 从文件获取pos, 如果是空文件, binlogName为空字符串
func (f *PosFileHandler) getPosOfFile(filename string) (binlogName string, pos uint32, err error) {
	const readSize = 64 // 读取需要部分的大小

	// 打开pos文件
	file, err := os.OpenFile(filename, os.O_RDONLY, 0666)
	if err != nil {
		if os.IsNotExist(err) { // pos文件不存在则忽略
			return
		}
		err = fmt.Errorf("打开pos文件错误: %v", err)
		return
	}
	defer file.Close()

	// 获取文件大小
	fi, err := file.Stat()
	if err != nil {
		err = fmt.Errorf("检查pos文件错误: %v", err)
		return
	}
	if fi.IsDir() {
		err = errors.New("pos文件是一个目录")
		return
	}
	if fi.Size() == 0 { // 空文件
		return
	}

	// 移动偏移量
	if fi.Size() > readSize {
		_, err = file.Seek(-readSize, 2)
		if err != nil {
			err = fmt.Errorf("修改pos文件偏移量错误: %v", err)
			return
		}
	}

	// 只读取需要的部分
	bs, err := io.ReadAll(file)
	if err != nil {
		err = fmt.Errorf("读取pos失败: %v", err)
		return
	}
	// 去除尾部的\n
	if len(bs) > 0 && bs[len(bs)-1] == '\n' {
		bs = bs[:len(bs)-1]
	}
	if len(bs) == 0 { // 空文件
		return
	}
	// 查找数据
	k := bytes.LastIndexByte(bs, '\n') // 如果是-1表示找不到数据而从开头开始检查
	data := string(bs[k+1:])

	// 解析数据
	vv := strings.Split(data, ",")
	if len(vv) != 2 {
		err = errors.New("无法从pos文件中分析出pos位置, 格式不对")
		return
	}
	if len(vv[0]) == 0 { // name
		err = errors.New("无法从pos文件中分析出pos位置, 格式不对")
		return
	}
	p, err := strconv.Atoi(vv[1]) // pos
	if err != nil {
		err = errors.New("无法从pos文件中分析出pos位置, 格式不对")
		return
	}
	return vv[0], uint32(p), nil
}

func (f *PosFileHandler) OnPosSynced(binlogName string, pos uint32, force bool) error {
	if err := f.preparePosFile(); err != nil {
		log.Fatal("准备pos文件失败", zap.Error(err))
	}

	var buff bytes.Buffer
	buff.WriteString(binlogName)
	buff.WriteByte(',')
	buff.WriteString(strconv.Itoa(int(pos)))
	buff.WriteByte('\n')
	line := buff.Bytes()

	f.size += int64(len(line))
	if f.maxFileSize > 0 && f.size > f.maxFileSize { // 超出最大大小则旋转
		err := f.rotate(line)
		if err != nil {
			log.Fatal("旋转失败", zap.Error(err))
		}
		return nil
	}

	// 写入
	_, err := f.file.Write(line)
	if err != nil {
		return err
	}

	if force { // 如果是重要变更则刷新到磁盘
		return f.file.Sync()
	}
	return nil
}

// 准备好pos文件
func (f *PosFileHandler) preparePosFile() (err error) {
	if f.file != nil {
		return nil
	}

	f.file, err = os.OpenFile(f.filename, os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("打开pos文件失败: %v", err)
	}
	fi, err := f.file.Stat()
	if err != nil {
		return fmt.Errorf("获取pos文件信息失败: %v", err)
	}
	if fi.IsDir() {
		return fmt.Errorf("pos文件是一个目录")
	}
	f.size = fi.Size()
	return err
}

// 旋转并写入行
func (f *PosFileHandler) rotate(line []byte) error {
	// 原来的文件立即刷新并关闭
	err := f.file.Sync()
	if err != nil {
		return fmt.Errorf("刷新pos文件到磁盘失败: %v", err)
	}
	_ = f.file.Close()
	f.file = nil

	// 创建旋转文件
	rotateFilename := f.filename + posRotateFileNameSuffix
	file, err := os.Create(rotateFilename)
	if err != nil {
		return fmt.Errorf("创建pos旋转文件失败: %v", err)
	}
	// 写入行
	_, err = file.Write(line)
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("写入pos到新文件失败: %v", err)
	}
	// 刷新到磁盘
	err = file.Sync()
	if err != nil {
		return fmt.Errorf("刷新旋转文件到磁盘失败: %v", err)
	}
	_ = file.Close()

	// 覆盖
	err = os.Rename(rotateFilename, f.filename)
	if err != nil {
		return fmt.Errorf("替换pos文件失败: %v", err)
	}
	return nil
}

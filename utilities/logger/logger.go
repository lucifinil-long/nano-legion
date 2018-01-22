package logger

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DATEFORMAT is the date format
	DATEFORMAT = "2006-01-02"
	// HOURFORMAT is the date and hour format
	HOURFORMAT = "2006010215"
)

var logLevel = [4]string{"debug", "trace", "warn", "error"}

// Logger is logger struct
/*
 * 	默认日志文件级别包括debug/trace/warn/error
 */
type Logger struct {
	logMap     map[string]*LoggerInfo
	suffixInfo string
	logLevel   int // 需要记录的日志级别
	sync.RWMutex
}

// LoggerInfo is logger info struct
type LoggerInfo struct {
	filename       string
	bufferInfoLock sync.RWMutex
	buffer         *LoggerBuffer
	bufferQueue    chan LoggerBuffer
	fsyncInterval  time.Duration
	hour           time.Time
	fileOrder      int
	logFile        *os.File
	backupDir      string
}

const (
	_ = iota
	// KB is 1024 Bytes
	KB int64 = 1 << (iota * 10)
	// MB is 1024 KB
	MB
	// GB is 1024 MB
	GB
	// TB is 1024 GB
	TB
	maxFileSize       = 2 * GB
	maxFileCount      = 10
	defaultBufferSize = 2 * KB
)

// LoggerBuffer is logger buffer struct
type LoggerBuffer struct {
	bufferLock    sync.RWMutex
	bufferContent *bytes.Buffer
}

// NewLogger creates new logger object
/*
 * 创建一个新的日志记录对象
 * 创建新日志对象的同时，也会启动日志写入协程
 * @param filename: 日志文件名
 * @param suffix: 每条日志记录可能会追加的信息
 * @param backupDir: 日志备份目录
 * @return 成功则返回(*Logger, nil)；否则返回 (nil, error)
 */
func NewLogger(filename, suffix, backupDir string) (*Logger, error) {
	var err error
	var loggerInfo *LoggerInfo
	logMap := make(map[string]*LoggerInfo)
	for _, level := range logLevel {
		if loggerInfo, err = newLoggerInfo(filename, level); err != nil {
			return nil, err
		}

		loggerInfo.backupDir = backupDir
		go loggerInfo.WriteBufferToQueue()
		go loggerInfo.FlushBufferQueue()
		logMap[level] = loggerInfo
	}

	logger := &Logger{logMap: logMap, suffixInfo: suffix}
	return logger, nil
}

/*
 * 写日志，根据filename重新创建一个LoggerInfo，主要是针对自定义文件
 * @param filename：文件名
 * @param suffix：是否需要后缀信息
 * @param args：写入的内容
 */
func (logger *Logger) Write(filename string, suffix bool, args ...interface{}) {
	var loggerInfo *LoggerInfo
	var err error
	var Ok bool
	// 不存在需要重新初始化一下
	logger.Lock()
	defer logger.Unlock()
	if loggerInfo, Ok = logger.logMap[filename]; !Ok {
		if loggerInfo, err = newLoggerInfo(filename, ""); err != nil {
			println("[NewLoggerInfo] Write : " + err.Error())
			return
		}
		go loggerInfo.WriteBufferToQueue()
		go loggerInfo.FlushBufferQueue()
		logger.logMap[filename] = loggerInfo
	}
	loggerInfo.Write(Format(suffix, logger.suffixInfo, args...))
}

/*
 * 设置记录级别
 * @param l：记录级别，0最低，所有日志都记录，3表示只记录error日志
 */
func (logger *Logger) SetLevel(l int) {
	logger.Lock()
	defer logger.Unlock()
	if l > len(logLevel) {
		logger.logLevel = len(logLevel)
	} else {
		logger.logLevel = l
	}
}

/*
 * 检查记录级别
 * @param logType：需要检查的日志类别
 * @return 返回true表示当前需要记录该级别日志类型的日志；否则不需要
 */
func (logger *Logger) CheckLevel(logType string) bool {
	if logger.logLevel <= 0 {
		return true
	}
	logSet := logLevel[logger.logLevel:]
	for _, v := range logSet {
		if logType == v {
			return true
		}
	}
	return false
}

/*
 * 以下四个函数主要是写入不同的日志类型
 * @param args：写入的具体内容数组
 */
func (logger *Logger) Debug(args ...interface{}) {
	logger.RLock()
	loggerInfo := logger.logMap["debug"]
	d := logger.CheckLevel("debug")
	logger.RUnlock()
	if !d {
		return
	}

	pc, file, line, ok := runtime.Caller(1)
	if ok {
		funcName := ""
		if funcObj := runtime.FuncForPC(pc); funcObj != nil {
			funcName = funcObj.Name()
		}
		file = file[strings.Index(file, "src/"):]
		content := []interface{}{fmt.Sprintf("%v,%v:%v", file, line, funcName)}
		args = append(content, args...)
	}

	loggerInfo.Write(Format(true, logger.suffixInfo, args...))
}

func (logger *Logger) Trace(args ...interface{}) {
	logger.RLock()
	loggerInfo := logger.logMap["trace"]
	d := logger.CheckLevel("trace")
	logger.RUnlock()
	if !d {
		return
	}

	pc, file, line, ok := runtime.Caller(1)
	if ok {
		funcName := ""
		if funcObj := runtime.FuncForPC(pc); funcObj != nil {
			funcName = funcObj.Name()
		}
		file = file[strings.Index(file, "src/"):]
		content := []interface{}{fmt.Sprintf("%v,%v:%v", file, line, funcName)}
		args = append(content, args...)
	}
	loggerInfo.Write(Format(true, logger.suffixInfo, args...))
}

func (logger *Logger) Warn(args ...interface{}) {
	logger.RLock()
	loggerInfo := logger.logMap["warn"]
	d := logger.CheckLevel("warn")
	logger.RUnlock()
	if !d {
		return
	}
	loggerInfo.Write(Format(true, logger.suffixInfo, args...))
}

func (logger *Logger) Error(args ...interface{}) {
	logger.RLock()
	loggerInfo := logger.logMap["error"]
	d := logger.CheckLevel("error")
	logger.RUnlock()
	if !d {
		return
	}
	loggerInfo.Write(Format(true, logger.suffixInfo, args...))
}

/*
 * 构建一个LoggerInfo对象
 * @param filename：日志文件名信息
 * @param level：日志级别
 * @return 成功则返回(*LoggerInfo, nil)；否则返回(nil, error)
 */
func newLoggerInfo(filename, level string) (*LoggerInfo, error) {
	var err error
	loggerInfo := &LoggerInfo{
		bufferQueue:   make(chan LoggerBuffer, 50000),
		fsyncInterval: time.Second,
		buffer:        NewLoggerBuffer(),
		fileOrder:     0,
		backupDir:     "",
	}

	t, _ := time.Parse(HOURFORMAT, time.Now().Format(HOURFORMAT))
	loggerInfo.hour = t

	// 直接调用write写日志的文件名，用原始的文件名
	if len(level) == 0 {
		loggerInfo.filename = filename
	} else {
		loggerInfo.filename = filename + "-" + level + ".log"
	}

	err = loggerInfo.CreateFile()
	if err != nil {
		println("[NewLogger] openfile error : " + err.Error())
		return nil, err
	}
	return loggerInfo, nil
}

/*
 * 获取文件大小，如果文件不存在则重新创建文件
 * 则文件指针指向错误，重新open一下文件
 * 如果有其他的错误，此处无法处理，只能是丢掉部分日志内容
 */
func (logger *LoggerInfo) FileSize() (int64, error) {
	if f, err := os.Stat(logger.filename); err != nil {
		return 0, err
	} else {
		return f.Size(), nil
	}
}

/*
 * 创建文件
 */
func (this *LoggerInfo) CreateFile() error {
	var err error
	this.logFile, err = os.OpenFile(this.filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
	return err
}

/*
 * 判断文件是否需要切分
 */
func (logger *LoggerInfo) NeedSplit() (split bool, backup bool) {
	t, _ := time.Parse(HOURFORMAT, time.Now().Format(HOURFORMAT))
	if t.After(logger.hour) {
		return false, true
	} else {
		/*
		 * 判断文件大小错误，当做文件不存在，
		 * 重新创建一次文件，只重建一次，如果还有错误，
		 * 只做记录
		 */
		if size, err := logger.FileSize(); err != nil {
			if os.IsNotExist(err) {
				/* 文件不存在，重新创建文件 */
				println("[NeedSplit] FileSize: " + err.Error())
				if err = logger.CreateFile(); err != nil {
					println("[NeedSplit] CreateFile : " + err.Error())
				}
				return false, false
			} else {
				/* 如果不是文件不存在错误，不做处理*/
				println("[NeedSplit] FileSize: " + err.Error())
				return false, false
			}
		} else {
			if size > maxFileSize {
				return true, false
			}
		}
		return false, false
	}
	return false, false
}

func (logger *LoggerInfo) Write(content string) {
	logger.bufferInfoLock.Lock()
	logger.buffer.WriteString(content)
	logger.bufferInfoLock.Unlock()
}

/*
 * 将buffer中的数据写到队列中等待flush协程写入到硬盘
 */
func (logger *LoggerInfo) WriteBufferToQueue() {
	ticker := time.NewTicker(logger.fsyncInterval)
	defer ticker.Stop()
	for {
		<-ticker.C
		logger.bufferInfoLock.RLock()
		logger.buffer.WriteBuffer(logger.bufferQueue)
		logger.bufferInfoLock.RUnlock()
	}
}

/*
 * 将buffer中的数据flush到硬盘
 */
func (logger *LoggerInfo) FlushBufferQueue() {
	for {
		select {
		case buffer := <-logger.bufferQueue:
			/* 需要做文件切分 */
			isSplit, isBackup := logger.NeedSplit()
			if isSplit {
				logger.logFile.Close()
				newFilename := logger.filename + "." + logger.hour.Format(HOURFORMAT) + "." + strconv.Itoa(logger.fileOrder%maxFileCount)
				_, fileErr := os.Stat(newFilename)
				if fileErr == nil {
					os.Remove(newFilename)
				}
				err := os.Rename(logger.filename, newFilename)
				if err != nil {
					println("[FlushBufferQueue] Rename : " + err.Error())
				}
				if err = logger.CreateFile(); err != nil {
					println("[FlushBufferQueue] CreateFile : " + err.Error())
				}

				logger.fileOrder++
				if isBackup {
					logger.fileOrder = 0
					go logger.LoggerBackup(logger.hour)
					logger.hour, _ = time.Parse(HOURFORMAT, time.Now().Format(HOURFORMAT))
				}
			} else {
				if isBackup {
					logger.logFile.Close()

					var newFilename string
					if logger.fileOrder == 0 {
						newFilename = logger.filename + "." + logger.hour.Format(HOURFORMAT)
					} else {
						newFilename = logger.filename + "." + logger.hour.Format(HOURFORMAT) + "." + strconv.Itoa(logger.fileOrder%maxFileCount)
					}

					_, fileErr := os.Stat(newFilename)
					if fileErr == nil {
						os.Remove(newFilename)
					}
					err := os.Rename(logger.filename, newFilename)
					if err != nil {
						println("[FlushBufferQueue] Rename : " + err.Error())
					}
					if err = logger.CreateFile(); err != nil {
						println("[FlushBufferQueue] CreateFile : " + err.Error())
					}

					logger.fileOrder = 0
					go logger.LoggerBackup(logger.hour)
					logger.hour, _ = time.Parse(HOURFORMAT, time.Now().Format(HOURFORMAT))
				}
			}

			/* 写失败的话尝试再写一次 */
			if _, err := logger.logFile.Write(buffer.bufferContent.Bytes()); err != nil {
				println("[FlushBufferQueue] File.Write : " + err.Error())
				logger.logFile.Write(buffer.bufferContent.Bytes())
			}
			logger.logFile.Sync()

		}
	}
}

/*
 * 错误日志备份
 * backupDir 待备份的目录
 * os中没有mv的函数，只能先rename，后remove
 * backupDir -> /data/servers/log/saver/trace/2014-09-10/*.log
 */
func (logger *LoggerInfo) LoggerBackup(hour time.Time) {
	var oldFile string   //待备份文件
	var newFile string   //需要备份的新文件
	var backupDir string //备份的路径

	if logger.backupDir == "" {
		return
	}
	backupDir = filepath.Join(logger.backupDir, hour.Format(DATEFORMAT))
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		os.MkdirAll(backupDir, 0777)
	}

	/* backup filename like saver-error.log.2014-09-10*/
	oldFile = logger.filename + "." + hour.Format(HOURFORMAT)
	if stat, err := os.Stat(oldFile); err == nil {
		newFile = filepath.Join(backupDir, stat.Name())
		if err := os.Rename(oldFile, newFile); err != nil {
			println("[LoggerBackup] os.Rename:" + err.Error())
		}
	}

	/* backup filename like saver-error.log.2014-09-10.{0/1...} */
	for i := 0; i < maxFileCount; i++ {
		oldFile = logger.filename + "." + hour.Format(HOURFORMAT) + "." + strconv.Itoa(i)
		if stat, err := os.Stat(oldFile); err == nil {
			newFile = filepath.Join(backupDir, stat.Name())
			if err := os.Rename(oldFile, newFile); err != nil {
				println("[LoggerBackup] os.Rename:" + err.Error())
			}
		}
	}
}

func NewLoggerBuffer() *LoggerBuffer {
	return &LoggerBuffer{
		bufferContent: bytes.NewBuffer(make([]byte, 0, defaultBufferSize)),
	}
}

func (logger *LoggerBuffer) WriteString(str string) {
	logger.bufferContent.WriteString(str)
}

func (logger *LoggerBuffer) WriteBuffer(bufferQueue chan LoggerBuffer) {
	logger.bufferLock.Lock()
	if logger.bufferContent.Len() > 0 {
		bufferQueue <- *logger
		logger.bufferContent = bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
	}
	logger.bufferLock.Unlock()
}

func getDatetime() string {
	return time.Now().Format("2006-01-02 15:04:05.000")
}

func Format(suffix bool, suffixInfo string, args ...interface{}) string {
	var content string
	for _, arg := range args {
		switch arg.(type) {
		case int:
			content = content + "|" + strconv.Itoa(arg.(int))
			break
		case string:
			content = content + "|" + strings.TrimRight(arg.(string), "\n")
			break
		case int64:
			str := strconv.FormatInt(arg.(int64), 10)
			content = content + "|" + str
			break
		default:
			content = content + "|" + fmt.Sprintf("%v", arg)
			break
		}
	}
	if suffix {
		content = getDatetime() + content + "|" + suffixInfo + "\n"
	} else {
		content = getDatetime() + content + "\n"
	}
	return content
}

func GetInnerIp() string {
	info, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range info {
		ipMask := strings.Split(addr.String(), "/")
		if ipMask[0] != "127.0.0.1" && ipMask[0] != "24" {
			return ipMask[0]
		}
	}
	return ""
}

package Logger

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Logger interface {
	SetLogger(Level int, FilePath string, MaxDay int64)
	Errorf(format string, a ...interface{})
	Infof(format string, a ...interface{})
	GetConf()
	Close()
}

const (
	Debug = iota + 1
	Info
	Error
)

type Log struct {
	LogLevel    int         // 日志级别
	FilePath    string      // 文件存储路径
	MaxDay      int64       // 最大存储天数
	currentFile *os.File    // 当前文件
	currentDate string      // 文件创建时的日期
	mutex       sync.Mutex  // 互斥锁
	logChannels chan string // 异步写入
}

func NewLogger() Logger {
	Nlog := new(Log)
	return Nlog
}

func (l *Log) InitLogger() {
	l.LogLevel = Info
	l.MaxDay = 7
	l.FilePath = "."
	l.logChannels = make(chan string, 3000)
	// 清理日志文件
	go func() {
		err := l.clearOldLogs()
		if err != nil {
			log.Println("Failed to clean old logs:", err)
		}
	}()
}

func (l *Log) SetLogger(Level int, FilePath string, MaxDay int64) {
	l.InitLogger()
	if Level != 0 {
		switch Level {
		case Debug:
			l.LogLevel = Debug
		case Info:
			l.LogLevel = Info
		case Error:
			l.LogLevel = Error
		}
	}
	if FilePath != "" {
		// 确保日志文件目录存在
		err := os.MkdirAll(FilePath, 0777)
		if err != nil {
			log.Fatal(err)
		}
		l.FilePath = FilePath
	}
	l.FilePath = relativePathToAbsPath(l.FilePath)
	l.MaxDay = MaxDay
	FileName := formatLogFileName(time.Now())
	File, err := os.OpenFile(l.FilePath+"/"+FileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	l.currentFile = File
	l.currentDate = formatLogFileName(time.Now())
	go l.logWriteToFile()
}

func (l *Log) logWriteToFile() {
	for logline := range l.logChannels {
		if logline != "" {
			currentDate := time.Now().Format("2006-01-02")
			if currentDate != l.currentDate {
				l.createLogFile(time.Now())
			}
			_, _ = l.currentFile.WriteString(logline)

			// 检查并执行清理操作
			go func() {
				err := l.clearOldLogs()
				if err != nil {
					log.Println("Failed to clean old logs:", err)
				}
			}()
		}
	}
}

func (l *Log) syncWriteLog(format string, a ...interface{}) {
	message := l.logWithCallerInfo(fmt.Sprintf(format, a...))
	l.logChannels <- message
}

func (l *Log) createLogFile(date time.Time) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.currentFile != nil {
		_ = l.currentFile.Close()
	}
	// 创建新文件
	FileName := formatLogFileName(time.Now())
	File, err := os.OpenFile(l.FilePath+"/"+FileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
		return
	}
	l.currentFile = File
	l.currentDate = date.Format("2006-01-02")
}

func (l *Log) Errorf(format string, a ...interface{}) {
	l.LogLevel = Error
	l.syncWriteLog(format, a...)
}

func (l *Log) Infof(format string, a ...interface{}) {
	l.syncWriteLog(format, a...)
}

func (l *Log) GetLevelString() string {
	var Level string
	switch l.LogLevel {
	case Debug:
		Level = "Debug"
	case Info:
		Level = "Info"
	case Error:
		Level = "Error"
	}
	return Level
}

func (l *Log) GetConf() {
	var Level string
	switch l.LogLevel {
	case Debug:
		Level = "Debug"
	case Info:
		Level = "Info"
	case Error:
		Level = "Error"
	}
	fmt.Println(Level, l.FilePath, l.MaxDay)
}

func formatLogFileName(data time.Time) string {
	return data.Format("2006-01-02") + ".log"
}

// 获取对应文件名，行号，方法名
func (l *Log) logWithCallerInfo(logline string) string {
	pc, file, line, _ := runtime.Caller(3)
	funcName := runtime.FuncForPC(pc).Name()
	Level := l.GetLevelString()
	return fmt.Sprintf("[%s][%s] fileLine:%s:%d funcName:%s;message:%s\n", Level, time.Now().Format("2006-01-02 15:04:05"), file, line, getFunctionName(funcName), logline)
}

// 获取对应的方法名
func getFunctionName(fullName string) string {
	// 获取函数名的最后一个点号后面的部分
	lastDotIndex := 0
	for i := len(fullName) - 1; i >= 0; i-- {
		if fullName[i] == '.' {
			lastDotIndex = i
			break
		}
	}
	if fullName == "" {
		return ""
	}
	return fullName[lastDotIndex+1:]
}

// 清除过期日志
func (l *Log) clearOldLogs() error {
	// 需要清除的日期范围
	cutoffDate := time.Now().AddDate(0, 0, -int(l.MaxDay))

	err := filepath.Walk(l.FilePath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 检查文件是否为目录
		if info.IsDir() {
			return nil

		}
		// 检查文件日期是否早于需要清除的日期范围
		if info.ModTime().Before(cutoffDate) {
			// 删除文件
			if strings.HasSuffix(path, ".log") {
				if err = os.Remove(path); err != nil {
					return err
				}
			}
			log.Printf("Removed log file: %s\n", path)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to clear old logs:%v", err)
	}
	return nil
}

func relativePathToAbsPath(Path string) string {
	absolutePath, err := filepath.Abs(Path)
	if err != nil {
		fmt.Println("Failed to get absolute path:", err)
		return ""
	}
	fmt.Println(absolutePath)
	return absolutePath
}

// 关闭对应的写入通道和文件
func (l *Log) Close() {
	close(l.logChannels)
	if l.currentFile != nil {
		_ = l.currentFile.Close()
	}
}

package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 全局logger实例
var (
	InfoLogger  *logrus.Logger
	ErrorLogger *logrus.Logger
	GinLogger   *logrus.Logger
)

// Config 日志配置结构
type Config struct {
	Level            string
	EnableGinLogger  bool
	EnableFileOutput bool
	MaxFileSize      int // 单个日志文件最大大小(MB)
	MaxBackups       int // 保留的备份文件数量
	MaxAge           int // 保留文件的最大天数
	Compress         bool // 是否压缩旧文件
}

// Init 初始化日志系统
func Init() error {
	return InitWithConfig(Config{
		Level:            "info",
		EnableGinLogger:  true,
		EnableFileOutput: true,
		MaxFileSize:      100, // 100MB
		MaxBackups:       7,   // 保留7个备份文件
		MaxAge:           30,  // 保留30天
		Compress:         true, // 压缩旧文件
	})
}

// InitWithConfig 使用配置初始化日志系统
func InitWithConfig(config Config) error {
	// 创建日志目录
	logDir := "./configs/logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// 设置默认值
	if config.MaxBackups == 0 {
		config.MaxBackups = 7
	}
	if config.MaxAge == 0 {
		config.MaxAge = 30
	}
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 100
	}

	// 根据配置决定输出位置
	var infoOutputs []io.Writer
	var errorOutputs []io.Writer
	var ginOutputs []io.Writer

	// 总是输出到控制台
	infoOutputs = append(infoOutputs, os.Stdout)
	errorOutputs = append(errorOutputs, os.Stderr)
	ginOutputs = append(ginOutputs, os.Stdout)

	// 根据配置决定是否输出到文件
	if config.EnableFileOutput {
		// 创建info日志轮转器
		infoRotator := &lumberjack.Logger{
			Filename:   filepath.Join(logDir, "app.log"),
			MaxSize:    config.MaxFileSize, // MB
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge, // days
			Compress:   config.Compress,
		}
		infoOutputs = append(infoOutputs, infoRotator)

		// 创建error日志轮转器
		errorRotator := &lumberjack.Logger{
			Filename:   filepath.Join(logDir, "error.log"),
			MaxSize:    config.MaxFileSize, // MB
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge, // days
			Compress:   config.Compress,
		}
		errorOutputs = append(errorOutputs, errorRotator)

		// 只有启用Gin日志时才创建gin日志轮转器
		if config.EnableGinLogger {
			ginRotator := &lumberjack.Logger{
				Filename:   filepath.Join(logDir, "gin.log"),
				MaxSize:    config.MaxFileSize, // MB
				MaxBackups: config.MaxBackups,
				MaxAge:     config.MaxAge, // days
				Compress:   config.Compress,
			}
			ginOutputs = append(ginOutputs, ginRotator)
		}
	}

	// 解析日志级别
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel // 默认级别
	}

	// 设置info logger
	InfoLogger = logrus.New()
	InfoLogger.SetOutput(io.MultiWriter(infoOutputs...))
	InfoLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05 MST",
	})
	InfoLogger.SetLevel(level)

	// 设置error logger
	ErrorLogger = logrus.New()
	ErrorLogger.SetOutput(io.MultiWriter(errorOutputs...))
	ErrorLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05 MST",
	})
	ErrorLogger.SetLevel(logrus.WarnLevel)

	// 设置gin logger（只有启用时才设置）
	if config.EnableGinLogger {
		GinLogger = logrus.New()
		GinLogger.SetOutput(io.MultiWriter(ginOutputs...))
		GinLogger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05 MST",
		})
		GinLogger.SetLevel(level)
	} else {
		// 如果不启用Gin日志，创建一个空的logger
		GinLogger = logrus.New()
		GinLogger.SetOutput(io.Discard)
	}

	// 设置标准库log输出到app日志文件（普通信息）和控制台
	log.SetOutput(io.MultiWriter(infoOutputs...))
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// InfoLogger.Infof("Logger initialized, logs will be saved to %s", logDir)
	return nil
}

// GetGinLogWriter 获取GIN日志写入器
func GetGinLogWriter() io.Writer {
	logDir := "./configs/logs"
	
	// 使用lumberjack进行日志轮转
	return &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "gin.log"),
		MaxSize:    100, // MB
		MaxBackups: 7,   // 保留7个备份文件
		MaxAge:     30,  // 保留30天
		Compress:   true, // 压缩旧文件
	}
}

// 便捷函数，不同级别的日志
func Printf(format string, args ...interface{}) {
	InfoLogger.Infof(format, args...)
}

func Println(args ...interface{}) {
	InfoLogger.Info(args...)
}

// 错误级别的日志函数
func Errorf(format string, args ...interface{}) {
	ErrorLogger.Errorf(format, args...)
}

func Error(args ...interface{}) {
	ErrorLogger.Error(args...)
}

// 警告级别的日志函数
func Warnf(format string, args ...interface{}) {
	ErrorLogger.Warnf(format, args...)
}

func Warn(args ...interface{}) {
	ErrorLogger.Warn(args...)
}

func Fatalf(format string, v ...interface{}) {
	if ErrorLogger != nil {
		ErrorLogger.Fatalf(format, v...)
	}
}

func Fatal(v ...interface{}) {
	if ErrorLogger != nil {
		ErrorLogger.Fatal(v...)
	}
}

func Panicf(format string, v ...interface{}) {
	if ErrorLogger != nil {
		ErrorLogger.Panicf(format, v...)
	}
}

func Panic(v ...interface{}) {
	if ErrorLogger != nil {
		ErrorLogger.Panic(v...)
	}
}

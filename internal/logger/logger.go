package logger

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 全局logger实例
var (
	InfoLogger  = newLogger(os.Stdout, logrus.InfoLevel)
	ErrorLogger = newLogger(os.Stderr, logrus.WarnLevel)
	GinLogger   = newLogger(io.Discard, logrus.InfoLevel)
	fileOutputs []*lumberjack.Logger
)

// Config 日志配置结构
type Config struct {
	Level            string
	EnableGinLogger  bool
	EnableFileOutput bool
	MaxFileSize      int  // 单个日志文件最大大小(MB)
	MaxBackups       int  // 保留的备份文件数量
	MaxAge           int  // 保留文件的最大天数
	Compress         bool // 是否压缩旧文件
}

// Init 初始化日志系统
func Init() error {
	return InitWithConfig(Config{
		Level:            "info",
		EnableGinLogger:  true,
		EnableFileOutput: true,
		MaxFileSize:      100,  // 100MB
		MaxBackups:       7,    // 保留7个备份文件
		MaxAge:           30,   // 保留30天
		Compress:         true, // 压缩旧文件
	})
}

// InitWithConfig 使用配置初始化日志系统
func InitWithConfig(config Config) error {
	return initWithConfig(config, "./configs/logs", os.Stdout, os.Stderr)
}

func initWithConfig(config Config, logDir string, stdout, stderr io.Writer) error {
	if config.EnableFileOutput {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("create log directory: %w", err)
		}
	}
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 100
	}
	if config.MaxBackups == 0 {
		config.MaxBackups = 7
	}
	if config.MaxAge == 0 {
		config.MaxAge = 30
	}
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}

	var rotators []*lumberjack.Logger
	output := func(console io.Writer, filename string) io.Writer {
		if !config.EnableFileOutput {
			return console
		}
		rotator := &lumberjack.Logger{
			Filename:   filepath.Join(logDir, filename),
			MaxSize:    config.MaxFileSize,
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,
			Compress:   config.Compress,
		}
		rotators = append(rotators, rotator)
		return io.MultiWriter(console, rotator)
	}
	info := newLogger(output(stdout, "app.log"), level)
	errorLog := newLogger(output(stderr, "error.log"), logrus.WarnLevel)
	ginOutput := io.Writer(io.Discard)
	if config.EnableGinLogger {
		ginOutput = output(stdout, "gin.log")
	}
	ginLog := newLogger(ginOutput, level)

	// Initialization takes place before request handling starts.
	if err := Close(); err != nil {
		return err
	}
	fileOutputs = rotators
	InfoLogger, ErrorLogger, GinLogger = info, errorLog, ginLog
	log.SetOutput(info.Out)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	return nil
}

func newLogger(output io.Writer, level logrus.Level) *logrus.Logger {
	instance := logrus.New()
	instance.SetOutput(output)
	instance.SetLevel(level)
	instance.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05 MST",
	})
	return instance
}

// Close releases the rotating file writers at shutdown or reinitialization.
func Close() error {
	var failures []error
	for _, output := range fileOutputs {
		if err := output.Close(); err != nil {
			failures = append(failures, err)
		}
	}
	fileOutputs = nil
	return errors.Join(failures...)
}

// GetGinLogWriter reuses the configured output, including the disabled state.
func GetGinLogWriter() io.Writer { return GinLogger.Out }

// GinMiddleware delegates request timing and formatting to Gin.
func GinMiddleware() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Output:          GetGinLogWriter(),
		SkipQueryString: true,
		Skip: func(c *gin.Context) bool {
			level := logrus.InfoLevel
			if c.Writer.Status() >= 400 {
				level = logrus.ErrorLevel
			}
			return !GinLogger.IsLevelEnabled(level)
		},
	})
}

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

package main

import (
	apiv1 "FireFlow/internal/api/v1"
	"FireFlow/internal/core"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/service"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

//go:embed all:web
var webFS embed.FS

// 全局logger
var (
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
	GinLogger   *log.Logger
)

// 默认配置内容
const defaultConfigContent = `
server:
  port: ":9686"

database:
  path: "./configs/database.db"  # SQLite数据库文件
`

// initLogger 初始化日志系统
func initLogger() error {
	// 创建日志目录
	logDir := "./configs/logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// 生成日志文件名（按日期）
	currentTime := time.Now()
	dateStr := currentTime.Format("2006-01-02")

	infoLogFile := filepath.Join(logDir, fmt.Sprintf("app-%s.log", dateStr))
	errorLogFile := filepath.Join(logDir, fmt.Sprintf("error-%s.log", dateStr))
	ginLogFile := filepath.Join(logDir, fmt.Sprintf("gin-%s.log", dateStr))

	// 创建或打开info日志文件
	infoFile, err := os.OpenFile(infoLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open info log file: %v", err)
	}

	// 创建或打开error日志文件
	errorFile, err := os.OpenFile(errorLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open error log file: %v", err)
	}

	// 创建或打开gin日志文件
	ginFile, err := os.OpenFile(ginLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open gin log file: %v", err)
	}

	// 设置info logger：同时输出到控制台和文件
	InfoLogger = log.New(io.MultiWriter(os.Stdout, infoFile), "[INFO] ", log.LstdFlags|log.Lshortfile)

	// 设置error logger：同时输出到控制台和文件
	ErrorLogger = log.New(io.MultiWriter(os.Stderr, errorFile), "[ERROR] ", log.LstdFlags|log.Lshortfile)

	// 设置gin logger：同时输出到控制台和文件
	GinLogger = log.New(io.MultiWriter(os.Stdout, ginFile), "[GIN] ", log.LstdFlags)

	// 替换默认的logger
	log.SetOutput(io.MultiWriter(os.Stdout, infoFile))
	log.SetPrefix("[INFO] ")
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	InfoLogger.Printf("Logger initialized, logs will be saved to %s", logDir)
	return nil
}

// getGinLogWriter 获取GIN日志写入器
func getGinLogWriter() io.Writer {
	if GinLogger == nil {
		return os.Stdout
	}
	// 从GinLogger中提取文件写入器
	logDir := "./configs/logs"
	currentTime := time.Now()
	dateStr := currentTime.Format("2006-01-02")
	ginLogFile := filepath.Join(logDir, fmt.Sprintf("gin-%s.log", dateStr))

	ginFile, err := os.OpenFile(ginLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return os.Stdout
	}
	return ginFile
}

// ginLoggerMiddleware 自定义GIN日志中间件
func ginLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 记录日志
		end := time.Now()
		latency := end.Sub(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		// 根据状态码决定日志级别
		if statusCode >= 400 {
			if GinLogger != nil {
				GinLogger.Printf("[ERROR] %3d | %13v | %15s | %-7s %s",
					statusCode,
					latency,
					clientIP,
					method,
					path,
				)
			}
		} else {
			if GinLogger != nil {
				GinLogger.Printf("%3d | %13v | %15s | %-7s %s",
					statusCode,
					latency,
					clientIP,
					method,
					path,
				)
			}
		}
	}
}

// createDefaultConfig 创建默认配置文件
func createDefaultConfig(configPath string) error {
	// 确保配置文件目录存在
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// 写入默认配置
	return os.WriteFile(configPath, []byte(defaultConfigContent), 0644)
}

// initializeCronJobs 初始化定时任务
func initializeCronJobs(configService service.ConfigService, cronManager *core.CronManager, firewallService *service.FirewallService) error {
	// 获取定时任务相关配置
	cronEnabledStr, err := configService.GetConfig("cron_enabled")
	if err != nil || cronEnabledStr == "" {
		InfoLogger.Printf("Cron configuration not found, using default settings")
		// 设置默认配置
		configService.SetConfig("cron_enabled", "true", "string", "system", "定时任务启用状态")
		configService.SetConfig("ip_check_interval", "30", "string", "system", "IP检查间隔(分钟)")
		cronEnabledStr = "true"
	}

	intervalStr, err := configService.GetConfig("ip_check_interval")
	if err != nil || intervalStr == "" {
		intervalStr = "30" // 默认30分钟
	}

	var intervalMinutes int
	if _, err := fmt.Sscanf(intervalStr, "%d", &intervalMinutes); err != nil {
		intervalMinutes = 30 // 默认30分钟
	}

	cronEnabled := cronEnabledStr == "true"

	if cronEnabled && intervalMinutes > 0 {
		// 检查是否应该立即执行一次
		shouldRunNow, err := firewallService.CheckIfShouldRunNow(intervalMinutes)
		if err != nil {
			ErrorLogger.Printf("Warning: Failed to check if should run now: %v", err)
		} else if shouldRunNow {
			// 立即执行一次
			InfoLogger.Println("Running firewall update immediately due to elapsed time")
			err := cronManager.ExecuteNow()
			if err != nil {
				ErrorLogger.Printf("Warning: Failed to execute immediate update: %v", err)
			}
		}

		// 启动定时任务
		err = cronManager.StartFirewallUpdateJob(intervalMinutes)
		if err != nil {
			return fmt.Errorf("failed to start firewall update job: %v", err)
		}
		InfoLogger.Printf("Firewall update job started with %d minute interval", intervalMinutes)
	} else {
		InfoLogger.Println("Firewall update job is disabled or invalid interval")
	}

	return nil
}

func main() {
	// 初始化日志系统
	if err := initLogger(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// 加载 .env 文件（如果存在）
	if err := godotenv.Load(); err != nil {
		// InfoLogger.Printf("No .env file found, using system environment variables")
	} else {
		InfoLogger.Printf("Loaded environment variables from .env file")
	}

	// 从环境变量获取运行模式
	appMode := os.Getenv("APP_MODE")
	if appMode == "" {
		appMode = "production" // 默认为生产模式
	}

	// 设置 Gin 模式
	if os.Getenv("GIN_MODE") == "" {
		if appMode == "development" {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")

	// 尝试读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		// 如果是找不到配置文件的错误，创建默认配置
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			configPath := "./configs/config.yaml"
			InfoLogger.Printf("Config file not found, creating default config at %s", configPath)

			if err := createDefaultConfig(configPath); err != nil {
				ErrorLogger.Fatalf("Failed to create default config file: %v", err)
			}

			// 重新尝试读取配置
			if err := viper.ReadInConfig(); err != nil {
				ErrorLogger.Fatalf("Error reading newly created config file: %v", err)
			}

			InfoLogger.Printf("Default config file created successfully at %s", configPath)
		} else {
			// 其他读取错误
			ErrorLogger.Fatalf("Error reading config file: %v", err)
		}
	}

	// 确保数据库目录存在
	dbPath := viper.GetString("database.path")
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		ErrorLogger.Fatalf("Failed to create database directory: %v", err)
	}

	// 使用纯 Go SQLite 驱动配置，添加防锁配置
	dsn := dbPath + "?_timeout=30000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000&_foreign_keys=1&_busy_timeout=30000"
	db, err := gorm.Open(sqlite.New(sqlite.Config{
		DriverName: "sqlite",
		DSN:        dsn,
	}), &gorm.Config{
		// 添加数据库连接池配置
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		ErrorLogger.Fatalf("Failed to connect to database: %v", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		ErrorLogger.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	// 设置连接池参数以避免锁定
	sqlDB.SetMaxOpenConns(1)    // SQLite 只支持单个写连接
	sqlDB.SetMaxIdleConns(1)    // 保持一个空闲连接
	sqlDB.SetConnMaxLifetime(0) // 连接不过期

	// 执行WAL模式初始化
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		ErrorLogger.Printf("Warning: Failed to set WAL mode: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		ErrorLogger.Printf("Warning: Failed to set synchronous mode: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA busy_timeout=30000;"); err != nil {
		ErrorLogger.Printf("Warning: Failed to set busy timeout: %v", err)
	}
	// Auto-migrate the schema
	if err := db.AutoMigrate(
		&model.FirewallRule{},
		&model.ConfigItem{},
		&model.CloudProviderConfig{},
	); err != nil {
		ErrorLogger.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize repositories
	firewallRepo := repository.NewFirewallRepo(db)
	configRepo := repository.NewConfigRepository(db)

	// Initialize services
	configService := service.NewConfigService(configRepo)
	firewallService := service.NewFirewallService(firewallRepo, configService)

	// 初始化定时任务管理器
	cronManager := core.NewCronManager()
	cronManager.SetUpdateFunc(func() {
		firewallService.UpdateAllRules()
	})
	cronManager.Start() // 启动cron引擎

	// 检查并启动定时任务
	err = initializeCronJobs(configService, cronManager, firewallService)
	if err != nil {
		ErrorLogger.Printf("Warning: Failed to initialize cron jobs: %v", err)
	}

	InfoLogger.Printf("Firewall service initialized")

	// 创建Gin实例并配置自定义日志
	gin.DisableConsoleColor() // 禁用控制台颜色以便于文件日志
	gin.DefaultWriter = io.MultiWriter(os.Stdout, getGinLogWriter())
	gin.DefaultErrorWriter = io.MultiWriter(os.Stderr, getGinLogWriter())

	r := gin.Default()

	// 使用自定义的日志中间件
	r.Use(ginLoggerMiddleware())

	// 根据运行模式配置CORS和前端路由
	if appMode == "" {
		appMode = viper.GetString("server.mode")
		if appMode == "" {
			appMode = "production"
		}
	}

	if appMode == "development" {
		// 开发模式：添加CORS支持，允许前端跨域访问
		r.Use(func(c *gin.Context) {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")

			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}

			c.Next()
		})

		InfoLogger.Printf("Development mode: CORS enabled for frontend at %s", os.Getenv("FRONTEND_URL"))
	} else {
		// 生产模式：提供静态文件服务
		frontend, err := fs.Sub(webFS, "web")
		if err != nil {
			ErrorLogger.Fatal(err)
		}

		static, err := fs.Sub(frontend, "static")
		if err != nil {
			ErrorLogger.Fatal(err)
		}

		// 处理静态文件
		r.StaticFS("/static", http.FS(static))

		// 处理根路径
		r.GET("/", func(c *gin.Context) {
			data, err := frontend.Open("index.html")
			if err != nil {
				c.Status(404)
				return
			}
			defer data.Close()
			c.DataFromReader(200, -1, "text/html; charset=utf-8", data, nil)
		})

		// 对于所有其他路由，返回 index.html（SPA 路由支持）
		r.NoRoute(func(c *gin.Context) {
			// 只对非 API 路径和非静态文件路径返回 index.html
			if !strings.HasPrefix(c.Request.URL.Path, "/api/") &&
				!strings.HasPrefix(c.Request.URL.Path, "/static/") {
				data, err := frontend.Open("index.html")
				if err != nil {
					c.Status(404)
					return
				}
				defer data.Close()
				c.DataFromReader(200, -1, "text/html; charset=utf-8", data, nil)
			} else {
				c.Status(404)
			}
		})

	}

	// Register API v1 routes
	apiV1Group := r.Group("/api/v1")
	apiv1.RegisterRoutes(apiV1Group, firewallService, configService, cronManager)

	port := viper.GetString("server.port")
	InfoLogger.Printf("Server starting on port %s", port)
	if err := r.Run(port); err != nil {
		ErrorLogger.Fatalf("Failed to start server: %v", err)
	}
}

package main

import (
	apiv1 "FireFlow/internal/api/v1"
	"FireFlow/internal/core"
	"FireFlow/internal/logger"
	"FireFlow/internal/middleware"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/service"
	"crypto/rand"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

//go:embed all:web
var webFS embed.FS

// 默认配置内容
const defaultConfigContent = `
server:
  port: ":9686"
  timezone: "Asia/Shanghai"      # 服务器时区设置

database:
  path: "./configs/database.db"  # SQLite数据库文件

logging:
  level: "info"                  # 日志级别: debug, info, warn, error
  enable_gin_logger: true        # 是否启用Gin HTTP请求日志
  enable_file_output: true       # 是否输出日志到文件
  max_file_size: 100             # 日志文件最大大小(MB)
  max_backups: 7                 # 保留的备份文件数量
  max_age: 30                    # 保留文件的最大天数
  compress: true                 # 是否压缩旧文件

security:
  jwt_secret: ""                 # JWT密钥，留空将自动生成
  expire_time: 72                # JWT过期时间(小时)  
`

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
			if logger.GinLogger != nil {
				logger.GinLogger.Errorf("%3d | %13v | %15s | %-7s %s",
					statusCode,
					latency,
					clientIP,
					method,
					path,
				)
			}
		} else {
			if logger.GinLogger != nil {
				logger.GinLogger.Infof("%3d | %13v | %15s | %-7s %s",
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
		logger.InfoLogger.Info("Cron configuration not found, using default settings")
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
			logger.ErrorLogger.Warnf("Warning: Failed to check if should run now: %v", err)
		} else if shouldRunNow {
			// 立即执行一次
			logger.InfoLogger.Info("Running firewall update immediately due to elapsed time")
			err := cronManager.ExecuteNow()
			if err != nil {
				logger.ErrorLogger.Warnf("Warning: Failed to execute immediate update: %v", err)
			}
		}

		// 启动定时任务
		err = cronManager.StartFirewallUpdateJob(intervalMinutes)
		if err != nil {
			return fmt.Errorf("failed to start firewall update job: %v", err)
		}
		// logger.InfoLogger.Infof("Firewall update job started with %d minute interval", intervalMinutes)
	} else {
		logger.InfoLogger.Info("Firewall update job is disabled or invalid interval")
	}

	return nil
}

// generateJWTSecret 生成随机JWT密钥
func generateJWTSecret() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// 转换为十六进制字符串
	return fmt.Sprintf("%x", bytes), nil
}

// updateJWTSecretInConfigFile 手动更新配置文件中的JWT密钥，保持格式和注释
func updateJWTSecretInConfigFile(configFile, newSecret string) error {
	// 读取原始文件内容
	content, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}

	// 转换为字符串并替换JWT密钥
	configStr := string(content)
	lines := strings.Split(configStr, "\n")

	for i, line := range lines {
		// 查找 jwt_secret 行
		if strings.Contains(line, "jwt_secret:") && strings.Contains(line, "\"\"") {
			// 保持原有的缩进和格式，只替换空字符串为新密钥
			lines[i] = strings.Replace(line, `""`, `"`+newSecret+`"`, 1)
			break
		}
	}

	// 重新组合内容
	updatedContent := strings.Join(lines, "\n")

	// 写回文件
	return os.WriteFile(configFile, []byte(updatedContent), 0644)
}

// setupJWTSecret 设置JWT密钥
func setupJWTSecret() error {
	jwtSecret := viper.GetString("security.jwt_secret")

	if jwtSecret == "" {
		// 生成新的JWT密钥
		newSecret, err := generateJWTSecret()
		if err != nil {
			return fmt.Errorf("failed to generate JWT secret: %v", err)
		}

		// 设置到配置中
		viper.Set("security.jwt_secret", newSecret)

		// 保存到配置文件
		configFile := viper.ConfigFileUsed()
		if configFile != "" {
			if err := updateJWTSecretInConfigFile(configFile, newSecret); err != nil {
				logger.ErrorLogger.Warnf("Failed to write JWT secret to config file: %v", err)
			} else {
				logger.InfoLogger.Info("Generated and saved new JWT secret to config file")
			}
		}

		jwtSecret = newSecret
	}

	// 设置JWT密钥到中间件
	middleware.SetJWTSecret([]byte(jwtSecret))
	// 设置JWT过期时间
	expireTime := viper.GetInt("security.expire_time")
	if expireTime <= 0 {
		expireTime = 72 // 默认72小时
	}
	middleware.SetTokenExpiration(expireTime)

	return nil
}

// setupTimezone 设置时区
func setupTimezone() error {
	timezone := viper.GetString("server.timezone")
	if timezone == "" {
		timezone = "Asia/Shanghai" // 默认时区
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		logger.InfoLogger.Warnf("Failed to load timezone %s, using Asia/Shanghai: %v", timezone, err)
		location, err = time.LoadLocation("Asia/Shanghai")
		if err != nil {
			logger.InfoLogger.Warnf("Failed to load Asia/Shanghai timezone, using UTC: %v", err)
			location = time.UTC
		}
	}

	time.Local = location
	return nil
}

// printHelp 打印帮助信息
func printHelp() {
	fmt.Println("FireFlow - 防火墙管理系统")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  ./fireflow          启动服务器")
	fmt.Println("  ./fireflow reset    重置管理员密码为 'password'")
	fmt.Println("  ./fireflow help     显示此帮助信息")
	fmt.Println()
}

// handleResetCommand 处理重置命令
func handleResetCommand() {
	fmt.Println("正在重置管理员账户...")

	// 初始化基本配置
	if err := logger.Init(); err != nil {
		fmt.Printf("日志初始化失败: %v\n", err)
		return
	}

	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		// 忽略错误，使用系统环境变量
	}

	// 读取配置文件
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("读取配置文件失败: %v\n", err)
		return
	}

	// 设置时区
	if err := setupTimezone(); err != nil {
		fmt.Printf("设置时区失败: %v\n", err)
		return
	}

	// 连接数据库
	dbPath := viper.GetString("database.path")
	if dbPath == "" {
		dbPath = "./configs/database.db"
	}

	// 确保数据库目录存在
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		fmt.Printf("创建数据库目录失败: %v\n", err)
		return
	}

	dsn := dbPath + "?_timeout=30000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000&_foreign_keys=1&_busy_timeout=30000"
	db, err := gorm.Open(sqlite.New(sqlite.Config{
		DriverName: "sqlite",
		DSN:        dsn,
	}), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		fmt.Printf("连接数据库失败: %v\n", err)
		return
	}

	// 确保数据表存在
	if err := db.AutoMigrate(&model.AuthUser{}); err != nil {
		fmt.Printf("数据库迁移失败: %v\n", err)
		return
	}

	// 创建仓库和服务
	authRepo := repository.NewAuthUserRepository(db)
	authService := service.NewAuthService(authRepo)

	// 重置管理员密码
	if err := authService.ResetAdminPassword(); err != nil {
		fmt.Printf("重置管理员密码失败: %v\n", err)
		return
	}

	fmt.Println("✅ 管理员账户重置成功！")
	fmt.Println("   用户名: admin")
	fmt.Println("   密码: password")
}

func main() {
	// 检查命令行参数
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "reset":
			handleResetCommand()
			return
		case "help", "-h", "--help":
			printHelp()
			return
		default:
			fmt.Printf("Unknown command: %s\n", os.Args[1])
			printHelp()
			return
		}
	}

	// 初始化日志系统
	if err := logger.Init(); err != nil {
		logrus.Fatalf("Failed to initialize logger: %v", err)
	}
	if err := godotenv.Load(); err != nil {
		// logger.InfoLogger.Info("No .env file found, using system environment variables")
	} else {
		logger.InfoLogger.Info("Loaded environment variables from .env file")
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
			logger.InfoLogger.Infof("Config file not found, creating default config at %s", configPath)

			if err := createDefaultConfig(configPath); err != nil {
				logger.ErrorLogger.Fatalf("Failed to create default config file: %v", err)
			}

			// 重新尝试读取配置
			if err := viper.ReadInConfig(); err != nil {
				logger.ErrorLogger.Fatalf("Error reading newly created config file: %v", err)
			}

			logger.InfoLogger.Infof("Default config file created successfully at %s", configPath)
		} else {
			// 其他读取错误
			logger.ErrorLogger.Fatalf("Error reading config file: %v", err)
		}
	}

	// 重新初始化日志系统（基于配置文件）
	logConfig := logger.Config{
		Level:            viper.GetString("logging.level"),
		EnableGinLogger:  viper.GetBool("logging.enable_gin_logger"),
		EnableFileOutput: viper.GetBool("logging.enable_file_output"),
		MaxFileSize:      viper.GetInt("logging.max_file_size"),
		MaxBackups:       viper.GetInt("logging.max_backups"),
		MaxAge:           viper.GetInt("logging.max_age"),
		Compress:         viper.GetBool("logging.compress"),
	}
	if err := logger.InitWithConfig(logConfig); err != nil {
		logger.ErrorLogger.Fatalf("Failed to reinitialize logger with config: %v", err)
	}

	// 设置时区
	if err := setupTimezone(); err != nil {
		logger.ErrorLogger.Fatalf("Failed to setup timezone: %v", err)
	}

	// 设置JWT密钥
	if err := setupJWTSecret(); err != nil {
		logger.ErrorLogger.Fatalf("Failed to setup JWT secret: %v", err)
	}

	// 确保数据库目录存在
	dbPath := viper.GetString("database.path")
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		logger.ErrorLogger.Fatalf("Failed to create database directory: %v", err)
	}

	// 使用纯 Go SQLite 驱动配置，添加防锁配置
	dsn := dbPath + "?_timeout=30000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000&_foreign_keys=1&_busy_timeout=30000"
	db, err := gorm.Open(sqlite.New(sqlite.Config{
		DriverName: "sqlite",
		DSN:        dsn,
	}), &gorm.Config{
		// 添加数据库连接池配置
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   gormlogger.Default.LogMode(gormlogger.Silent), // 设置GORM日志为静默模式
	})
	if err != nil {
		logger.ErrorLogger.Fatalf("Failed to connect to database: %v", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		logger.ErrorLogger.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	// 设置连接池参数以避免锁定
	sqlDB.SetMaxOpenConns(1)    // SQLite 只支持单个写连接
	sqlDB.SetMaxIdleConns(1)    // 保持一个空闲连接
	sqlDB.SetConnMaxLifetime(0) // 连接不过期

	// 执行WAL模式初始化
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		logger.ErrorLogger.Warnf("Warning: Failed to set WAL mode: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		logger.ErrorLogger.Warnf("Warning: Failed to set synchronous mode: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA busy_timeout=30000;"); err != nil {
		logger.ErrorLogger.Warnf("Warning: Failed to set busy timeout: %v", err)
	}
	// Auto-migrate the schema
	if err := db.AutoMigrate(
		&model.FirewallRule{},
		&model.ConfigItem{},
		&model.CloudProviderConfig{},
		&model.AuthUser{},
	); err != nil {
		logger.ErrorLogger.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize repositories
	firewallRepo := repository.NewFirewallRepo(db)
	configRepo := repository.NewConfigRepository(db)
	authRepo := repository.NewAuthUserRepository(db)

	// Initialize services
	configService := service.NewConfigService(configRepo)
	firewallService := service.NewFirewallService(firewallRepo, configService)
	authService := service.NewAuthService(authRepo)

	// 设置JWT令牌版本验证器
	middleware.SetTokenValidator(authService)

	// Initialize default admin user
	if err := authService.InitializeDefaultUser(); err != nil {
		logger.ErrorLogger.Fatalf("Failed to initialize default user: %v", err)
	}

	// 初始化定时任务管理器
	cronManager := core.NewCronManager()
	cronManager.SetUpdateFunc(func() {
		firewallService.UpdateAllRules()
	})
	cronManager.Start() // 启动cron引擎

	// 检查并启动定时任务
	err = initializeCronJobs(configService, cronManager, firewallService)
	if err != nil {
		logger.ErrorLogger.Warnf("Warning: Failed to initialize cron jobs: %v", err)
	}

	// logger.InfoLogger.Info("Firewall service initialized")

	// 创建Gin实例并配置自定义日志
	gin.DisableConsoleColor() // 禁用控制台颜色以便于文件日志

	r := gin.New()

	// 使用默认的恢复中间件
	r.Use(gin.Recovery())

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

		logger.InfoLogger.Infof("Development mode: CORS enabled for frontend at %s", os.Getenv("FRONTEND_URL"))
	} else {
		// 生产模式：提供静态文件服务
		frontend, err := fs.Sub(webFS, "web")
		if err != nil {
			logger.ErrorLogger.Fatal(err)
		}

		static, err := fs.Sub(frontend, "static")
		if err != nil {
			logger.ErrorLogger.Fatal(err)
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
	apiv1.RegisterRoutes(apiV1Group, firewallService, configService, cronManager, authService)

	port := viper.GetString("server.port")
	logger.InfoLogger.Infof("Server starting on port %s", port)
	if err := r.Run(port); err != nil {
		logger.ErrorLogger.Fatalf("Failed to start server: %v", err)
	}
}

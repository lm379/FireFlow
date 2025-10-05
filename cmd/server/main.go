package main

import (
	apiv1 "FireFlow/internal/api/v1"
	"FireFlow/internal/core"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/service"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

//go:embed all:web
var webFS embed.FS

// 默认配置内容
const defaultConfigContent = `
server:
  port: ":9686"

database:
  path: "./configs/database.db"  # SQLite数据库文件
`

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

func main() {
	// 加载 .env 文件（如果存在）
	if err := godotenv.Load(); err != nil {
		// log.Printf("No .env file found, using system environment variables")
	} else {
		log.Printf("Loaded environment variables from .env file")
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
			log.Printf("Config file not found, creating default config at %s", configPath)

			if err := createDefaultConfig(configPath); err != nil {
				log.Fatalf("Failed to create default config file: %v", err)
			}

			// 重新尝试读取配置
			if err := viper.ReadInConfig(); err != nil {
				log.Fatalf("Error reading newly created config file: %v", err)
			}

			log.Printf("Default config file created successfully at %s", configPath)
		} else {
			// 其他读取错误
			log.Fatalf("Error reading config file: %v", err)
		}
	}

	// 确保数据库目录存在
	dbPath := viper.GetString("database.path")
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
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
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	// 设置连接池参数以避免锁定
	sqlDB.SetMaxOpenConns(1)    // SQLite 只支持单个写连接
	sqlDB.SetMaxIdleConns(1)    // 保持一个空闲连接
	sqlDB.SetConnMaxLifetime(0) // 连接不过期

	// 执行WAL模式初始化
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("Warning: Failed to set WAL mode: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		log.Printf("Warning: Failed to set synchronous mode: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA busy_timeout=30000;"); err != nil {
		log.Printf("Warning: Failed to set busy timeout: %v", err)
	}
	// Auto-migrate the schema
	if err := db.AutoMigrate(
		&model.FirewallRule{},
		&model.ConfigItem{},
		&model.CloudProviderConfig{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
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

	// 注释掉单规则定时任务，避免与全局任务冲突
	// if err := firewallService.StartRuleUpdateJobs(cronManager); err != nil {
	// 	log.Printf("Failed to start rule update jobs: %v", err)
	// }

	log.Printf("Global firewall update job started")

	r := gin.Default()

	// 根据运行模式配置CORS和前端路由
	if appMode == "" {
		appMode = viper.GetString("server.mode")
		if appMode == "" {
			appMode = "production"
		}
	}

	log.Printf("Running in %s mode", appMode)

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

		log.Printf("Development mode: CORS enabled for frontend at %s", os.Getenv("FRONTEND_URL"))
	} else {
		// 生产模式：提供静态文件服务
		frontend, err := fs.Sub(webFS, "web")
		if err != nil {
			log.Fatal(err)
		}

		static, err := fs.Sub(frontend, "static")
		if err != nil {
			log.Fatal(err)
		}

		// 处理静态文件
		r.StaticFS("/static", http.FS(static))

		// 处理根目录下的静态文件（如 vite.svg）
		r.GET("/vite.svg", func(c *gin.Context) {
			data, err := frontend.Open("vite.svg")
			if err != nil {
				c.Status(404)
				return
			}
			defer data.Close()
			c.DataFromReader(200, -1, "image/svg+xml", data, nil)
		})

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

		log.Printf("Production mode: Serving embedded frontend files")
	}

	// Register API v1 routes
	apiV1Group := r.Group("/api/v1")
	apiv1.RegisterRoutes(apiV1Group, firewallService, configService, cronManager)

	port := viper.GetString("server.port")
	log.Printf("Server starting on port %s", port)
	if err := r.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

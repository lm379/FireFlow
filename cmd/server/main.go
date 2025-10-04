package main

import (
	apiv1 "FireFlow/internal/api/v1"
	"FireFlow/internal/core"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/service"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

//go:embed web/templates/*
var templateFS embed.FS

//go:embed web/static
var staticFS embed.FS

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

// setupWebAssets 设置嵌入的模板和静态文件
func setupWebAssets(r *gin.Engine) {
	// 加载嵌入的模板
	tmpl, err := template.ParseFS(templateFS, "web/templates/*")
	if err != nil {
		log.Fatalf("Failed to parse embedded templates: %v", err)
	}
	r.SetHTMLTemplate(tmpl)

	// 设置嵌入的静态文件
	staticSubFS, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		log.Fatalf("Failed to create static sub filesystem: %v", err)
	}
	r.StaticFS("/static", http.FS(staticSubFS))
}

func main() {
	// 加载 .env 文件（如果存在）
	if err := godotenv.Load(); err != nil {
		// log.Printf("No .env file found, using system environment variables")
	} else {
		log.Printf("Loaded environment variables from .env file")
	}

	// 设置 Gin 模式
	if os.Getenv("GIN_MODE") == "" {
		// 如果没有设置环境变量，默认使用 release 模式
		gin.SetMode(gin.ReleaseMode)
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

	// 使用纯 Go SQLite 驱动配置
	db, err := gorm.Open(sqlite.New(sqlite.Config{
		DriverName: "sqlite",
		DSN:        dbPath,
	}), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	// Auto-migrate the schema
	if err := db.AutoMigrate(
		&model.FirewallRule{},
		&model.ConfigItem{},
		&model.CloudProviderConfig{},
		&model.CronJobConfig{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize repositories
	firewallRepo := repository.NewFirewallRepo(db)
	configRepo := repository.NewConfigRepository(db)

	// Initialize services
	configService := service.NewConfigService(configRepo)
	firewallService := service.NewFirewallService(firewallRepo, configService)

	// 初始化定时任务管理器，但不自动启动任务
	cronManager := core.NewCronManager()
	cronManager.SetUpdateFunc(func() {
		firewallService.UpdateAllRules()
	})
	cronManager.Start() // 只启动cron引擎，不添加具体任务

	r := gin.Default()

	// Setup web assets (templates and static files)
	setupWebAssets(r)

	// Base URL for the frontend
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "动态防火墙规则管理",
		})
	})

	// Register API v1 routes
	apiV1Group := r.Group("/api/v1")
	apiv1.RegisterRoutes(apiV1Group, firewallService, configService, cronManager)

	port := viper.GetString("server.port")
	log.Printf("Server starting on port %s", port)
	if err := r.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

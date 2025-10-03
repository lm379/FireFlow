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

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

//go:embed web/templates/*
var templateFS embed.FS

//go:embed web/static
var staticFS embed.FS

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
	// 设置 Gin 模式
	if os.Getenv("GIN_MODE") == "" {
		// 如果没有设置环境变量，根据是否为生产环境自动设置
		if os.Getenv("ENV") == "production" {
			gin.SetMode(gin.ReleaseMode)
		} else {
			gin.SetMode(gin.DebugMode)
		}
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}

	db, err := gorm.Open(sqlite.Open(viper.GetString("database.path")), &gorm.Config{})
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

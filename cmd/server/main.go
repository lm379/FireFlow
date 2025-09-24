package main

import (
	apiv1 "FireFlow/internal/api/v1"
	"FireFlow/internal/core"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/service"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	// 1. Load Configuration
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}

	// 2. Initialize Database
	db, err := gorm.Open(sqlite.Open(viper.GetString("database.path")), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	// Auto-migrate the schema
	if err := db.AutoMigrate(&model.FirewallRule{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// 3. Setup Dependencies
	firewallRepo := repository.NewFirewallRepo(db)
	firewallService := service.NewFirewallService(firewallRepo)

	// 4. Start Cron Job
	cronManager := core.NewCronManager()
	err = cronManager.AddFirewallUpdateJob(func() {
		firewallService.UpdateAllRules()
	})
	if err != nil {
		log.Fatalf("Failed to add firewall update job: %v", err)
	}
	cronManager.Start()

	// 5. Setup Gin Router
	r := gin.Default()

	// Load HTML templates
	r.LoadHTMLGlob("web/templates/*")
	// Serve static files
	r.Static("/static", "./web/static")

	// Base URL for the frontend
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "动态防火墙规则管理",
		})
	})

	// Register API v1 routes
	apiV1Group := r.Group("/api/v1")
	apiv1.RegisterRoutes(apiV1Group, firewallService)

	// 6. Start Server
	port := viper.GetString("server.port")
	log.Printf("Server starting on port %s", port)
	if err := r.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

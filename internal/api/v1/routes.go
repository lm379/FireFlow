package v1

import (
	"FireFlow/internal/core"
	"FireFlow/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all v1 API routes.
func RegisterRoutes(router *gin.RouterGroup, firewallService *service.FirewallService, configService service.ConfigService, cronManager *core.CronManager) {
	firewallHandler := NewFirewallHandler(firewallService)
	firewallHandler.SetConfigService(configService) // 设置配置服务
	firewallHandler.SetCronManager(cronManager)     // 设置定时任务管理器
	configHandler := NewConfigHandler(configService, cronManager)
	configHandler.SetFirewallService(firewallService) // 设置防火墙服务
	cloudConfigHandler := NewCloudConfigHandler(configService)

	// 防火墙规则路由
	ruleRoutes := router.Group("/rules")
	{
		ruleRoutes.GET("/", firewallHandler.GetRules)
		ruleRoutes.GET("/:id", firewallHandler.GetRule)
		ruleRoutes.POST("/", firewallHandler.CreateRule)
		ruleRoutes.PUT("/:id", firewallHandler.UpdateRule)
		ruleRoutes.DELETE("/:id", firewallHandler.DeleteRule)
		ruleRoutes.POST("/:id/execute", firewallHandler.ExecuteRule)
		ruleRoutes.POST("/sync", firewallHandler.SyncRules)
	}

	// 云服务配置路由
	cloudConfigRoutes := router.Group("/cloud-configs")
	{
		cloudConfigRoutes.GET("/", cloudConfigHandler.GetCloudConfigs)
		cloudConfigRoutes.GET("/:id", cloudConfigHandler.GetCloudConfig)
		cloudConfigRoutes.POST("/", cloudConfigHandler.CreateCloudConfig)
		cloudConfigRoutes.PUT("/:id", cloudConfigHandler.UpdateCloudConfig)
		cloudConfigRoutes.DELETE("/:id", cloudConfigHandler.DeleteCloudConfig)
		cloudConfigRoutes.POST("/:id/test", cloudConfigHandler.TestCloudConfig)
	}

	// 配置路由
	configRoutes := router.Group("/config")
	{
		configRoutes.GET("/:key", configHandler.GetConfig)
		configRoutes.POST("/", configHandler.SetConfig)
		configRoutes.GET("/category/:category", configHandler.GetConfigsByCategory)
	}

	// 系统配置路由
	systemRoutes := router.Group("/system-config")
	{
		systemRoutes.GET("/", configHandler.GetSystemConfig)
		systemRoutes.PUT("/", configHandler.SetSystemConfig)
	}

	// IP同步路由
	router.POST("/sync-ip/", configHandler.SyncIPNow)
	router.GET("/current-ip/", configHandler.GetCurrentIP)
}

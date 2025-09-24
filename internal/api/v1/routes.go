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
	configHandler := NewConfigHandler(configService, cronManager)
	configHandler.SetFirewallService(firewallService) // 设置防火墙服务
	cloudConfigHandler := NewCloudConfigHandler(configService)
	cronJobHandler := NewCronJobHandler(configService)

	// 防火墙规则路由
	ruleRoutes := router.Group("/rules")
	{
		ruleRoutes.GET("/", firewallHandler.GetRules)
		ruleRoutes.POST("/", firewallHandler.CreateRule)
		ruleRoutes.PUT("/:id", firewallHandler.UpdateRule)
		ruleRoutes.DELETE("/:id", firewallHandler.DeleteRule)
		ruleRoutes.POST("/:id/execute", firewallHandler.ExecuteRule)
	}

	// 云服务配置路由
	cloudConfigRoutes := router.Group("/cloud-configs")
	{
		cloudConfigRoutes.GET("/", cloudConfigHandler.GetCloudConfigs)
		cloudConfigRoutes.POST("/", cloudConfigHandler.CreateCloudConfig)
		cloudConfigRoutes.PUT("/:id", cloudConfigHandler.UpdateCloudConfig)
		cloudConfigRoutes.DELETE("/:id", cloudConfigHandler.DeleteCloudConfig)
		cloudConfigRoutes.POST("/:id/test", cloudConfigHandler.TestCloudConfig)
	}

	// 定时任务路由
	cronJobRoutes := router.Group("/cron-jobs")
	{
		cronJobRoutes.GET("/", cronJobHandler.GetCronJobs)
		cronJobRoutes.POST("/", cronJobHandler.CreateCronJob)
		cronJobRoutes.PUT("/:id", cronJobHandler.UpdateCronJob)
		cronJobRoutes.DELETE("/:id", cronJobHandler.DeleteCronJob)
		cronJobRoutes.POST("/:id/run", cronJobHandler.RunCronJob)
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

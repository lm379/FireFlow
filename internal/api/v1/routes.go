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
	regionHandler := NewRegionHandler() // 创建地域处理器
	serviceTypeHandler := NewServiceTypeHandler() // 创建服务类型处理器

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

	// 地域相关路由
	regionRoutes := router.Group("/regions")
	{
		regionRoutes.GET("", regionHandler.GetRegions)        // 获取地域列表
		regionRoutes.GET("/search", regionHandler.SearchRegions) // 搜索地域
		regionRoutes.GET("/detail", regionHandler.GetRegionByCode) // 获取地域详情
	}
	
	// 云厂商路由
	router.GET("/providers", regionHandler.GetProviders) // 获取云厂商列表
	
	// 服务类型路由
	router.GET("/service-types", serviceTypeHandler.GetServiceTypesByProvider) // 根据提供商获取服务类型
}

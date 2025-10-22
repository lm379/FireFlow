package v1

import (
	"FireFlow/internal/core"
	"FireFlow/internal/middleware"
	"FireFlow/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all v1 API routes.
func RegisterRoutes(router *gin.RouterGroup, firewallService *service.FirewallService, configService service.ConfigService, cronManager *core.CronManager, authService service.AuthService) {
	firewallHandler := NewFirewallHandler(firewallService)
	firewallHandler.SetConfigService(configService) // 设置配置服务
	firewallHandler.SetCronManager(cronManager)     // 设置定时任务管理器
	configHandler := NewConfigHandler(configService, cronManager)
	configHandler.SetFirewallService(firewallService) // 设置防火墙服务
	cloudConfigHandler := NewCloudConfigHandler(configService)
	regionHandler := NewRegionHandler()           // 创建地域处理器
	serviceTypeHandler := NewServiceTypeHandler() // 创建服务类型处理器
	authHandler := NewAuthHandler(authService)    // 创建认证处理器

	// 认证相关路由
	authRoutes := router.Group("/auth")
	{
		authRoutes.POST("/login", authHandler.Login)       // POST /api/v1/auth/login
		authRoutes.GET("/verify", authHandler.VerifyToken) // GET /api/v1/auth/verify
	}

	// 需要认证的路由组
	protectedRoutes := router.Group("/")
	protectedRoutes.Use(middleware.JWTAuthMiddleware())
	{
		// 认证用户相关路由
		authProtectedRoutes := protectedRoutes.Group("/auth")
		{
			authProtectedRoutes.GET("/me", authHandler.GetCurrentUser)               // GET /api/v1/auth/me
			authProtectedRoutes.POST("/change-password", authHandler.ChangePassword) // POST /api/v1/auth/change-password
			authProtectedRoutes.GET("/first-login", authHandler.CheckFirstLogin)     // GET /api/v1/auth/first-login
			authProtectedRoutes.POST("/logout", authHandler.Logout)                  // POST /api/v1/auth/logout
			authProtectedRoutes.POST("/refresh", authHandler.RefreshToken)           // POST /api/v1/auth/refresh
		}

		ruleRoutes := protectedRoutes.Group("/rules")
		{
			ruleRoutes.GET("/", firewallHandler.GetRules)         // GET /api/v1/rules
			ruleRoutes.GET("/:id", firewallHandler.GetRule)       // GET /api/v1/rules/:id
			ruleRoutes.POST("/", firewallHandler.CreateRule)      // POST /api/v1/rules
			ruleRoutes.PUT("/:id", firewallHandler.UpdateRule)    // PUT /api/v1/rules/:id
			ruleRoutes.DELETE("/:id", firewallHandler.DeleteRule) // DELETE /api/v1/rules/:id
			ruleRoutes.PATCH("/:id", firewallHandler.ExecuteRule) // PATCH /api/v1/rules/:id (action=execute)
		}

		// 云服务配置路由
		cloudConfigRoutes := protectedRoutes.Group("/cloud-configs")
		{
			cloudConfigRoutes.GET("/", cloudConfigHandler.GetCloudConfigs)             // GET /api/v1/cloud-configs
			cloudConfigRoutes.GET("/:id", cloudConfigHandler.GetCloudConfig)           // GET /api/v1/cloud-configs/:id
			cloudConfigRoutes.POST("/", cloudConfigHandler.CreateCloudConfig)          // POST /api/v1/cloud-configs
			cloudConfigRoutes.PUT("/:id", cloudConfigHandler.UpdateCloudConfig)        // PUT /api/v1/cloud-configs/:id
			cloudConfigRoutes.DELETE("/:id", cloudConfigHandler.DeleteCloudConfig)     // DELETE /api/v1/cloud-configs/:id
			cloudConfigRoutes.POST("/:id/actions", cloudConfigHandler.TestCloudConfig) // POST /api/v1/cloud-configs/:id/actions (action=test)
		}

		// 配置路由
		configRoutes := protectedRoutes.Group("/configs")
		{
			configRoutes.GET("/", configHandler.GetConfigs)    // GET /api/v1/configs?category=xxx
			configRoutes.GET("/:key", configHandler.GetConfig) // GET /api/v1/configs/:key
			configRoutes.PUT("/:key", configHandler.SetConfig) // PUT /api/v1/configs/:key
			// 保持向后兼容的旧路由
			configRoutes.GET("/category/:category", configHandler.GetConfigsByCategory) // 兼容旧API
		}

		// 系统配置路由
		systemRoutes := protectedRoutes.Group("/system")
		{
			systemRoutes.GET("/config", configHandler.GetSystemConfig) // GET /api/v1/system/config
			systemRoutes.PUT("/config", configHandler.SetSystemConfig) // PUT /api/v1/system/config

			// IP相关路由
			systemRoutes.GET("/ip/current", configHandler.GetCurrentIP) // GET /api/v1/system/ip/current
			systemRoutes.POST("/ip/sync", configHandler.SyncIPNow)      // POST /api/v1/system/ip/sync
		}

		// 地域相关路由
		regionRoutes := protectedRoutes.Group("/regions")
		{
			regionRoutes.GET("/", regionHandler.SearchRegions)        // GET /api/v1/regions?provider=xxx&search=keyword
			regionRoutes.GET("/:code", regionHandler.GetRegionByCode) // GET /api/v1/regions/:code?provider=xxx
		}

		// 云厂商路由
		protectedRoutes.GET("/providers", regionHandler.GetProviders)                                           // GET /api/v1/providers
		protectedRoutes.GET("/providers/:provider/service-types", serviceTypeHandler.GetServiceTypesByProvider) // GET /api/v1/providers/:provider/service-types
	}
}

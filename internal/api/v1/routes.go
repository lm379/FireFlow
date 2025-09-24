package v1

import (
	"FireFlow/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all v1 API routes.
func RegisterRoutes(router *gin.RouterGroup, s *service.FirewallService) {
	handler := NewFirewallHandler(s)

	ruleRoutes := router.Group("/rules")
	{
		ruleRoutes.GET("/", handler.GetRules)
		ruleRoutes.POST("/", handler.CreateRule)
		ruleRoutes.DELETE("/:id", handler.DeleteRule)
	}
}

package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ServiceTypeHandler 服务类型处理器
type ServiceTypeHandler struct{}

// NewServiceTypeHandler 创建服务类型处理器
func NewServiceTypeHandler() *ServiceTypeHandler {
	return &ServiceTypeHandler{}
}

// ServiceTypeInfo 服务类型信息
type ServiceTypeInfo struct {
	Value       int    `json:"value"`        // 类型值
	Name        string `json:"name"`         // 英文名称
	DisplayName string `json:"display_name"` // 中文显示名称
	Description string `json:"description"`  // 描述
}

// ServiceTypeResponse 服务类型响应
type ServiceTypeResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Data    []ServiceTypeInfo `json:"data"`
}

// 定义所有服务类型数据
var allServiceTypes = map[string][]ServiceTypeInfo{
	"Aliyun": {
		{
			Value:       0,
			Name:        "ECS",
			DisplayName: "云服务器 ECS",
			Description: "阿里云弹性计算服务，提供安全可靠的弹性计算服务",
		},
		{
			Value:       1,
			Name:        "SWAS",
			DisplayName: "轻量应用服务器",
			Description: "阿里云轻量应用服务器，适合轻量级业务场景",
		},
	},
	"TencentCloud": {
		{
			Value:       0,
			Name:        "CVM",
			DisplayName: "云服务器 CVM",
			Description: "腾讯云云服务器，提供安全稳定的云端计算服务",
		},
		{
			Value:       1,
			Name:        "Lighthouse",
			DisplayName: "轻量应用服务器",
			Description: "腾讯云轻量应用服务器，开箱即用的应用镜像",
		},
	},
	"HuaweiCloud": {
		{
			Value:       0,
			Name:        "ECS",
			DisplayName: "弹性云服务器/Flexus云服务",
			Description: "华为云弹性云服务器，提供多种规格的云服务器",
		},
	},
}

// GetServiceTypesByProvider 根据云服务商获取服务类型 - GET /api/v1/providers/:provider/service-types
func (h *ServiceTypeHandler) GetServiceTypesByProvider(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "云服务商参数不能为空",
			"data":    nil,
		})
		return
	}

	serviceTypes, exists := allServiceTypes[provider]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "不支持的云服务商",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, ServiceTypeResponse{
		Code:    200,
		Message: "获取服务类型成功",
		Data:    serviceTypes,
	})
}

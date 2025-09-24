package v1

import (
	"FireFlow/internal/service"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ConfigHandler struct {
	configService service.ConfigService
}

func NewConfigHandler(configService service.ConfigService) *ConfigHandler {
	return &ConfigHandler{
		configService: configService,
	}
}

// SetConfigRequest 设置配置请求体
type SetConfigRequest struct {
	Key         string `json:"key" binding:"required"`
	Value       string `json:"value" binding:"required"`
	Type        string `json:"type"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// 通用配置接口

// GetConfig 获取配置值
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "配置键名不能为空"})
		return
	}

	value, err := h.configService.GetConfig(key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置项不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"key":   key,
		"value": value,
	})
}

// SetConfig 设置配置值
func (h *ConfigHandler) SetConfig(c *gin.Context) {
	var req SetConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	err := h.configService.SetConfig(req.Key, req.Value, req.Type, req.Category, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置保存成功"})
}

// GetConfigsByCategory 获取分类配置
func (h *ConfigHandler) GetConfigsByCategory(c *gin.Context) {
	category := c.Param("category")
	if category == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "配置分类不能为空"})
		return
	}

	configs, err := h.configService.GetConfigsByCategory(category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"category": category,
		"configs":  configs,
	})
}

// 系统配置接口

// GetSystemConfig 获取系统配置
func (h *ConfigHandler) GetSystemConfig(c *gin.Context) {
	configs, err := h.configService.GetConfigsByCategory("system")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转换为前端需要的格式
	result := make(map[string]interface{})
	for _, config := range configs {
		result[config.ConfigKey] = config.ConfigValue
	}

	c.JSON(http.StatusOK, result)
}

// SetSystemConfig 设置系统配置
func (h *ConfigHandler) SetSystemConfig(c *gin.Context) {
	var configMap map[string]interface{}
	if err := c.ShouldBindJSON(&configMap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for key, value := range configMap {
		valueStr := fmt.Sprintf("%v", value)
		err := h.configService.SetConfig(key, valueStr, "string", "system", "系统配置")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存配置 %s 失败: %v", key, err)})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "系统配置保存成功"})
}

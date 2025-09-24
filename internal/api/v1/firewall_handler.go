package v1

import (
	"FireFlow/internal/model"
	"FireFlow/internal/service"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type FirewallHandler struct {
	service       *service.FirewallService
	configService service.ConfigService
}

func NewFirewallHandler(s *service.FirewallService) *FirewallHandler {
	return &FirewallHandler{
		service:       s,
		configService: nil, // 将在需要时设置
	}
}

// SetConfigService 设置配置服务
func (h *FirewallHandler) SetConfigService(configService service.ConfigService) {
	h.configService = configService
}

// GetRules handles GET /api/v1/rules
func (h *FirewallHandler) GetRules(c *gin.Context) {
	rules, err := h.service.GetAllRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

// CreateRule handles POST /api/v1/rules
func (h *FirewallHandler) CreateRule(c *gin.Context) {
	var rule model.FirewallRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证必填字段
	if strings.TrimSpace(rule.Remark) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "备注为必填项"})
		return
	}

	// 如果提供了CloudConfigID，从云服务配置中获取Provider和InstanceID
	if rule.CloudConfigID != 0 && h.configService != nil {
		cloudConfig, err := h.getCloudConfigByID(rule.CloudConfigID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的云服务配置ID: " + err.Error()})
			return
		}

		// 自动填充Provider和InstanceID
		rule.Provider = cloudConfig.Provider
		rule.InstanceID = cloudConfig.InstanceId
	}

	// 设置协议默认值
	if rule.Protocol == "" {
		rule.Protocol = "TCP"
	}

	// 当协议为ICMP时，强制端口为ALL
	if rule.Protocol == "ICMP" || rule.Protocol == "ALL" {
		rule.Port = "ALL"
	}

	if err := h.service.CreateRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

// getCloudConfigByID 根据ID获取云服务配置
func (h *FirewallHandler) getCloudConfigByID(id uint) (*model.CloudProviderConfig, error) {
	if h.configService == nil {
		return nil, fmt.Errorf("配置服务不可用")
	}

	return h.configService.GetCloudConfigByID(id)
}

// DeleteRule handles DELETE /api/v1/rules/:id
func (h *FirewallHandler) DeleteRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	if err := h.service.DeleteRule(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Rule deleted successfully"})
}

// UpdateRule handles PUT /api/v1/rules/:id
func (h *FirewallHandler) UpdateRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var rule model.FirewallRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule.ID = uint(id)

	// 当协议为ICMP或ALL时，强制端口为ALL
	if rule.Protocol == "ICMP" || rule.Protocol == "ALL" {
		rule.Port = "ALL"
	}

	if err := h.service.UpdateRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// ExecuteRule handles POST /api/v1/rules/:id/execute
func (h *FirewallHandler) ExecuteRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	if err := h.service.ExecuteRule(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Rule executed successfully"})
}

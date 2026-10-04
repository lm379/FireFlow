package v1

import (
	"FireFlow/internal/core"
	"FireFlow/internal/dto"
	"FireFlow/internal/response"
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
	cronManager   *core.CronManager
}

func NewFirewallHandler(s *service.FirewallService) *FirewallHandler {
	return &FirewallHandler{
		service:       s,
		configService: nil, // 将在需要时设置
		cronManager:   nil, // 将在需要时设置
	}
}

// SetConfigService 设置配置服务
func (h *FirewallHandler) SetConfigService(configService service.ConfigService) {
	h.configService = configService
}

// SetCronManager 设置定时任务管理器
func (h *FirewallHandler) SetCronManager(cronManager *core.CronManager) {
	h.cronManager = cronManager
}

// GetRules handles GET /api/v1/rules
func (h *FirewallHandler) GetRules(c *gin.Context) {
	rules, err := h.service.GetAllRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(dto.FirewallRules(rules)))
}

// GetRule handles GET /api/v1/rules/:id
func (h *FirewallHandler) GetRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid ID format"))
		return
	}

	rule, err := h.service.GetRuleByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCode(http.StatusNotFound, "Rule not found"))
		return
	}
	c.JSON(http.StatusOK, response.Success(dto.FirewallRule(*rule)))
}

// CreateRule handles POST /api/v1/rules
func (h *FirewallHandler) CreateRule(c *gin.Context) {
	var req dto.FirewallRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, err.Error()))
		return
	}
	rule := req.Model()

	// 验证必填字段
	if strings.TrimSpace(rule.Remark) == "" {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "备注为必填项"))
		return
	}

	if err := h.validateCloudConfigID(rule.CloudConfigID); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, err.Error()))
		return
	}

	if rule.Protocol == "" {
		rule.Protocol = "TCP"
	}

	// 当协议为ICMP时，强制端口为ALL
	if rule.Protocol == "ICMP" || rule.Protocol == "ALL" {
		rule.Port = "ALL"
	}

	if err := h.service.CreateRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, response.Success(dto.FirewallRule(rule)))
}

// DeleteRule handles DELETE /api/v1/rules/:id
func (h *FirewallHandler) DeleteRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid ID format"))
		return
	}

	ruleID := uint(id)

	if err := h.service.DeleteRule(ruleID); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(nil, "Rule deleted successfully"))
}

// UpdateRule handles PUT /api/v1/rules/:id
func (h *FirewallHandler) UpdateRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid ID format"))
		return
	}

	var req dto.FirewallRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, err.Error()))
		return
	}
	rule := req.Model()
	if err := h.validateCloudConfigID(rule.CloudConfigID); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, err.Error()))
		return
	}

	ruleID := uint(id)
	rule.ID = ruleID

	// 当协议为ICMP或ALL时，强制端口为ALL
	if rule.Protocol == "ICMP" || rule.Protocol == "ALL" {
		rule.Port = "ALL"
	}

	if err := h.service.UpdateRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(dto.FirewallRule(rule)))
}

func (h *FirewallHandler) validateCloudConfigID(id uint) error {
	if id == 0 {
		return fmt.Errorf("cloud_config_id is required")
	}
	if h.configService != nil {
		if _, err := h.configService.GetCloudConfigByID(id); err != nil {
			return fmt.Errorf("invalid cloud_config_id: %w", err)
		}
	}
	return nil
}

// ExecuteRule handles PATCH /api/v1/rules/:id (with action=execute)
func (h *FirewallHandler) ExecuteRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid ID format"))
		return
	}

	// Check if this is an execute action
	var actionReq struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&actionReq); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid request body"))
		return
	}

	if actionReq.Action != "execute" {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid action. Expected 'execute'"))
		return
	}

	result, err := h.service.ExecuteRule(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, err.Error()))
		return
	}

	message, _ := result["message"].(string)
	delete(result, "message")
	c.JSON(http.StatusOK, response.Success(result, message))
}

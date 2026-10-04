package v1

import (
	"FireFlow/internal/core"
	"FireFlow/internal/dto"
	"FireFlow/internal/logger"
	"FireFlow/internal/response"
	"FireFlow/internal/service"
	"FireFlow/internal/utils"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type ConfigHandler struct {
	configService   service.ConfigService
	cronManager     *core.CronManager
	firewallService *service.FirewallService
}

func NewConfigHandler(configService service.ConfigService, cronManager *core.CronManager) *ConfigHandler {
	return &ConfigHandler{
		configService:   configService,
		cronManager:     cronManager,
		firewallService: nil, // 将在路由注册时设置
	}
}

// SetFirewallService 设置防火墙服务
func (h *ConfigHandler) SetFirewallService(firewallService *service.FirewallService) {
	h.firewallService = firewallService
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

// GetConfig 获取配置值 - GET /api/v1/configs/:key
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "配置键名不能为空"))
		return
	}

	value, err := h.configService.GetConfig(key)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCode(http.StatusNotFound, "配置项不存在"))
		return
	}

	c.JSON(http.StatusOK, response.Success(dto.ConfigValueResponse{Key: key, Value: dto.PublicConfigValue(key, value)}))
}

// SetConfig 设置配置值 - PUT /api/v1/configs/:key
func (h *ConfigHandler) SetConfig(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "配置键名不能为空"))
		return
	}

	var req struct {
		Value       string `json:"value"`
		Type        string `json:"type"`
		Category    string `json:"category"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "参数错误: "+err.Error()))
		return
	}

	err := h.configService.SetConfig(key, req.Value, req.Type, req.Category, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, "保存配置失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(dto.ConfigValueResponse{Key: key, Value: dto.PublicConfigValue(key, req.Value)}, "配置保存成功"))
}

// GetConfigs 获取配置列表 - GET /api/v1/configs?category=xxx
func (h *ConfigHandler) GetConfigs(c *gin.Context) {
	category := c.Query("category")
	if category == "" {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "配置分类不能为空"))
		return
	}

	configs, err := h.configService.GetConfigsByCategory(category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, "获取配置失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"category": category, "configs": dto.ConfigItems(configs)}, "success"))
}

// GetConfigsByCategory 获取分类配置 (保持向后兼容)
func (h *ConfigHandler) GetConfigsByCategory(c *gin.Context) {
	category := c.Param("category")
	if category == "" {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "配置分类不能为空"))
		return
	}

	configs, err := h.configService.GetConfigsByCategory(category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, "获取配置失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"category": category, "configs": dto.ConfigItems(configs)}, "success"))
}

// 系统配置接口

// GetSystemConfig 获取系统配置 - GET /api/v1/system/config
func (h *ConfigHandler) GetSystemConfig(c *gin.Context) {
	configs, err := h.configService.GetConfigsByCategory("system")
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, err.Error()))
		return
	}

	// 转换为前端需要的格式，并设置默认值
	result := make(map[string]interface{})
	for _, config := range configs {
		if dto.PublicConfigKey(config.ConfigKey) {
			result[config.ConfigKey] = config.ConfigValue
		}
	}

	// 设置默认值（如果配置不存在）
	if _, exists := result["ip_fetch_url"]; !exists {
		result["ip_fetch_url"] = "https://4.ipw.cn"
	}
	if _, exists := result["ip_check_interval"]; !exists {
		result["ip_check_interval"] = 30 // 默认30分钟
	}
	if _, exists := result["cron_enabled"]; !exists {
		result["cron_enabled"] = "false" // 默认禁用
	}

	if value, ok := result["ip_check_interval"].(string); ok {
		interval, err := strconv.Atoi(value)
		if err != nil || core.ValidateInterval(interval) != nil {
			interval = 30
		}
		result["ip_check_interval"] = interval
	}

	c.JSON(http.StatusOK, response.Success(result))
}

// SetSystemConfig 设置系统配置 - PUT /api/v1/system/config
func (h *ConfigHandler) SetSystemConfig(c *gin.Context) {
	var configMap map[string]interface{}
	if err := c.ShouldBindJSON(&configMap); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, err.Error()))
		return
	}

	// 处理定时任务相关配置
	// Merge omitted settings with stored values and validate before writing.
	cronEnabled, _ := h.configService.GetConfigBool("cron_enabled")
	intervalMinutes, err := h.configService.GetConfigInt("ip_check_interval")
	if err != nil {
		intervalMinutes = 30
	}
	if value, exists := configMap["cron_enabled"]; exists {
		cronEnabled, err = strconv.ParseBool(fmt.Sprint(value))
		if err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "定时任务启用状态无效"))
			return
		}
	}
	if value, exists := configMap["ip_check_interval"]; exists {
		intervalMinutes, err = strconv.Atoi(fmt.Sprint(value))
		if err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "IP检查间隔必须为正整数分钟"))
			return
		}
	}
	if err := core.ValidateInterval(intervalMinutes); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, err.Error()))
		return
	}
	for key, value := range configMap {
		if err := h.configService.SetConfig(key, fmt.Sprint(value), "string", "system", "系统配置"); err != nil {
			c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, fmt.Sprintf("保存配置 %s 失败: %v", key, err)))
			return
		}
	}

	if cronEnabled && intervalMinutes > 0 {
		err := h.cronManager.StartFirewallUpdateJob(intervalMinutes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, fmt.Sprintf("启动定时任务失败: %v", err)))
			return
		}
	} else {
		h.cronManager.StopFirewallUpdateJob()
	}

	c.JSON(http.StatusOK, response.Success(nil, "系统配置保存成功"))
}

// GetCurrentIP 获取当前公网IP - GET /api/v1/system/ip/current
func (h *ConfigHandler) GetCurrentIP(c *gin.Context) {
	// 获取IP获取URL配置
	ipFetchURL, err := h.configService.GetConfig("ip_fetch_url")
	if err != nil || ipFetchURL == "" {
		ipFetchURL = "https://4.ipw.cn"
	}

	// 获取当前IP
	currentIP, err := utils.GetPublicIPWithURL(ipFetchURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, err.Error()))
		return
	}

	// 检查IP合法性（只允许IPv4，禁止IPv6、JSON、报错信息、内容过长等）
	if len(currentIP) > 40 || strings.Contains(currentIP, ":") || strings.ContainsAny(currentIP, "[{") || strings.Contains(strings.ToLower(currentIP), "error") || strings.Contains(strings.ToLower(currentIP), "html") {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, "获取到的IP地址不合法"))
		return
	}
	// 简单正则校验IPv4
	ipv4Parts := strings.Split(currentIP, ".")
	if len(ipv4Parts) != 4 {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, "获取到的IP地址不是合法IPv4"))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"current_ip": currentIP}, "success"))
}

// SyncIPNow 立即获取并同步IP到防火墙规则 - POST /api/v1/system/ip/sync
func (h *ConfigHandler) SyncIPNow(c *gin.Context) {
	// 检查防火墙服务是否可用
	if h.firewallService == nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, "防火墙服务不可用"))
		return
	}

	// 获取IP获取URL配置
	result, err := h.firewallService.SyncAllRules()
	if err != nil {
		logger.Errorf("IP sync failed: %v", err)
		c.JSON(http.StatusBadGateway, response.ErrorCode(http.StatusBadGateway, fmt.Sprintf("IP同步失败，成功 %d 条，失败 %d 条: %v", result.UpdatedRules, result.FailedRules, err)))
		return
	}
	c.JSON(http.StatusOK, response.Success(gin.H{"current_ip": result.CurrentIP, "updated_rules": result.UpdatedRules, "failed_rules": result.FailedRules}, fmt.Sprintf("IP同步成功，当前IP: %s，已更新 %d 条规则", result.CurrentIP, result.UpdatedRules)))
}

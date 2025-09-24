package v1

import (
	"FireFlow/internal/core"
	"FireFlow/internal/service"
	"FireFlow/internal/utils"
	"fmt"
	"log"
	"net/http"
	"regexp"
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

	// 转换为前端需要的格式，并设置默认值
	result := make(map[string]interface{})
	for _, config := range configs {
		result[config.ConfigKey] = config.ConfigValue
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

	c.JSON(http.StatusOK, result)
}

// SetSystemConfig 设置系统配置
func (h *ConfigHandler) SetSystemConfig(c *gin.Context) {
	var configMap map[string]interface{}
	if err := c.ShouldBindJSON(&configMap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 处理定时任务相关配置
	var cronEnabled bool
	var intervalMinutes int

	for key, value := range configMap {
		valueStr := fmt.Sprintf("%v", value)
		err := h.configService.SetConfig(key, valueStr, "string", "system", "系统配置")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存配置 %s 失败: %v", key, err)})
			return
		}

		// 提取定时任务配置
		if key == "cron_enabled" {
			cronEnabled = value == "true" || value == true
		}
		if key == "ip_check_interval" {
			if val, ok := value.(float64); ok {
				intervalMinutes = int(val)
			} else if val, ok := value.(int); ok {
				intervalMinutes = val
			}
		}
	}

	// 根据配置控制定时任务
	if cronEnabled && intervalMinutes > 0 {
		err := h.cronManager.StartFirewallUpdateJob(intervalMinutes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("启动定时任务失败: %v", err)})
			return
		}
	} else {
		h.cronManager.StopFirewallUpdateJob()
	}

	c.JSON(http.StatusOK, gin.H{"message": "系统配置保存成功"})
}

// GetCurrentIP 获取当前公网IP
func (h *ConfigHandler) GetCurrentIP(c *gin.Context) {
	// 获取IP获取URL配置
	ipFetchURL, err := h.configService.GetConfig("ip_fetch_url")
	if err != nil || ipFetchURL == "" {
		ipFetchURL = "https://4.ipw.cn"
	}

	// 获取当前IP
	currentIP, err := utils.GetPublicIPWithURL(ipFetchURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "获取IP失败",
			"message":    err.Error(),
			"current_ip": "未知",
		})
		return
	}

	// 检查IP合法性（只允许IPv4，禁止IPv6、JSON、报错信息、内容过长等）
	if len(currentIP) > 40 || strings.Contains(currentIP, ":") || strings.ContainsAny(currentIP, "[{") || strings.Contains(strings.ToLower(currentIP), "error") || strings.Contains(strings.ToLower(currentIP), "html") {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "获取到的IP地址不合法",
			"message":    "获取到的IP地址不合法",
			"current_ip": "未知",
		})
		return
	}
	// 简单正则校验IPv4
	ipv4Parts := strings.Split(currentIP, ".")
	if len(ipv4Parts) != 4 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "获取到的IP地址不是合法IPv4",
			"message":    "获取到的IP地址不是合法IPv4",
			"current_ip": "未知",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"current_ip": currentIP,
	})
}

// SyncIPNow 立即获取并同步IP到防火墙规则
func (h *ConfigHandler) SyncIPNow(c *gin.Context) {
	// 检查防火墙服务是否可用
	if h.firewallService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "防火墙服务不可用",
		})
		return
	}

	// 获取IP获取URL配置
	ipFetchURL, err := h.configService.GetConfig("ip_fetch_url")
	if err != nil || ipFetchURL == "" {
		ipFetchURL = "https://4.ipw.cn"
	}

	// 获取当前IP
	currentIP, err := utils.GetPublicIPWithURL(ipFetchURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("获取IP失败: %v", err),
		})
		return
	}

	// 检查IP合法性（只允许IPv4，禁止IPv6、JSON、报错信息、内容过长等）
	if len(currentIP) > 40 || strings.Contains(currentIP, ":") || strings.ContainsAny(currentIP, "[{") || strings.Contains(strings.ToLower(currentIP), "error") || strings.Contains(strings.ToLower(currentIP), "html") {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取到的IP地址不合法，未触发规则更新。",
		})
		return
	}
	// 严格正则校验IPv4
	ipv4Pattern := `^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`
	matched, _ := regexp.MatchString(ipv4Pattern, currentIP)
	if !matched {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取到的IP地址不是合法IPv4，未触发规则更新。",
		})
		return
	}

	// 执行防火墙规则更新
	h.firewallService.UpdateAllRules()

	// 获取所有启用的规则数量作为更新计数
	updatedRules, err := h.getEnabledRulesCount()
	if err != nil {
		log.Printf("获取启用规则数量失败: %v", err)
		updatedRules = 0 // 如果获取失败，返回0
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"current_ip":    currentIP,
		"updated_rules": updatedRules,
		"message":       fmt.Sprintf("IP同步成功，当前IP: %s，已更新 %d 条规则", currentIP, updatedRules),
	})
}

// getEnabledRulesCount 获取启用的规则数量
func (h *ConfigHandler) getEnabledRulesCount() (int, error) {
	if h.firewallService == nil {
		return 0, fmt.Errorf("防火墙服务不可用")
	}
	return h.firewallService.GetEnabledRulesCount()
}

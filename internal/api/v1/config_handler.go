package v1

import (
	"FireFlow/internal/model"
	"FireFlow/internal/service"
	"net/http"

	// "strconv"

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

// 通用配置接口

// @Summary 获取配置值
// @Tags Config
// @Accept json
// @Produce json
// @Param key path string true "配置键名"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/config/{key} [get]
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

// @Summary 设置配置值
// @Tags Config
// @Accept json
// @Produce json
// @Param config body SetConfigRequest true "配置信息"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/config [post]
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

// @Summary 获取分类配置
// @Tags Config
// @Accept json
// @Produce json
// @Param category path string true "配置分类"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/config/category/{category} [get]
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

// 云服务商配置接口

// @Summary 获取云服务商配置
// @Tags CloudConfig
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cloud/configs [get]
func (h *ConfigHandler) ListCloudConfigs(c *gin.Context) {
	configs, err := h.configService.ListCloudConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取云配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"configs": configs,
	})
}

// @Summary 获取指定云服务商配置
// @Tags CloudConfig
// @Accept json
// @Produce json
// @Param provider path string true "云服务商名称"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cloud/config/{provider} [get]
func (h *ConfigHandler) GetCloudConfig(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "云服务商名称不能为空"})
		return
	}

	config, err := h.configService.GetCloudConfig(provider)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "云配置不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": config,
	})
}

// @Summary 设置云服务商配置
// @Tags CloudConfig
// @Accept json
// @Produce json
// @Param config body model.CloudProviderConfig true "云配置信息"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cloud/config [post]
func (h *ConfigHandler) SetCloudConfig(c *gin.Context) {
	var config model.CloudProviderConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	err := h.configService.SetCloudConfig(&config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存云配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "云配置保存成功"})
}

// @Summary 获取默认云服务商配置
// @Tags CloudConfig
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cloud/config/default [get]
func (h *ConfigHandler) GetDefaultCloudConfig(c *gin.Context) {
	config, err := h.configService.GetDefaultCloudConfig()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "默认云配置不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": config,
	})
}

// 定时任务配置接口

// @Summary 获取定时任务配置列表
// @Tags CronConfig
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cron/configs [get]
func (h *ConfigHandler) ListCronConfigs(c *gin.Context) {
	configs, err := h.configService.ListCronConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取定时任务配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"configs": configs,
	})
}

// @Summary 获取指定定时任务配置
// @Tags CronConfig
// @Accept json
// @Produce json
// @Param jobName path string true "任务名称"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cron/config/{jobName} [get]
func (h *ConfigHandler) GetCronConfig(c *gin.Context) {
	jobName := c.Param("jobName")
	if jobName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务名称不能为空"})
		return
	}

	config, err := h.configService.GetCronConfig(jobName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "定时任务配置不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": config,
	})
}

// @Summary 设置定时任务配置
// @Tags CronConfig
// @Accept json
// @Produce json
// @Param config body model.CronJobConfig true "定时任务配置信息"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cron/config [post]
func (h *ConfigHandler) SetCronConfig(c *gin.Context) {
	var config model.CronJobConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	err := h.configService.SetCronConfig(&config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存定时任务配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "定时任务配置保存成功"})
}

// @Summary 启用定时任务
// @Tags CronConfig
// @Accept json
// @Produce json
// @Param jobName path string true "任务名称"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cron/config/{jobName}/enable [put]
func (h *ConfigHandler) EnableCronJob(c *gin.Context) {
	jobName := c.Param("jobName")
	if jobName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务名称不能为空"})
		return
	}

	err := h.configService.EnableCronJob(jobName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "启用任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已启用"})
}

// @Summary 禁用定时任务
// @Tags CronConfig
// @Accept json
// @Produce json
// @Param jobName path string true "任务名称"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cron/config/{jobName}/disable [put]
func (h *ConfigHandler) DisableCronJob(c *gin.Context) {
	jobName := c.Param("jobName")
	if jobName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务名称不能为空"})
		return
	}

	err := h.configService.DisableCronJob(jobName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "禁用任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已禁用"})
}

// 数据迁移接口

// @Summary 从YAML配置迁移到数据库
// @Tags Config
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/config/migrate [post]
func (h *ConfigHandler) MigrateFromYAML(c *gin.Context) {
	var req MigrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 迁移云配置
	if err := h.configService.MigrateCloudConfigFromYAML(req.Config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "迁移云配置失败: " + err.Error()})
		return
	}

	// 迁移定时任务配置
	if err := h.configService.MigrateCronConfigFromYAML(req.Config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "迁移定时任务配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置迁移成功"})
}

// 请求结构体
type SetConfigRequest struct {
	Key         string `json:"key" binding:"required"`
	Value       string `json:"value" binding:"required"`
	Type        string `json:"type"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type MigrateRequest struct {
	Config map[string]interface{} `json:"config" binding:"required"`
}

// 配置管理相关的路由注册
func RegisterConfigRoutes(router *gin.RouterGroup, configHandler *ConfigHandler) {
	// 通用配置路由
	config := router.Group("/config")
	{
		config.GET("/:key", configHandler.GetConfig)
		config.POST("", configHandler.SetConfig)
		config.GET("/category/:category", configHandler.GetConfigsByCategory)
		config.POST("/migrate", configHandler.MigrateFromYAML)
	}

	// 云配置路由
	cloud := router.Group("/cloud")
	{
		cloud.GET("/configs", configHandler.ListCloudConfigs)
		cloud.GET("/config/:provider", configHandler.GetCloudConfig)
		cloud.POST("/config", configHandler.SetCloudConfig)
		cloud.GET("/config/default", configHandler.GetDefaultCloudConfig)
	}

	// 定时任务配置路由
	cron := router.Group("/cron")
	{
		cron.GET("/configs", configHandler.ListCronConfigs)
		cron.GET("/config/:jobName", configHandler.GetCronConfig)
		cron.POST("/config", configHandler.SetCronConfig)
		cron.PUT("/config/:jobName/enable", configHandler.EnableCronJob)
		cron.PUT("/config/:jobName/disable", configHandler.DisableCronJob)
	}
}

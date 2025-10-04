package v1

import (
	"FireFlow/internal/model"
	"FireFlow/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CloudConfigHandler struct {
	configService service.ConfigService
}

func NewCloudConfigHandler(configService service.ConfigService) *CloudConfigHandler {
	return &CloudConfigHandler{
		configService: configService,
	}
}

// GetCloudConfigs 获取所有云服务配置
func (h *CloudConfigHandler) GetCloudConfigs(c *gin.Context) {
	configs, err := h.configService.GetAllCloudConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, configs)
}

// GetCloudConfig 获取单个云服务配置
func (h *CloudConfigHandler) GetCloudConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	config, err := h.configService.GetCloudConfigByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cloud config not found"})
		return
	}
	c.JSON(http.StatusOK, config)
}

// CreateCloudConfig 创建云服务配置
func (h *CloudConfigHandler) CreateCloudConfig(c *gin.Context) {
	var config model.CloudProviderConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.configService.CreateCloudConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, config)
}

// UpdateCloudConfig 更新云服务配置
func (h *CloudConfigHandler) UpdateCloudConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var config model.CloudProviderConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config.ID = uint(id)
	if err := h.configService.UpdateCloudConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, config)
}

// DeleteCloudConfig 删除云服务配置
func (h *CloudConfigHandler) DeleteCloudConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	if err := h.configService.DeleteCloudConfig(uint(id)); err != nil {
		// 检查是否是外键约束错误
		if err.Error() == "无法删除云服务配置，存在关联的防火墙规则。请先删除相关规则再进行操作" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Cloud config deleted successfully"})
}

// TestCloudConfig 测试云服务配置连接
func (h *CloudConfigHandler) TestCloudConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	result, err := h.configService.TestCloudConfig(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         result.Success,
		"message":         result.Message,
		"instance_ip":     result.InstanceIP,
		"instance_exists": result.InstanceExists,
	})
}

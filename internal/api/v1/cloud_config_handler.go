package v1

import (
	"FireFlow/internal/dto"
	"FireFlow/internal/model"
	"FireFlow/internal/response"
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
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(dto.CloudConfigs(configs)))
}

// GetCloudConfig 获取单个云服务配置
func (h *CloudConfigHandler) GetCloudConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid ID format"))
		return
	}

	config, err := h.configService.GetCloudConfigByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCode(http.StatusNotFound, "Cloud config not found"))
		return
	}
	c.JSON(http.StatusOK, response.Success(dto.CloudConfig(*config)))
}

// CreateCloudConfig 创建云服务配置
func (h *CloudConfigHandler) CreateCloudConfig(c *gin.Context) {
	var req dto.CloudConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, err.Error()))
		return
	}

	config := model.CloudProviderConfig{IsEnabled: true}
	req.ApplyTo(&config)
	if err := h.configService.CreateCloudConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, err.Error()))
		return
	}
	c.JSON(http.StatusCreated, response.Success(dto.CloudConfig(config)))
}

// UpdateCloudConfig 更新云服务配置
func (h *CloudConfigHandler) UpdateCloudConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid ID format"))
		return
	}

	var req dto.CloudConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, err.Error()))
		return
	}

	config, err := h.configService.GetCloudConfigByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCode(http.StatusNotFound, "Cloud config not found"))
		return
	}
	if req.Provider != nil && *req.Provider != config.Provider &&
		(req.SecretID == nil || *req.SecretID == "" || req.SecretKey == nil || *req.SecretKey == "") {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Changing provider requires new credentials"))
		return
	}
	req.ApplyTo(config)
	if err := h.configService.UpdateCloudConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(dto.CloudConfig(*config)))
}

// DeleteCloudConfig 删除云服务配置
func (h *CloudConfigHandler) DeleteCloudConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid ID format"))
		return
	}

	if err := h.configService.DeleteCloudConfig(uint(id)); err != nil {
		// 检查是否是外键约束错误
		if err.Error() == "无法删除云服务配置，存在关联的防火墙规则。请先删除相关规则再进行操作" {
			c.JSON(http.StatusConflict, response.ErrorCode(http.StatusConflict, err.Error()))
		} else {
			c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, response.Success(nil, "Cloud config deleted successfully"))
}

// TestCloudConfig 测试云服务配置连接 - POST /api/v1/cloud-configs/:id/actions with action=test
func (h *CloudConfigHandler) TestCloudConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid ID format"))
		return
	}

	var actionReq struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&actionReq); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid request body"))
		return
	}

	if actionReq.Action != "test" {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid action. Expected 'test'"))
		return
	}

	result, err := h.configService.TestCloudConfig(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCode(http.StatusInternalServerError, err.Error()))
		return
	}

	if !result.Success {
		c.JSON(http.StatusBadGateway, response.ErrorCode(http.StatusBadGateway, result.Message))
		return
	}
	c.JSON(http.StatusOK, response.Success(nil, result.Message))
}

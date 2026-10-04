package v1

import (
	"FireFlow/internal/response"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StatusActionRequest struct {
	Action string `json:"action" binding:"required,oneof=enable disable"`
}

func parseStatusAction(c *gin.Context) (uint, bool, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "Invalid ID format"))
		return 0, false, false
	}
	var request StatusActionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "action must be enable or disable"))
		return 0, false, false
	}
	return uint(id), request.Action == "enable", true
}

func statusActionError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, response.ErrorCode(http.StatusNotFound, "Resource not found"))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Error(err.Error()))
}

// SetStatus handles POST /api/v1/rules/:id/status.
func (h *FirewallHandler) SetStatus(c *gin.Context) {
	id, enabled, ok := parseStatusAction(c)
	if !ok {
		return
	}
	if err := h.service.SetRuleEnabled(id, enabled); err != nil {
		statusActionError(c, err)
		return
	}
	message := "规则已禁用"
	if enabled {
		message = "规则已启用"
	}
	c.JSON(http.StatusOK, response.Success(gin.H{"id": id, "enabled": enabled}, message))
}

// SetStatus handles POST /api/v1/cloud-configs/:id/status.
func (h *CloudConfigHandler) SetStatus(c *gin.Context) {
	id, enabled, ok := parseStatusAction(c)
	if !ok {
		return
	}
	if err := h.configService.SetCloudConfigEnabled(id, enabled); err != nil {
		statusActionError(c, err)
		return
	}
	message := "实例已禁用"
	if enabled {
		message = "实例已启用"
	}
	c.JSON(http.StatusOK, response.Success(gin.H{"id": id, "is_enabled": enabled}, message))
}

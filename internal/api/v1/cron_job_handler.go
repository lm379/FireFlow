package v1

import (
	"FireFlow/internal/model"
	"FireFlow/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CronJobHandler struct {
	configService service.ConfigService
}

func NewCronJobHandler(configService service.ConfigService) *CronJobHandler {
	return &CronJobHandler{
		configService: configService,
	}
}

// GetCronJobs 获取所有定时任务
func (h *CronJobHandler) GetCronJobs(c *gin.Context) {
	jobs, err := h.configService.GetAllCronJobs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, jobs)
}

// CreateCronJob 创建定时任务
func (h *CronJobHandler) CreateCronJob(c *gin.Context) {
	var job model.CronJobConfig
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.configService.CreateCronJob(&job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, job)
}

// UpdateCronJob 更新定时任务
func (h *CronJobHandler) UpdateCronJob(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var job model.CronJobConfig
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job.ID = uint(id)
	if err := h.configService.UpdateCronJob(&job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

// DeleteCronJob 删除定时任务
func (h *CronJobHandler) DeleteCronJob(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	if err := h.configService.DeleteCronJob(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Cron job deleted successfully"})
}

// RunCronJob 立即运行定时任务
func (h *CronJobHandler) RunCronJob(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	if err := h.configService.RunCronJob(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Cron job executed successfully"})
}

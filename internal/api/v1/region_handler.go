package v1

import (
	"FireFlow/internal/response"
	"FireFlow/pkg/cloud"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// RegionHandler 地域API处理器
type RegionHandler struct{}

// NewRegionHandler 创建地域处理器
func NewRegionHandler() *RegionHandler {
	return &RegionHandler{}
}

// GetRegions 获取地域列表
// @Summary 获取地域列表
// @Description 根据云厂商获取地域列表，用于前端下拉框选择
// @Tags regions
// @Accept json
// @Produce json
// @Param provider query string true "云厂商" Enums(aliyun, tencent, huawei)
// @Param page query int false "页码" default(1)
// @Param limit query int false "每页数量" default(50)
// @Success 200 {object} response.Response
// @Router /api/v1/regions [get]
func (h *RegionHandler) GetRegions(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "provider参数不能为空"))
		return
	}

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	// 获取地域选项
	options := cloud.GetRegionOptions(provider)
	total := len(options)

	// 分页处理
	start := (page - 1) * limit
	end := start + limit
	if start >= total {
		options = []cloud.RegionOption{}
	} else {
		if end > total {
			end = total
		}
		options = options[start:end]
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"items": options, "total": total}, "success"))
}

// SearchRegions 搜索地域
// @Summary 获取地域列表(支持搜索)
// @Description 获取地域列表，支持按关键词搜索 - GET /api/v1/regions?provider=xxx&search=keyword
// @Tags regions
// @Accept json
// @Produce json
// @Param provider query string false "云厂商" Enums(aliyun, tencent, huawei)
// @Param search query string false "搜索关键词"
// @Param page query int false "页码" default(1)
// @Param limit query int false "每页数量" default(20)
// @Success 200 {object} response.Response
// @Router /api/v1/regions [get]
func (h *RegionHandler) SearchRegions(c *gin.Context) {
	provider := c.Query("provider")
	keyword := c.Query("search") // 改为使用search参数

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// 搜索地域
	regions := cloud.SearchRegions(provider, keyword)
	total := len(regions)

	// 分页处理
	start := (page - 1) * limit
	end := start + limit
	if start >= total {
		regions = []cloud.Region{}
	} else {
		if end > total {
			end = total
		}
		regions = regions[start:end]
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"items": regions, "total": total}, "success"))
}

// GetProviders 获取支持的云厂商列表
// @Summary 获取云厂商列表
// @Description 获取所有支持的云厂商列表
// @Tags regions
// @Accept json
// @Produce json
// @Success 200 {object} response.Response
// @Router /api/v1/providers [get]
func (h *RegionHandler) GetProviders(c *gin.Context) {
	providers := cloud.GetProviders()

	c.JSON(http.StatusOK, response.Success(providers, "success"))
}

// GetRegionByCode 根据代码获取地域信息
// @Summary 获取地区详情
// @Description 根据地域代码获取具体地域信息 - GET /api/v1/regions/:code?provider=xxx
// @Tags regions
// @Accept json
// @Produce json
// @Param code path string true "地域代码"
// @Param provider query string true "云厂商"
// @Success 200 {object} response.Response
// @Router /api/v1/regions/{code} [get]
func (h *RegionHandler) GetRegionByCode(c *gin.Context) {
	provider := c.Query("provider")
	code := c.Param("code") // 改为从路径参数获取

	if provider == "" || code == "" {
		c.JSON(http.StatusBadRequest, response.ErrorCode(http.StatusBadRequest, "provider和code参数不能为空"))
		return
	}

	region := cloud.GetRegionByCode(provider, code)
	if region == nil {
		c.JSON(http.StatusNotFound, response.ErrorCode(http.StatusNotFound, "未找到对应的地域信息"))
		return
	}

	c.JSON(http.StatusOK, response.Success(region, "success"))
}

package v1

import (
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

// RegionListResponse 地域列表响应
type RegionListResponse struct {
	Code    int                  `json:"code"`
	Message string               `json:"message"`
	Data    []cloud.RegionOption `json:"data"`
	Total   int                  `json:"total"`
}

// SearchRegionResponse 地域搜索响应
type SearchRegionResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    []cloud.Region `json:"data"`
	Total   int            `json:"total"`
}

// ProviderListResponse 云厂商列表响应
type ProviderListResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Data    []string `json:"data"`
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
// @Success 200 {object} RegionListResponse
// @Router /api/v1/regions [get]
func (h *RegionHandler) GetRegions(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "provider参数不能为空",
			"data":    nil,
		})
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

	c.JSON(http.StatusOK, RegionListResponse{
		Code:    200,
		Message: "success",
		Data:    options,
		Total:   total,
	})
}

// SearchRegions 搜索地域
// @Summary 搜索地域
// @Description 根据关键词模糊搜索地域，支持按云厂商筛选
// @Tags regions
// @Accept json
// @Produce json
// @Param provider query string false "云厂商" Enums(aliyun, tencent, huawei)
// @Param keyword query string false "搜索关键词"
// @Param page query int false "页码" default(1)
// @Param limit query int false "每页数量" default(20)
// @Success 200 {object} SearchRegionResponse
// @Router /api/v1/regions/search [get]
func (h *RegionHandler) SearchRegions(c *gin.Context) {
	provider := c.Query("provider")
	keyword := c.Query("keyword")

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

	c.JSON(http.StatusOK, SearchRegionResponse{
		Code:    200,
		Message: "success",
		Data:    regions,
		Total:   total,
	})
}

// GetProviders 获取支持的云厂商列表
// @Summary 获取云厂商列表
// @Description 获取所有支持的云厂商列表
// @Tags regions
// @Accept json
// @Produce json
// @Success 200 {object} ProviderListResponse
// @Router /api/v1/providers [get]
func (h *RegionHandler) GetProviders(c *gin.Context) {
	providers := cloud.GetProviders()

	c.JSON(http.StatusOK, ProviderListResponse{
		Code:    200,
		Message: "success",
		Data:    providers,
	})
}

// GetRegionByCode 根据代码获取地域信息
// @Summary 根据代码获取地域信息
// @Description 根据云厂商和地域代码获取地域详细信息
// @Tags regions
// @Accept json
// @Produce json
// @Param provider query string true "云厂商"
// @Param code query string true "地域代码"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/regions/detail [get]
func (h *RegionHandler) GetRegionByCode(c *gin.Context) {
	provider := c.Query("provider")
	code := c.Query("code")

	if provider == "" || code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "provider和code参数不能为空",
			"data":    nil,
		})
		return
	}

	region := cloud.GetRegionByCode(provider, code)
	if region == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "未找到对应的地域信息",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    region,
	})
}

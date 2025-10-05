package cloud

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Region 表示云厂商的地域信息
type Region struct {
	Code     string `json:"code"`     // 地域代码，如 cn-beijing
	Name     string `json:"name"`     // 中文名称，如 北京
	Provider string `json:"provider"` // 云厂商，如 aliyun, tencent, huawei
}

// RegionManager 地域管理器
type RegionManager struct {
	regions map[string][]Region
	mu      sync.RWMutex
}

var (
	regionManager *RegionManager
	once          sync.Once
)

// getRegionManager 获取地域管理器实例（单例模式）
func getRegionManager() *RegionManager {
	once.Do(func() {
		regionManager = &RegionManager{
			regions: make(map[string][]Region),
		}
		// 尝试加载配置文件
		if err := regionManager.LoadFromFile("configs/regions.json"); err != nil {
			// 如果加载失败，使用默认配置
			regionManager.loadDefaultRegions()
		}
	})
	return regionManager
}

// LoadFromFile 从JSON文件加载地域配置
func (rm *RegionManager) LoadFromFile(filename string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 如果文件路径不是绝对路径，尝试相对于项目根目录
	if !filepath.IsAbs(filename) {
		// 获取当前工作目录
		if wd, err := os.Getwd(); err == nil {
			filename = filepath.Join(wd, filename)
		}
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read regions file: %w", err)
	}

	var regions map[string][]Region
	if err := json.Unmarshal(data, &regions); err != nil {
		return fmt.Errorf("failed to parse regions JSON: %w", err)
	}

	rm.regions = regions
	return nil
}

// loadDefaultRegions 加载默认地域配置（作为备用）
func (rm *RegionManager) loadDefaultRegions() {
	rm.regions = map[string][]Region{
		"aliyun": {
			{Code: "cn-beijing", Name: "华北2（北京）", Provider: "aliyun"},
			{Code: "cn-hangzhou", Name: "华东1（杭州）", Provider: "aliyun"},
			{Code: "cn-shanghai", Name: "华东2（上海）", Provider: "aliyun"},
			{Code: "cn-shenzhen", Name: "华南1（深圳）", Provider: "aliyun"},
		},
		"tencent": {
			{Code: "ap-beijing", Name: "北京", Provider: "tencent"},
			{Code: "ap-shanghai", Name: "上海", Provider: "tencent"},
			{Code: "ap-guangzhou", Name: "广州", Provider: "tencent"},
		},
		"huawei": {
			{Code: "cn-north-1", Name: "华北-北京一", Provider: "huawei"},
			{Code: "cn-north-2", Name: "华北-北京二", Provider: "huawei"},
			{Code: "cn-east-3", Name: "华东-上海一", Provider: "huawei"},
		},
	}
}

// GetRegionsByProvider 根据云厂商获取地域列表
func GetRegionsByProvider(provider string) []Region {
	rm := getRegionManager()
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if regions, exists := rm.regions[provider]; exists {
		// 返回副本以避免数据竞争
		result := make([]Region, len(regions))
		copy(result, regions)
		return result
	}
	return nil
}

// GetRegionByCode 根据地域代码和云厂商获取地域信息
func GetRegionByCode(provider, code string) *Region {
	regions := GetRegionsByProvider(provider)
	for _, region := range regions {
		if region.Code == code {
			// 返回副本
			regionCopy := region
			return &regionCopy
		}
	}
	return nil
}

// GetAllRegions 获取所有云厂商的地域列表
func GetAllRegions() []Region {
	rm := getRegionManager()
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var allRegions []Region
	for _, regions := range rm.regions {
		allRegions = append(allRegions, regions...)
	}
	return allRegions
}

// GetProviders 获取所有支持的云厂商列表
func GetProviders() []string {
	rm := getRegionManager()
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	providers := make([]string, 0, len(rm.regions))
	for provider := range rm.regions {
		providers = append(providers, provider)
	}
	return providers
}

// ReloadRegions 重新加载地域配置（用于热更新）
func ReloadRegions(filename string) error {
	rm := getRegionManager()
	return rm.LoadFromFile(filename)
}

// AddProvider 添加新的云厂商地域配置
func AddProvider(provider string, regions []Region) {
	rm := getRegionManager()
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.regions[provider] = regions
}

// SearchRegions 模糊搜索地域
func SearchRegions(provider, keyword string) []Region {
	var regions []Region
	
	if provider != "" {
		// 搜索特定云厂商的地域
		regions = GetRegionsByProvider(provider)
	} else {
		// 搜索所有地域
		regions = GetAllRegions()
	}
	
	if keyword == "" {
		return regions
	}
	
	// 模糊匹配
	var result []Region
	keyword = strings.ToLower(keyword)
	
	for _, region := range regions {
		// 匹配地域代码或名称
		if strings.Contains(strings.ToLower(region.Code), keyword) ||
			strings.Contains(strings.ToLower(region.Name), keyword) {
			result = append(result, region)
		}
	}
	
	return result
}

// GetRegionOptions 获取地域选项（用于前端下拉框）
type RegionOption struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Label string `json:"label"` // 显示标签，格式为 "名称 (代码)"
}

func GetRegionOptions(provider string) []RegionOption {
	regions := GetRegionsByProvider(provider)
	options := make([]RegionOption, len(regions))
	
	for i, region := range regions {
		options[i] = RegionOption{
			Code:  region.Code,
			Name:  region.Name,
			Label: fmt.Sprintf("%s (%s)", region.Name, region.Code),
		}
	}
	
	return options
}

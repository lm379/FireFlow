package cloud

import (
	"fmt"
	"strings"
	"sync"
)

// Region 表示云厂商的地域信息
type Region struct {
	Code string `json:"code"` // 地域代码，如 cn-beijing
	Name string `json:"name"` // 中文名称，如 北京
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
		// 直接加载硬编码的地域配置
		regionManager.loadRegions()
	})
	return regionManager
}

// loadRegions 加载地域配置（硬编码数据）
func (rm *RegionManager) loadRegions() {
	rm.regions = map[string][]Region{
		"TencentCloud": {
			{Code: "ap-beijing", Name: "北京"},
			{Code: "ap-shanghai", Name: "上海"},
			{Code: "ap-guangzhou", Name: "广州"},
			{Code: "ap-chengdu", Name: "成都"},
			{Code: "ap-nanjing", Name: "南京"},
			{Code: "ap-hongkong", Name: "中国香港"},
			{Code: "ap-singapore", Name: "新加坡"},
			{Code: "na-siliconvalley", Name: "硅谷"},
			{Code: "ap-tokyo", Name: "东京"},
			{Code: "ap-seoul", Name: "首尔"},
			{Code: "eu-frankfurt", Name: "法兰克福"},
			{Code: "ap-jakarta", Name: "雅加达"},
			{Code: "na-ashburn", Name: "弗吉尼亚"},
			{Code: "ap-bangkok", Name: "曼谷"},
			{Code: "sa-saopaulo", Name: "圣保罗"},
			{Code: "ap-chongqing", Name: "重庆"},
			{Code: "me-saudi-arabia", Name: "沙特阿拉伯"},
		},
		"Aliyun": {
			{Code: "cn-qingdao", Name: "华北1（青岛）"},
			{Code: "cn-beijing", Name: "华北2（北京）"},
			{Code: "cn-zhangjiakou", Name: "华北3（张家口）"},
			{Code: "cn-huhehaote", Name: "华北5（呼和浩特）"},
			{Code: "cn-wulanchabu", Name: "华北6（乌兰察布）"},
			{Code: "cn-hangzhou", Name: "华东1（杭州）"},
			{Code: "cn-shanghai", Name: "华东2（上海）"},
			{Code: "cn-nanjing", Name: "华东5 （南京）"},
			{Code: "cn-fuzhou", Name: "华东6（福州）"},
			{Code: "cn-shenzhen", Name: "华南1（深圳）"},
			{Code: "cn-heyuan", Name: "华南2（河源）"},
			{Code: "cn-guangzhou", Name: "华南3（广州）"},
			{Code: "cn-wuhan-lr", Name: "华中1（武汉）"},
			{Code: "cn-chengdu", Name: "西南1（成都）"},
			{Code: "cn-hongkong", Name: "中国香港"},
			{Code: "ap-southeast-1", Name: "新加坡"},
			{Code: "ap-southeast-3", Name: "马来西亚（吉隆坡）"},
			{Code: "ap-southeast-5", Name: "印度尼西亚（雅加达）"},
			{Code: "ap-southeast-6", Name: "菲律宾（马尼拉）"},
			{Code: "ap-southeast-7", Name: "泰国（曼谷）"},
			{Code: "ap-northeast-1", Name: "日本（东京）"},
			{Code: "ap-northeast-2", Name: "韩国（首尔）"},
			{Code: "us-west-1", Name: "美国（硅谷）"},
			{Code: "us-east-1", Name: "美国（弗吉尼亚）"},
			{Code: "eu-central-1", Name: "德国（法兰克福）"},
			{Code: "eu-west-1", Name: "英国（伦敦）"},
			{Code: "me-east-1", Name: "阿联酋（迪拜）"},
			{Code: "me-central-1", Name: "沙特（利雅得）"},
			{Code: "na-south-1", Name: "墨西哥"},
		},
		"HuaweiCloud": {
			{Code: "ae-ad-1", Name: "中东-阿布扎比-OP5"},
			{Code: "af-north-1", Name: "非洲-开罗"},
			{Code: "af-south-1", Name: "非洲-约翰内斯堡"},
			{Code: "ap-southeast-1", Name: "中国-香港"},
			{Code: "ap-southeast-2", Name: "亚太-曼谷"},
			{Code: "ap-southeast-3", Name: "亚太-新加坡"},
			{Code: "ap-southeast-4", Name: "亚太-雅加达"},
			{Code: "ap-southeast-5", Name: "亚太-马尼拉"},
			{Code: "cn-east-2", Name: "华东-上海二"},
			{Code: "cn-east-3", Name: "华东-上海一"},
			{Code: "cn-east-4", Name: "华东二"},
			{Code: "cn-east-5", Name: "华东-青岛"},
			{Code: "cn-north-1", Name: "华北-北京一"},
			{Code: "cn-north-11", Name: "华北-乌兰察布-汽车一"},
			{Code: "cn-north-12", Name: "华北三"},
			{Code: "cn-north-2", Name: "华北-北京二"},
			{Code: "cn-north-4", Name: "华北-北京四"},
			{Code: "cn-north-9", Name: "华北-乌兰察布一"},
			{Code: "cn-south-1", Name: "华南-广州"},
			{Code: "cn-south-2", Name: "华南-深圳"},
			{Code: "cn-south-4", Name: "华南-广州-友好用户环境"},
			{Code: "cn-southwest-2", Name: "西南-贵阳一"},
			{Code: "cn-southwest-3", Name: "西南-贵阳-汽车二"},
			{Code: "eu-west-0", Name: "欧洲-巴黎"},
			{Code: "eu-west-101", Name: "欧洲-都柏林"},
			{Code: "la-north-2", Name: "拉美-墨西哥城二"},
			{Code: "la-south-2", Name: "拉美-圣地亚哥"},
			{Code: "me-east-1", Name: "中东-利雅得"},
			{Code: "my-kualalumpur-1", Name: "亚太-吉隆坡-OP6"},
			{Code: "na-mexico-1", Name: "拉美-墨西哥城一"},
			{Code: "ru-moscow-1", Name: "俄罗斯-莫斯科-OP4"},
			{Code: "ru-northwest-2", Name: "俄罗斯-莫斯科二"},
			{Code: "sa-brazil-1", Name: "拉美-圣保罗一"},
			{Code: "tr-west-1", Name: "土耳其-伊斯坦布尔"},
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

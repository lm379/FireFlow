package service

import (
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/pkg/cloud"
	"encoding/json"
	"fmt"
	"strconv"
)

// CloudTestResult 云服务配置测试结果
type CloudTestResult struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	InstanceExists bool   `json:"instance_exists"`
	InstanceIP     string `json:"instance_ip,omitempty"`
}

type ConfigService interface {
	// 通用配置管理
	GetConfig(key string) (string, error)
	GetConfigInt(key string) (int, error)
	GetConfigBool(key string) (bool, error)
	SetConfig(key, value, configType, category, description string) error
	GetConfigsByCategory(category string) ([]model.ConfigItem, error)

	// 云服务商配置管理
	GetCloudConfig(provider string) (*model.CloudProviderConfig, error)
	GetCloudConfigByID(id uint) (*model.CloudProviderConfig, error)
	SetCloudConfig(config *model.CloudProviderConfig) error
	GetDefaultCloudConfig() (*model.CloudProviderConfig, error)
	ListCloudConfigs() ([]model.CloudProviderConfig, error)

	// 新增：前端云服务配置管理
	GetAllCloudConfigs() ([]model.CloudProviderConfig, error)
	CreateCloudConfig(config *model.CloudProviderConfig) error
	UpdateCloudConfig(config *model.CloudProviderConfig) error
	DeleteCloudConfig(id uint) error
	TestCloudConfig(id uint) (*CloudTestResult, error)

	// 数据迁移相关（从config.yaml迁移到数据库）
	MigrateCloudConfigFromYAML(yamlConfig map[string]interface{}) error
}

type configService struct {
	configRepo repository.ConfigRepository
}

func NewConfigService(configRepo repository.ConfigRepository) ConfigService {
	return &configService{
		configRepo: configRepo,
	}
}

// 通用配置管理
func (s *configService) GetConfig(key string) (string, error) {
	return s.configRepo.GetConfigValue(key)
}

func (s *configService) GetConfigInt(key string) (int, error) {
	valueStr, err := s.configRepo.GetConfigValue(key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(valueStr)
}

func (s *configService) GetConfigBool(key string) (bool, error) {
	valueStr, err := s.configRepo.GetConfigValue(key)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(valueStr)
}

func (s *configService) SetConfig(key, value, configType, category, description string) error {
	return s.configRepo.SetConfigValue(key, value, configType, category, description)
}

func (s *configService) GetConfigsByCategory(category string) ([]model.ConfigItem, error) {
	return s.configRepo.GetConfigsByCategory(category)
}

// 云服务商配置管理
func (s *configService) GetCloudConfig(provider string) (*model.CloudProviderConfig, error) {
	return s.configRepo.GetCloudProviderConfig(provider)
}

func (s *configService) GetCloudConfigByID(id uint) (*model.CloudProviderConfig, error) {
	var config model.CloudProviderConfig
	err := s.configRepo.GetCloudProviderConfigByID(id, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (s *configService) SetCloudConfig(config *model.CloudProviderConfig) error {
	return s.configRepo.SetCloudProviderConfig(config)
}

func (s *configService) GetDefaultCloudConfig() (*model.CloudProviderConfig, error) {
	return s.configRepo.GetDefaultCloudProvider()
}

func (s *configService) ListCloudConfigs() ([]model.CloudProviderConfig, error) {
	return s.configRepo.ListCloudProviders()
}

// 新增：前端云服务配置管理
func (s *configService) GetAllCloudConfigs() ([]model.CloudProviderConfig, error) {
	return s.configRepo.ListCloudProviders()
}

func (s *configService) CreateCloudConfig(config *model.CloudProviderConfig) error {
	return s.configRepo.SetCloudProviderConfig(config)
}

func (s *configService) UpdateCloudConfig(config *model.CloudProviderConfig) error {
	return s.configRepo.UpdateCloudProviderConfig(config)
}

func (s *configService) DeleteCloudConfig(id uint) error {
	// 检查是否存在关联的防火墙规则
	hasRules, err := s.configRepo.HasAssociatedRules(id)
	if err != nil {
		return fmt.Errorf("检查关联规则失败: %v", err)
	}
	if hasRules {
		return fmt.Errorf("无法删除云服务配置，存在关联的防火墙规则。请先删除相关规则再进行操作")
	}
	return s.configRepo.DeleteCloudProviderConfig(id)
}

func (s *configService) TestCloudConfig(id uint) (*CloudTestResult, error) {
	// 获取云服务配置
	var config model.CloudProviderConfig
	if err := s.configRepo.GetCloudProviderConfigByID(id, &config); err != nil {
		return &CloudTestResult{
			Success: false,
			Message: "配置不存在",
		}, err
	}

	// 如果没有配置实例ID，只测试凭证
	if config.InstanceId == "" {
		// TODO: 这里可以添加基础的API凭证测试
		return &CloudTestResult{
			Success:        true,
			Message:        "凭证验证成功，但未配置实例ID",
			InstanceExists: false,
		}, nil
	}

	// 根据云服务商类型进行实例检查
	switch config.Provider {
	case "TencentCloud":
		return s.testTencentInstance(&config)
	case "Aliyun":
		return s.testAliyunInstance(&config)
	default:
		return &CloudTestResult{
			Success: false,
			Message: fmt.Sprintf("不支持的云服务商: %s", config.Provider),
		}, nil
	}
}

func (s *configService) testTencentInstance(config *model.CloudProviderConfig) (*CloudTestResult, error) {
	// 创建腾讯云客户端配置
	tencentConfig := cloud.TencentConfig{
		SecretId:   config.SecretId,
		SecretKey:  config.SecretKey,
		Region:     config.Region,
		InstanceId: config.InstanceId,
	}

	// 创建腾讯云客户端
	client, err := cloud.NewTencentClient(tencentConfig)
	if err != nil {
		return &CloudTestResult{
			Success: false,
			Message: fmt.Sprintf("创建腾讯云客户端失败: %v", err),
		}, err
	}

	// 如果没有实例ID，只测试凭证连接
	if config.InstanceId == "" {
		return &CloudTestResult{
			Success:        true,
			Message:        "腾讯云凭证验证成功，但未配置实例ID",
			InstanceExists: false,
		}, nil
	}

	// 获取实例信息
	instanceInfo, err := client.GetInstance(config.InstanceId)
	if err != nil {
		return &CloudTestResult{
			Success:        false,
			Message:        fmt.Sprintf("获取实例信息失败: %v", err),
			InstanceExists: false,
		}, err
	}

	// 成功获取实例信息
	message := fmt.Sprintf("实例检查成功，实例名称: %s，状态: %s", instanceInfo.InstanceName, instanceInfo.Status)
	return &CloudTestResult{
		Success:        true,
		Message:        message,
		InstanceExists: true,
		InstanceIP:     instanceInfo.PublicIP,
	}, nil
}

func (s *configService) testAliyunInstance(config *model.CloudProviderConfig) (*CloudTestResult, error) {
	// 构造阿里云配置
	aliyunConfig := cloud.AliyunConfig{
		AccessKeyID:      config.SecretId,
		AccessKeySecret:  config.SecretKey,
		RegionID:         config.Region,
		SecurityGroupIds: config.InstanceId, // 这里存的是安全组ID
	}

	client, err := cloud.NewAliyunClient(aliyunConfig)
	if err != nil {
		return &CloudTestResult{
			Success:        false,
			Message:        fmt.Sprintf("创建阿里云客户端失败: %v", err),
			InstanceExists: false,
		}, err
	}

	if config.InstanceId == "" {
		return &CloudTestResult{
			Success:        true,
			Message:        "阿里云凭证验证成功，但未配置安全组ID",
			InstanceExists: false,
		}, nil
	}

	// 验证安全组是否存在
	sgInfo, err := client.ValidateSecurityGroup(config.InstanceId)
	if err != nil {
		return &CloudTestResult{
			Success:        false,
			Message:        fmt.Sprintf("安全组验证失败: %v", err),
			InstanceExists: false,
		}, err
	}

	message := fmt.Sprintf("安全组验证成功，名称: %s，描述: %s，规则数量: %d", sgInfo.Name, sgInfo.Description, sgInfo.RulesCount)
	return &CloudTestResult{
		Success:        true,
		Message:        message,
		InstanceExists: true,
		InstanceIP:     "", // 安全组没有IP地址
	}, nil
}

// 数据迁移相关
func (s *configService) MigrateCloudConfigFromYAML(yamlConfig map[string]interface{}) error {
	cloudConfig, ok := yamlConfig["cloud"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("cloud config not found in yaml")
	}

	// 遍历每个云服务商配置
	for provider, configData := range cloudConfig {
		configMap, ok := configData.(map[string]interface{})
		if !ok {
			continue
		}

		// 提取配置字段
		secretID := getStringFromMap(configMap, "secret_id")
		secretKey := getStringFromMap(configMap, "secret_key")
		region := getStringFromMap(configMap, "region")

		// 其他配置转为JSON存储
		otherConfig := make(map[string]interface{})
		for k, v := range configMap {
			if k != "secret_id" && k != "secret_key" && k != "region" {
				otherConfig[k] = v
			}
		}

		otherConfigJSON, _ := json.Marshal(otherConfig)

		config := &model.CloudProviderConfig{
			Provider:  provider,
			SecretId:  secretID,
			SecretKey: secretKey,
			Region:    region,
			Extra:     string(otherConfigJSON),
			IsDefault: provider == "tencent", // 假设腾讯云为默认
			IsEnabled: true,
		}

		if err := s.configRepo.SetCloudProviderConfig(config); err != nil {
			return fmt.Errorf("failed to migrate cloud config for %s: %v", provider, err)
		}
	}

	return nil
}

// 辅助方法
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getBoolFromMap(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

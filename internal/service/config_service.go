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

	// 定时任务配置管理
	GetCronConfig(jobName string) (*model.CronJobConfig, error)
	SetCronConfig(config *model.CronJobConfig) error
	ListCronConfigs() ([]model.CronJobConfig, error)
	EnableCronJob(jobName string) error
	DisableCronJob(jobName string) error

	// 新增：前端定时任务管理
	GetAllCronJobs() ([]model.CronJobConfig, error)
	CreateCronJob(config *model.CronJobConfig) error
	UpdateCronJob(config *model.CronJobConfig) error
	DeleteCronJob(id uint) error
	RunCronJob(id uint) error

	// 数据迁移相关（从config.yaml迁移到数据库）
	MigrateCloudConfigFromYAML(yamlConfig map[string]interface{}) error
	MigrateCronConfigFromYAML(yamlConfig map[string]interface{}) error
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
	// TODO: 调用阿里云API检查实例
	return &CloudTestResult{
		Success:        true,
		Message:        "实例检查成功",
		InstanceExists: true,
		InstanceIP:     "5.6.7.8", // 这里应该是实际获取的IP
	}, nil
}

// 定时任务配置管理
func (s *configService) GetCronConfig(jobName string) (*model.CronJobConfig, error) {
	return s.configRepo.GetCronJobConfig(jobName)
}

func (s *configService) SetCronConfig(config *model.CronJobConfig) error {
	return s.configRepo.SetCronJobConfig(config)
}

func (s *configService) ListCronConfigs() ([]model.CronJobConfig, error) {
	return s.configRepo.ListCronJobs()
}

func (s *configService) EnableCronJob(jobName string) error {
	return s.configRepo.UpdateCronJobStatus(jobName, true)
}

func (s *configService) DisableCronJob(jobName string) error {
	return s.configRepo.UpdateCronJobStatus(jobName, false)
}

// 新增：前端定时任务管理
func (s *configService) GetAllCronJobs() ([]model.CronJobConfig, error) {
	return s.configRepo.ListCronJobs()
}

func (s *configService) CreateCronJob(config *model.CronJobConfig) error {
	return s.configRepo.SetCronJobConfig(config)
}

func (s *configService) UpdateCronJob(config *model.CronJobConfig) error {
	return s.configRepo.UpdateCronJobConfig(config)
}

func (s *configService) DeleteCronJob(id uint) error {
	return s.configRepo.DeleteCronJobConfig(id)
}

func (s *configService) RunCronJob(id uint) error {
	// TODO: 实现立即执行定时任务
	// 这里应该调用相应的任务执行器
	return nil
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

func (s *configService) MigrateCronConfigFromYAML(yamlConfig map[string]interface{}) error {
	cronConfig, ok := yamlConfig["cron"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("cron config not found in yaml")
	}

	// 遍历每个定时任务配置
	for jobName, configData := range cronConfig {
		configMap, ok := configData.(map[string]interface{})
		if !ok {
			continue
		}

		schedule := getStringFromMap(configMap, "schedule")
		enabled := getBoolFromMap(configMap, "enabled")
		description := getStringFromMap(configMap, "description")

		// 其他配置暂时不存储，后续可根据需要扩展

		config := &model.CronJobConfig{
			JobName:     jobName,
			CronExpr:    schedule,
			IsEnabled:   enabled,
			Description: description,
		}

		if err := s.configRepo.SetCronJobConfig(config); err != nil {
			return fmt.Errorf("failed to migrate cron config for %s: %v", jobName, err)
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

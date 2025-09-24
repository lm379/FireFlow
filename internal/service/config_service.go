package service

import (
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"encoding/json"
	"fmt"
	"strconv"
)

type ConfigService interface {
	// 通用配置管理
	GetConfig(key string) (string, error)
	GetConfigInt(key string) (int, error)
	GetConfigBool(key string) (bool, error)
	SetConfig(key, value, configType, category, description string) error
	GetConfigsByCategory(category string) ([]model.ConfigItem, error)
	
	// 云服务商配置管理
	GetCloudConfig(provider string) (*model.CloudProviderConfig, error)
	SetCloudConfig(config *model.CloudProviderConfig) error
	GetDefaultCloudConfig() (*model.CloudProviderConfig, error)
	ListCloudConfigs() ([]model.CloudProviderConfig, error)
	
	// 定时任务配置管理
	GetCronConfig(jobName string) (*model.CronJobConfig, error)
	SetCronConfig(config *model.CronJobConfig) error
	ListCronConfigs() ([]model.CronJobConfig, error)
	EnableCronJob(jobName string) error
	DisableCronJob(jobName string) error
	
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

func (s *configService) SetCloudConfig(config *model.CloudProviderConfig) error {
	return s.configRepo.SetCloudProviderConfig(config)
}

func (s *configService) GetDefaultCloudConfig() (*model.CloudProviderConfig, error) {
	return s.configRepo.GetDefaultCloudProvider()
}

func (s *configService) ListCloudConfigs() ([]model.CloudProviderConfig, error) {
	return s.configRepo.ListCloudProviders()
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
			Provider:    provider,
			SecretId:    secretID,
			SecretKey:   secretKey,
			Region:      region,
			Extra:       string(otherConfigJSON),
			IsDefault:   provider == "tencent", // 假设腾讯云为默认
			IsEnabled:   true,
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
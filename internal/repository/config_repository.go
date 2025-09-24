package repository

import (
	"FireFlow/internal/model"
	"encoding/json"

	"gorm.io/gorm"
)

type ConfigRepository interface {
	// 通用配置项
	GetConfigValue(key string) (string, error)
	SetConfigValue(key, value, configType, category, description string) error
	GetConfigsByCategory(category string) ([]model.ConfigItem, error)

	// 云服务商配置
	GetCloudProviderConfig(provider string) (*model.CloudProviderConfig, error)
	GetCloudProviderConfigByID(id uint, config *model.CloudProviderConfig) error
	SetCloudProviderConfig(config *model.CloudProviderConfig) error
	GetDefaultCloudProvider() (*model.CloudProviderConfig, error)
	ListCloudProviders() ([]model.CloudProviderConfig, error)
	UpdateCloudProviderConfig(config *model.CloudProviderConfig) error
	DeleteCloudProviderConfig(id uint) error

	// 定时任务配置
	GetCronJobConfig(jobName string) (*model.CronJobConfig, error)
	SetCronJobConfig(config *model.CronJobConfig) error
	ListCronJobs() ([]model.CronJobConfig, error)
	UpdateCronJobStatus(jobName string, isEnabled bool) error
	UpdateCronJobConfig(config *model.CronJobConfig) error
	DeleteCronJobConfig(id uint) error
}

type configRepository struct {
	db *gorm.DB
}

func NewConfigRepository(db *gorm.DB) ConfigRepository {
	return &configRepository{db: db}
}

// 通用配置项方法
func (r *configRepository) GetConfigValue(key string) (string, error) {
	var config model.ConfigItem
	result := r.db.Where("config_key = ? AND is_enabled = ?", key, true).First(&config)
	if result.Error != nil {
		return "", result.Error
	}
	return config.ConfigValue, nil
}

func (r *configRepository) SetConfigValue(key, value, configType, category, description string) error {
	config := model.ConfigItem{
		ConfigKey:   key,
		ConfigValue: value,
		ConfigType:  configType,
		Category:    category,
		Description: description,
		IsEnabled:   true,
	}

	result := r.db.Where("config_key = ?", key).FirstOrCreate(&config)
	if result.Error != nil {
		return result.Error
	}

	// 更新值
	return r.db.Model(&config).Updates(map[string]interface{}{
		"config_value": value,
		"config_type":  configType,
		"category":     category,
		"description":  description,
	}).Error
}

func (r *configRepository) GetConfigsByCategory(category string) ([]model.ConfigItem, error) {
	var configs []model.ConfigItem
	result := r.db.Where("category = ? AND is_enabled = ?", category, true).Find(&configs)
	return configs, result.Error
}

// 云服务商配置方法
func (r *configRepository) GetCloudProviderConfig(provider string) (*model.CloudProviderConfig, error) {
	var config model.CloudProviderConfig
	result := r.db.Where("provider = ? AND is_enabled = ?", provider, true).First(&config)
	if result.Error != nil {
		return nil, result.Error
	}
	return &config, nil
}

func (r *configRepository) GetCloudProviderConfigByID(id uint, config *model.CloudProviderConfig) error {
	result := r.db.First(config, id)
	return result.Error
}

func (r *configRepository) SetCloudProviderConfig(config *model.CloudProviderConfig) error {
	// 如果设置为默认配置，先取消其他默认配置
	if config.IsDefault {
		r.db.Model(&model.CloudProviderConfig{}).Where("provider = ? AND id != ?", config.Provider, config.ID).Update("is_default", false)
	}

	// 直接创建新记录，不使用FirstOrCreate
	result := r.db.Create(config)
	return result.Error
}

func (r *configRepository) GetDefaultCloudProvider() (*model.CloudProviderConfig, error) {
	var config model.CloudProviderConfig
	result := r.db.Where("is_default = ? AND is_enabled = ?", true, true).First(&config)
	if result.Error != nil {
		return nil, result.Error
	}
	return &config, nil
}

func (r *configRepository) ListCloudProviders() ([]model.CloudProviderConfig, error) {
	var configs []model.CloudProviderConfig
	result := r.db.Where("is_enabled = ?", true).Find(&configs)
	return configs, result.Error
}

func (r *configRepository) UpdateCloudProviderConfig(config *model.CloudProviderConfig) error {
	// 如果设置为默认配置，先取消其他默认配置
	if config.IsDefault {
		r.db.Model(&model.CloudProviderConfig{}).Where("id != ?", config.ID).Update("is_default", false)
	}
	return r.db.Save(config).Error
}

func (r *configRepository) DeleteCloudProviderConfig(id uint) error {
	return r.db.Delete(&model.CloudProviderConfig{}, id).Error
}

// 定时任务配置方法
func (r *configRepository) GetCronJobConfig(jobName string) (*model.CronJobConfig, error) {
	var config model.CronJobConfig
	result := r.db.Where("job_name = ?", jobName).First(&config)
	if result.Error != nil {
		return nil, result.Error
	}
	return &config, nil
}

func (r *configRepository) SetCronJobConfig(config *model.CronJobConfig) error {
	result := r.db.Where("job_name = ?", config.JobName).FirstOrCreate(config)
	if result.Error != nil {
		return result.Error
	}

	// 更新配置
	return r.db.Model(config).Updates(config).Error
}

func (r *configRepository) ListCronJobs() ([]model.CronJobConfig, error) {
	var configs []model.CronJobConfig
	result := r.db.Find(&configs)
	return configs, result.Error
}

func (r *configRepository) UpdateCronJobStatus(jobName string, isEnabled bool) error {
	return r.db.Model(&model.CronJobConfig{}).Where("job_name = ?", jobName).Update("is_enabled", isEnabled).Error
}

func (r *configRepository) UpdateCronJobConfig(config *model.CronJobConfig) error {
	return r.db.Save(config).Error
}

func (r *configRepository) DeleteCronJobConfig(id uint) error {
	return r.db.Delete(&model.CronJobConfig{}, id).Error
}

// 辅助方法：将结构体转换为JSON字符串
func StructToJSON(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// 辅助方法：从JSON字符串解析到结构体
func JSONToStruct(jsonStr string, v interface{}) error {
	return json.Unmarshal([]byte(jsonStr), v)
}

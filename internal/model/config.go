package model

import (
	"gorm.io/gorm"
)

// ConfigItem 配置项模型
type ConfigItem struct {
	gorm.Model
	ConfigKey   string `gorm:"unique;not null;comment:配置键名" json:"config_key"`
	ConfigValue string `gorm:"type:text;comment:配置值" json:"config_value"`
	ConfigType  string `gorm:"type:varchar(50);default:'string';comment:配置类型(string,json,int,bool)" json:"config_type"`
	Category    string `gorm:"type:varchar(50);comment:配置分类(cloud,cron,system)" json:"category"`
	Description string `gorm:"type:varchar(255);comment:配置描述" json:"description"`
	IsEnabled   bool   `gorm:"default:true;comment:是否启用" json:"is_enabled"`
}

// CloudProviderConfig 云服务商配置模型
type CloudProviderConfig struct {
	gorm.Model
	Provider    string `gorm:"type:varchar(50);not null;comment:云服务商名称" json:"provider"`
	SecretId    string `gorm:"type:varchar(255);comment:访问密钥ID" json:"secret_id"`
	SecretKey   string `gorm:"type:varchar(255);comment:访问密钥Key" json:"secret_key"`
	Region      string `gorm:"type:varchar(100);comment:区域" json:"region"`
	Extra       string `gorm:"type:text;comment:额外配置(JSON格式)" json:"extra"`
	IsDefault   bool   `gorm:"default:false;comment:是否为默认配置" json:"is_default"`
	IsEnabled   bool   `gorm:"default:true;comment:是否启用" json:"is_enabled"`
	Description string `gorm:"type:varchar(255);comment:配置描述" json:"description"`
}

// CronJobConfig 定时任务配置模型
type CronJobConfig struct {
	gorm.Model
	JobName     string          `gorm:"type:varchar(100);not null;comment:任务名称" json:"job_name"`
	CronExpr    string          `gorm:"type:varchar(100);not null;comment:Cron表达式" json:"cron_expr"`
	Description string          `gorm:"type:varchar(255);comment:任务描述" json:"description"`
	IsEnabled   bool            `gorm:"default:true;comment:是否启用" json:"is_enabled"`
	LastRun     *gorm.DeletedAt `gorm:"comment:上次运行时间" json:"last_run"`
	NextRun     *gorm.DeletedAt `gorm:"comment:下次运行时间" json:"next_run"`
}

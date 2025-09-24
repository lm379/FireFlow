package model

import (
	gorm "gorm.io/gorm"
)

type FirewallRule struct {
	gorm.Model
	Provider      string              `gorm:"type:varchar(50);not null;comment:云厂商 (e.g., 'TencentCloud', 'Aliyun')" json:"provider"`
	CloudConfigID uint                `gorm:"comment:关联的云服务配置ID" json:"cloud_config_id"`
	InstanceID    string              `gorm:"type:varchar(100);not null;comment:服务器实例ID" json:"instance_id"`
	Port          string              `gorm:"type:varchar(20);not null;comment:需要开放的端口 (e.g., '80', '22')" json:"port"`
	Protocol      string              `gorm:"type:varchar(10);default:'TCP';comment:协议类型 (ICMP, TCP, UDP, ALL)" json:"protocol"`
	RuleID        string              `gorm:"type:varchar(100);comment:防火墙规则ID" json:"rule_id"`
	LastIP        string              `gorm:"type:varchar(50);comment:上一次更新的IP" json:"last_ip"`
	Enabled       bool                `gorm:"default:true;comment:是否启用" json:"enabled"`
	Remark        string              `gorm:"type:varchar(255);not null;comment:备注(必填)" json:"remark"`
	CloudConfig   CloudProviderConfig `gorm:"foreignKey:CloudConfigID" json:"cloud_config"`
}

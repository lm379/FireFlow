package model

import (
	gorm "gorm.io/gorm"
)

type FirewallRule struct {
	gorm.Model
	CloudConfigID uint                `gorm:"not null;index;check:cloud_config_id > 0;comment:关联的云服务配置ID" json:"cloud_config_id"`
	Port          string              `gorm:"type:varchar(20);not null;comment:需要开放的端口 (e.g., '80', '22')" json:"port"`
	Protocol      string              `gorm:"type:varchar(10);default:'TCP';comment:协议类型 (ICMP, TCP, UDP, ALL)" json:"protocol"`
	LastIP        string              `gorm:"type:varchar(50);comment:上一次更新的IP" json:"last_ip"`
	Enabled       bool                `gorm:"default:true;comment:是否启用" json:"enabled"`
	Remark        string              `gorm:"type:varchar(255);not null;comment:备注(必填)" json:"remark"`
	CloudConfig   CloudProviderConfig `gorm:"foreignKey:CloudConfigID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
}

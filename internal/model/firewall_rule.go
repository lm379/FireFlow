package model

import (
	gorm "gorm.io/gorm"
)

type FirewallRule struct {
	gorm.Model
	Provider   string `gorm:"type:varchar(50);not null;comment:云厂商 (e.g., 'TencentCloud', 'Aliyun')"`
	InstanceID string `gorm:"type:varchar(100);not null;comment:服务器实例ID"`
	Port       string `gorm:"type:varchar(20);not null;comment:需要开放的端口 (e.g., '80', '22')"`
	RuleID     string `gorm:"type:varchar(100);comment:防火墙规则ID"`
	LastIP     string `gorm:"type:varchar(50);comment:上一次更新的IP"`
	Enabled    bool   `gorm:"default:true;comment:是否启用"`
	Remark     string `gorm:"type:varchar(255)" json:"remark"`
}

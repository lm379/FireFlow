package repository

import (
	"FireFlow/internal/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

type FirewallRepository interface {
	GetAllEnabled() ([]model.FirewallRule, error)
	GetAll() ([]model.FirewallRule, error)
	GetByID(id uint) (*model.FirewallRule, error)
	Create(rule *model.FirewallRule) error
	Update(rule *model.FirewallRule) error
	UpdateIP(id uint, ip string) error
	Delete(id uint) error
}

type firewallRepo struct {
	db *gorm.DB
}

// NewFirewallRepo creates a new repository.
func NewFirewallRepo(db *gorm.DB) FirewallRepository {
	return &firewallRepo{db: db}
}

// retryOnBusy 在SQLite锁定时重试数据库操作
func (r *firewallRepo) retryOnBusy(fn func() error, maxRetries int) error {
	for i := 0; i < maxRetries; i++ {
		err := fn()
		if err == nil {
			return nil
		}

		// 检查是否是SQLite锁定错误
		if strings.Contains(err.Error(), "database is locked") ||
			strings.Contains(err.Error(), "SQLITE_BUSY") {
			if i < maxRetries-1 {
				// 等待一段时间后重试
				time.Sleep(time.Duration(50*(i+1)) * time.Millisecond)
				continue
			}
		}
		return err
	}
	return nil
}

func (r *firewallRepo) GetAllEnabled() ([]model.FirewallRule, error) {
	var rules []model.FirewallRule
	err := r.db.Where("enabled = ?", true).Find(&rules).Error
	return rules, err
}

func (r *firewallRepo) GetAll() ([]model.FirewallRule, error) {
	var rules []model.FirewallRule
	err := r.db.Find(&rules).Error
	return rules, err
}

func (r *firewallRepo) GetByID(id uint) (*model.FirewallRule, error) {
	var rule model.FirewallRule
	err := r.db.First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *firewallRepo) Create(rule *model.FirewallRule) error {
	return r.db.Create(rule).Error
}

func (r *firewallRepo) Update(rule *model.FirewallRule) error {
	// 使用Select方法明确指定要更新的字段，确保布尔字段也能正确更新，但保留created_at
	return r.retryOnBusy(func() error {
		return r.db.Model(&model.FirewallRule{}).Where("id = ?", rule.ID).
			Select("provider", "cloud_config_id", "instance_id", "port", "protocol", "rule_id", "last_ip", "enabled", "remark", "updated_at").
			Updates(rule).Error
	}, 3)
}

func (r *firewallRepo) UpdateIP(id uint, ip string) error {
	return r.retryOnBusy(func() error {
		return r.db.Model(&model.FirewallRule{}).Where("id = ?", id).Update("last_ip", ip).Error
	}, 3)
}

func (r *firewallRepo) Delete(id uint) error {
	return r.db.Unscoped().Delete(&model.FirewallRule{}, id).Error
}

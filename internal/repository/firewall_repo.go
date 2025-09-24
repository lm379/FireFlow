package repository

import (
	"FireFlow/internal/model"

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
	return r.db.Save(rule).Error
}

func (r *firewallRepo) UpdateIP(id uint, ip string) error {
	return r.db.Model(&model.FirewallRule{}).Where("id = ?", id).Update("last_ip", ip).Error
}

func (r *firewallRepo) Delete(id uint) error {
	return r.db.Unscoped().Delete(&model.FirewallRule{}, id).Error
}

package migration

import (
	"FireFlow/internal/model"
	"fmt"

	"gorm.io/gorm"
)

// Migrate upgrades the SQLite schema atomically, including legacy rule associations.
func Migrate(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var sequence int64
		hasRules := tx.Migrator().HasTable("firewall_rules")
		if hasRules {
			if err := tx.Raw("SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name = 'firewall_rules'), 0)").Scan(&sequence).Error; err != nil {
				return err
			}
		}
		if err := tx.AutoMigrate(&model.CloudProviderConfig{}, &model.ConfigItem{}, &model.AuthUser{}); err != nil {
			return err
		}
		if hasRules {
			if err := normalizeRules(tx); err != nil {
				return err
			}
		}
		if err := tx.AutoMigrate(&model.FirewallRule{}); err != nil {
			return err
		}
		return tx.Exec("UPDATE sqlite_sequence SET seq = MAX(seq, ?) WHERE name = 'firewall_rules'", sequence).Error
	})
}

type legacyRule struct {
	ID            uint
	CloudConfigID uint
	Provider      string
	InstanceID    string
	ProjectID     string
}

func normalizeRules(tx *gorm.DB) error {
	legacy := tx.Migrator().HasColumn("firewall_rules", "provider") ||
		tx.Migrator().HasColumn("firewall_rules", "instance_id") ||
		tx.Migrator().HasColumn("firewall_rules", "project_id")
	if !legacy {
		return nil
	}
	var rules []legacyRule
	if err := tx.Table("firewall_rules").Find(&rules).Error; err != nil {
		return err
	}
	var configs []model.CloudProviderConfig
	if err := tx.Unscoped().Find(&configs).Error; err != nil {
		return err
	}
	for _, rule := range rules {
		var matches []uint
		for _, config := range configs {
			if rule.CloudConfigID != 0 {
				if config.ID == rule.CloudConfigID {
					matches = append(matches, config.ID)
				}
			} else if !config.DeletedAt.Valid && config.Provider == rule.Provider && config.InstanceId == rule.InstanceID &&
				(rule.ProjectID == "" || config.ProjectID == rule.ProjectID) {
				matches = append(matches, config.ID)
			}
		}
		if len(matches) != 1 {
			return fmt.Errorf("firewall rule %d: cannot resolve cloud_config_id (found %d matching cloud configs); repair its association before restarting", rule.ID, len(matches))
		}
		if err := tx.Table("firewall_rules").Where("id = ?", rule.ID).UpdateColumn("cloud_config_id", matches[0]).Error; err != nil {
			return err
		}
	}
	// AutoMigrate retains removed columns; rebuilding also installs the actual foreign key.
	statements := []string{
		`CREATE TABLE firewall_rules_normalized (id INTEGER PRIMARY KEY AUTOINCREMENT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME, cloud_config_id INTEGER NOT NULL, port VARCHAR(20) NOT NULL, protocol VARCHAR(10) DEFAULT 'TCP', last_ip VARCHAR(50), enabled NUMERIC DEFAULT true, remark VARCHAR(255) NOT NULL, CONSTRAINT chk_firewall_rules_cloud_config_id CHECK (cloud_config_id > 0), CONSTRAINT fk_firewall_rules_cloud_config FOREIGN KEY (cloud_config_id) REFERENCES cloud_provider_configs(id) ON UPDATE CASCADE ON DELETE RESTRICT)`,
		`INSERT INTO firewall_rules_normalized (id, created_at, updated_at, deleted_at, cloud_config_id, port, protocol, last_ip, enabled, remark)
			SELECT id, created_at, updated_at, deleted_at, cloud_config_id, port, protocol, last_ip, enabled, remark FROM firewall_rules`,
		`DROP TABLE firewall_rules`,
		`ALTER TABLE firewall_rules_normalized RENAME TO firewall_rules`,
	}
	for _, statement := range statements {
		if err := tx.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

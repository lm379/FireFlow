package migration

import (
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

type oldRule struct {
	gorm.Model
	Provider      string `gorm:"not null"`
	CloudConfigID uint
	InstanceID    string `gorm:"not null"`
	ProjectID     string
	Port          string `gorm:"not null"`
	Protocol      string
	LastIP        string
	Enabled       bool
	Remark        string `gorm:"not null"`
}

func (oldRule) TableName() string { return "firewall_rules" }

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: filepath.Join(t.TempDir(), "migration.db")}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.Exec("PRAGMA foreign_keys=ON").Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestLegacyRulesMigrateAndReadLatestCloudConfig(t *testing.T) {
	db := testDB(t)
	if err := db.AutoMigrate(&model.CloudProviderConfig{}, &oldRule{}); err != nil {
		t.Fatal(err)
	}
	config := model.CloudProviderConfig{Provider: "Azure", InstanceId: "nsg", ProjectID: "project", IsEnabled: true}
	if err := db.Create(&config).Error; err != nil {
		t.Fatal(err)
	}
	timestamp := time.Now().UTC().Truncate(time.Second)
	rules := []oldRule{
		{Model: gorm.Model{ID: 4, CreatedAt: timestamp, UpdatedAt: timestamp}, Provider: "Azure", InstanceID: "nsg", ProjectID: "project", Port: "22", Protocol: "TCP", LastIP: "192.0.2.1", Enabled: false, Remark: "ssh"},
		// An explicit association takes precedence over old, stale copies.
		{Model: gorm.Model{ID: 8}, CloudConfigID: config.ID, Provider: "Aliyun", InstanceID: "stale", Port: "80", Protocol: "TCP", Enabled: true, Remark: "http"},
		{Model: gorm.Model{ID: 9, DeletedAt: gorm.DeletedAt{Time: timestamp, Valid: true}}, CloudConfigID: config.ID, Provider: "Azure", InstanceID: "nsg", Port: "443", Protocol: "TCP", Remark: "deleted"},
	}
	if err := db.Create(&rules).Error; err != nil {
		t.Fatal(err)
	}
	removed := oldRule{Model: gorm.Model{ID: 50}, CloudConfigID: config.ID, Provider: "Azure", InstanceID: "nsg", Port: "80", Remark: "removed"}
	if err := db.Create(&removed).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Unscoped().Delete(&removed).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := Migrate(db); err != nil {
			t.Fatal(err)
		}
	}
	for _, column := range []string{"provider", "instance_id", "project_id"} {
		if db.Migrator().HasColumn("firewall_rules", column) {
			t.Fatalf("duplicate column %s remains", column)
		}
	}
	repo := repository.NewFirewallRepo(db)
	rule, err := repo.GetByID(4)
	if err != nil {
		t.Fatal(err)
	}
	if rule.CloudConfigID != config.ID || rule.LastIP != "192.0.2.1" || rule.Enabled || rule.Remark != "ssh" || !rule.CreatedAt.Equal(timestamp) || !rule.UpdatedAt.Equal(timestamp) {
		t.Fatalf("rule data changed: %#v", rule)
	}
	var deleted model.FirewallRule
	if err := db.Unscoped().First(&deleted, 9).Error; err != nil || !deleted.DeletedAt.Valid {
		t.Fatalf("deleted rule lost: %#v, %v", deleted, err)
	}
	if err := db.Model(&config).Updates(map[string]interface{}{"provider": "HuaweiCloud", "instance_id": "new-nsg", "project_id": "new-project"}).Error; err != nil {
		t.Fatal(err)
	}
	all, err := repo.GetAll()
	if err != nil || len(all) != 2 {
		t.Fatalf("all rules: %v, %v", all, err)
	}
	enabled, err := repo.GetAllEnabled()
	if err != nil || len(enabled) != 1 {
		t.Fatalf("enabled rules: %v, %v", enabled, err)
	}
	rule, err = repo.GetByID(4)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range append(all, append(enabled, *rule)...) {
		if r.CloudConfig.Provider != "HuaweiCloud" || r.CloudConfig.InstanceId != "new-nsg" || r.CloudConfig.ProjectID != "new-project" {
			t.Fatalf("stale config: %#v", r)
		}
	}
	assertConstraints(t, db, config.ID)
	next := model.FirewallRule{CloudConfigID: config.ID, Port: "8080", Remark: "new"}
	if err := repo.Create(&next); err != nil || next.ID <= 50 {
		t.Fatalf("autoincrement sequence not preserved: %d, %v", next.ID, err)
	}
}

func assertConstraints(t *testing.T, db *gorm.DB, configID uint) {
	t.Helper()
	for _, id := range []uint{0, 9999} {
		if err := db.Create(&model.FirewallRule{CloudConfigID: id, Port: "22", Remark: "invalid"}).Error; err == nil {
			t.Fatalf("invalid cloud config %d accepted", id)
		}
	}
	if err := db.Unscoped().Delete(&model.CloudProviderConfig{}, configID).Error; err == nil {
		t.Fatal("referenced cloud config deleted")
	}
	var violations []struct{ Table string }
	if err := db.Raw("PRAGMA foreign_key_check").Scan(&violations).Error; err != nil || len(violations) != 0 {
		t.Fatalf("foreign key violations: %v, %v", violations, err)
	}
}

func TestFreshSchemaRequiresCloudReference(t *testing.T) {
	db := testDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	config := model.CloudProviderConfig{Provider: "Azure"}
	if err := db.Create(&config).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.FirewallRule{CloudConfigID: config.ID, Port: "22", Remark: "test"}).Error; err != nil {
		t.Fatal(err)
	}
	assertConstraints(t, db, config.ID)
}

func TestUnresolvableLegacyAssociationsRollBack(t *testing.T) {
	for _, scenario := range []string{"missing", "ambiguous", "dangling"} {
		t.Run(scenario, func(t *testing.T) {
			db := testDB(t)
			if err := db.AutoMigrate(&model.CloudProviderConfig{}, &oldRule{}); err != nil {
				t.Fatal(err)
			}
			config := model.CloudProviderConfig{Provider: "Azure", InstanceId: "nsg", ProjectID: "project"}
			if err := db.Create(&config).Error; err != nil {
				t.Fatal(err)
			}
			valid := oldRule{Model: gorm.Model{ID: 1}, Provider: "Azure", InstanceID: "nsg", ProjectID: "project", Port: "22", Remark: "valid"}
			invalid := oldRule{Model: gorm.Model{ID: 2}, Provider: "Azure", InstanceID: "unknown", Port: "80", Remark: "invalid"}
			if scenario == "ambiguous" {
				config.ID = 0
				config.InstanceId = "unknown"
				if err := db.Create(&config).Error; err != nil {
					t.Fatal(err)
				}
				config.ID = 0
				if err := db.Create(&config).Error; err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "dangling" {
				invalid.CloudConfigID = 9999
			}
			if err := db.Create(&[]oldRule{valid, invalid}).Error; err != nil {
				t.Fatal(err)
			}
			if err := Migrate(db); err == nil || !strings.Contains(err.Error(), "rule 2") {
				t.Fatalf("missing actionable error: %v", err)
			}
			if !db.Migrator().HasColumn("firewall_rules", "provider") {
				t.Fatal("old schema removed on failure")
			}
			var stored oldRule
			if err := db.First(&stored, 1).Error; err != nil || stored.CloudConfigID != 0 {
				t.Fatalf("partial migration persisted: %#v, %v", stored, err)
			}
		})
	}
}

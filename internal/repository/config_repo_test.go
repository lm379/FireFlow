package repository

import (
	"FireFlow/internal/model"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestAzureIdentityFieldsPersistOnUpdate(t *testing.T) {
	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: filepath.Join(t.TempDir(), "test.db")}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&model.CloudProviderConfig{}, &model.FirewallRule{}); err != nil {
		t.Fatal(err)
	}
	repo := NewConfigRepository(db)
	config := model.CloudProviderConfig{Provider: "Azure", TenantID: "old-tenant", SubscriptionID: "old-sub", IsEnabled: true}
	if err := repo.SetCloudProviderConfig(&config); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"new", ""} {
		config.TenantID, config.SubscriptionID = value, value
		if err := repo.UpdateCloudProviderConfig(&config); err != nil {
			t.Fatal(err)
		}
		var stored model.CloudProviderConfig
		if err := repo.GetCloudProviderConfigByID(config.ID, &stored); err != nil {
			t.Fatal(err)
		}
		if stored.TenantID != value || stored.SubscriptionID != value {
			t.Fatalf("identity fields not persisted: %#v", stored)
		}
	}
	ruleRepo := NewFirewallRepo(db)
	rule := model.FirewallRule{CloudConfigID: config.ID, Port: "22", Remark: "test"}
	if err := ruleRepo.Create(&rule); err != nil {
		t.Fatal(err)
	}
	config.ProjectID = "new-project"
	if err := repo.UpdateCloudProviderConfig(&config); err != nil {
		t.Fatal(err)
	}
	stored, err := ruleRepo.GetByID(rule.ID)
	if err != nil || stored.CloudConfig.ProjectID != "new-project" {
		t.Fatalf("rule did not read latest cloud config: %v, %v", stored, err)
	}
}

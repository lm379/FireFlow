package service

import (
	"FireFlow/internal/logger"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	logger.InfoLogger = logrus.New()
	logger.ErrorLogger = logrus.New()
	os.Exit(m.Run())
}

type ruleConfigStub struct {
	ConfigService
	configs         map[uint]*model.CloudProviderConfig
	url             string
	providerLookups int
}

func (s *ruleConfigStub) GetCloudConfigByID(id uint) (*model.CloudProviderConfig, error) {
	if config, ok := s.configs[id]; ok {
		return config, nil
	}
	return nil, errors.New("config not found")
}

func (s *ruleConfigStub) GetCloudConfig(provider string) (*model.CloudProviderConfig, error) {
	s.providerLookups++
	return &model.CloudProviderConfig{Provider: provider, IsEnabled: true}, nil
}

func (s *ruleConfigStub) GetConfig(key string) (string, error) { return s.url, nil }

type ruleRepoStub struct {
	repository.FirewallRepository
	rules map[uint]model.FirewallRule
}

func (r *ruleRepoStub) GetByID(id uint) (*model.FirewallRule, error) {
	rule, ok := r.rules[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &rule, nil
}

func (r *ruleRepoStub) Update(rule *model.FirewallRule) error {
	r.rules[rule.ID] = *rule
	return nil
}

func (r *ruleRepoStub) GetAllEnabled() ([]model.FirewallRule, error) {
	var rules []model.FirewallRule
	for _, rule := range r.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	return rules, nil
}

func testFirewallService(repo repository.FirewallRepository, config ConfigService) *FirewallService {
	return &FirewallService{repo: repo, configService: config, ruleLocks: make(map[uint]*sync.Mutex)}
}

func TestUpdateRuleRefreshesCloudAssociation(t *testing.T) {
	config := &ruleConfigStub{configs: map[uint]*model.CloudProviderConfig{
		2: {Provider: "Azure", InstanceId: "new-instance", ProjectID: "new-project", IsEnabled: true},
		3: {Provider: "TencentCloud", InstanceId: "lighthouse", IsEnabled: true},
	}}
	repo := &ruleRepoStub{rules: map[uint]model.FirewallRule{1: {Model: gorm.Model{ID: 1}, CloudConfigID: 1, CloudConfig: model.CloudProviderConfig{Provider: "Aliyun", InstanceId: "old-instance"}, LastIP: "192.0.2.1"}}}
	svc := testFirewallService(repo, config)
	rule, _ := repo.GetByID(1)
	rule.CloudConfigID = 2
	if err := svc.UpdateRule(rule); err != nil {
		t.Fatal(err)
	}
	stored := repo.rules[1]
	if stored.CloudConfig.Provider != "Azure" || stored.CloudConfig.InstanceId != "new-instance" || stored.CloudConfig.ProjectID != "new-project" || stored.LastIP != "" {
		t.Fatalf("stale association: %#v", stored)
	}
	rule.CloudConfigID = 3
	if err := svc.UpdateRule(rule); err != nil {
		t.Fatal(err)
	}
	if repo.rules[1].CloudConfig.ProjectID != "" {
		t.Fatal("old project retained")
	}
	rule.CloudConfigID = 99
	if err := svc.UpdateRule(rule); err == nil {
		t.Fatal("missing config accepted")
	}
	if repo.rules[1].CloudConfigID != 3 {
		t.Fatal("invalid association persisted")
	}
}

func TestDisabledAssociatedConfigCannotUseAnotherEnabledConfig(t *testing.T) {
	config := &ruleConfigStub{configs: map[uint]*model.CloudProviderConfig{2: {Provider: "Azure", IsEnabled: false}}}
	svc := testFirewallService(nil, config)
	rule := model.FirewallRule{CloudConfigID: 2, Enabled: true, Remark: "test"}
	if err := svc.checkCloudConfigEnabled(&rule); err == nil {
		t.Fatal("disabled config accepted")
	}
	if err := svc.processRule(rule, "192.0.2.1"); err == nil {
		t.Fatal("scheduled execution accepted disabled config")
	}
	repo := &ruleRepoStub{rules: map[uint]model.FirewallRule{1: rule}}
	svc.repo = repo
	if _, err := svc.ExecuteRule(1); err == nil {
		t.Fatal("manual execution accepted disabled config")
	}
	if config.providerLookups != 0 {
		t.Fatal("used provider fallback for an associated config")
	}
}

func TestSyncReportsFailuresAndFetchesIPOnce(t *testing.T) {
	requests := 0
	ipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Write([]byte("192.0.2.1"))
	}))
	defer ipServer.Close()
	config := &ruleConfigStub{url: ipServer.URL, configs: map[uint]*model.CloudProviderConfig{2: {Provider: "Azure", IsEnabled: false}}}
	repo := &ruleRepoStub{rules: map[uint]model.FirewallRule{
		1: {Model: gorm.Model{ID: 1}, CloudConfigID: 2, Enabled: true, Remark: "test"},
		2: {Model: gorm.Model{ID: 2}, CloudConfigID: 2, Enabled: true, Remark: "test"},
	}}
	svc := testFirewallService(repo, config)
	result, err := svc.SyncAllRules()
	if err == nil || result.UpdatedRules != 0 || result.FailedRules != 2 || result.CurrentIP != "192.0.2.1" || requests != 1 {
		t.Fatalf("incorrect sync result: %#v, err=%v, requests=%d", result, err, requests)
	}
	repo.rules = map[uint]model.FirewallRule{}
	result, err = svc.SyncAllRules()
	if err != nil || result.UpdatedRules != 0 || result.FailedRules != 0 {
		t.Fatalf("empty sync: %#v, %v", result, err)
	}
}

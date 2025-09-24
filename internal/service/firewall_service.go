package service

import (
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/utils"
	"FireFlow/pkg/cloud"
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type FirewallService struct {
	repo          repository.FirewallRepository
	tencentClient *cloud.TencentClient
}

func NewFirewallService(repo repository.FirewallRepository) *FirewallService {
	// Initialize Tencent Cloud client from config
	tencentConfig := cloud.TencentConfig{
		SecretId:  viper.GetString("cloud.tencent.secret_id"),
		SecretKey: viper.GetString("cloud.tencent.secret_key"),
		Region:    viper.GetString("cloud.tencent.region"),
	}

	var tencentClient *cloud.TencentClient
	if tencentConfig.SecretId != "" && tencentConfig.SecretKey != "" {
		var err error
		tencentClient, err = cloud.NewTencentClient(tencentConfig)
		if err != nil {
			log.Printf("Failed to initialize Tencent Cloud client: %v", err)
		} else {
			log.Println("Successfully initialized Tencent Cloud client")
		}
	}

	return &FirewallService{
		repo:          repo,
		tencentClient: tencentClient,
	}
}

// UpdateAllRules is the main logic executed by the cron job.
func (s *FirewallService) UpdateAllRules() {
	log.Println("Starting firewall update job...")

	// 1. Get current public IP
	currentIP, err := utils.GetPublicIP()
	if err != nil {
		log.Printf("Error getting public IP: %v", err)
		return
	}
	log.Printf("Current public IP is: %s", currentIP)

	// 2. Get all enabled rules from the database
	rules, err := s.repo.GetAllEnabled()
	if err != nil {
		log.Printf("Error getting firewall rules: %v", err)
		return
	}

	// 3. Iterate and update each rule if the IP has changed
	for _, rule := range rules {
		if rule.LastIP == currentIP {
			log.Printf("IP for rule %d (%s) is already up to date. Skipping.", rule.ID, rule.Remark)
			continue
		}

		log.Printf("IP changed for rule %d. Updating...", rule.ID)
		var updateErr error
		switch rule.Provider {
		case "TencentCloud":
			if s.tencentClient == nil {
				updateErr = fmt.Errorf("TencentCloud client not initialized")
			} else {
				updateErr = s.updateTencentFirewallRule(&rule, currentIP)
			}
		case "Aliyun":
			updateErr = fmt.Errorf("Aliyun provider not implemented yet")
		default:
			updateErr = fmt.Errorf("unsupported provider: %s", rule.Provider)
		}

		if updateErr != nil {
			log.Printf("Failed to update rule %d: %v", rule.ID, updateErr)
		} else {
			// 4. If update succeeds, save the new IP to the database
			if err := s.repo.UpdateIP(rule.ID, currentIP); err != nil {
				log.Printf("Failed to update IP in database for rule %d: %v", rule.ID, err)
			} else {
				log.Printf("Successfully updated rule %d to IP %s", rule.ID, currentIP)
			}
		}
	}
	log.Println("Firewall update job finished.")
}

// updateTencentFirewallRule updates a firewall rule in Tencent Cloud
func (s *FirewallService) updateTencentFirewallRule(rule *model.FirewallRule, newIP string) error {
	// 构建新的CIDR块（假设是单个IP的/32）
	newCidrBlock := fmt.Sprintf("%s/32", newIP)

	// 使用我们的TencentClient来更新防火墙规则
	return s.tencentClient.UpdateFirewallRule(rule.InstanceID, rule.RuleID, newCidrBlock)
}

// The following methods are for the API
func (s *FirewallService) GetAllRules() ([]model.FirewallRule, error) {
	return s.repo.GetAll()
}

func (s *FirewallService) CreateRule(rule *model.FirewallRule) error {
	return s.repo.Create(rule)
}

func (s *FirewallService) DeleteRule(id uint) error {
	return s.repo.Delete(id)
}

func (s *FirewallService) UpdateRule(rule *model.FirewallRule) error {
	return s.repo.Update(rule)
}

func (s *FirewallService) ExecuteRule(id uint) error {
	// 获取规则
	rule, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get rule: %v", err)
	}

	// 获取当前公网IP
	currentIP, err := utils.GetPublicIP()
	if err != nil {
		return fmt.Errorf("failed to get current IP: %v", err)
	}

	// 执行规则更新
	switch rule.Provider {
	case "TencentCloud":
		return s.updateTencentFirewallRule(rule, currentIP)
	case "Aliyun":
		// TODO: 实现阿里云规则更新
		return fmt.Errorf("Aliyun provider not implemented yet")
	default:
		return fmt.Errorf("unsupported provider: %s", rule.Provider)
	}
}

// CreateTencentFirewallRule creates a new firewall rule in Tencent Cloud and saves it to database
func (s *FirewallService) CreateTencentFirewallRule(instanceID, port, cidrBlock, protocol, description string) error {
	if s.tencentClient == nil {
		return fmt.Errorf("TencentCloud client not initialized")
	}

	// 创建防火墙规则规格
	ruleSpec := &cloud.FirewallRuleSpec{
		Port:        port,
		Protocol:    protocol,
		CidrBlock:   cidrBlock,
		Action:      "ACCEPT",
		Description: description,
	}

	// 在腾讯云创建规则
	result, err := s.tencentClient.CreateFirewallRule(instanceID, ruleSpec)
	if err != nil {
		return fmt.Errorf("failed to create firewall rule in Tencent Cloud: %v", err)
	}

	// 保存到数据库
	rule := &model.FirewallRule{
		Provider:   "TencentCloud",
		InstanceID: instanceID,
		Port:       port,
		RuleID:     result.RuleID,
		LastIP:     cidrBlock,
		Enabled:    true,
		Remark:     description,
	}

	return s.repo.Create(rule)
}

// SyncTencentFirewallRules synchronizes firewall rules from Tencent Cloud with local database
func (s *FirewallService) SyncTencentFirewallRules(instanceID string) error {
	if s.tencentClient == nil {
		return fmt.Errorf("TencentCloud client not initialized")
	}

	// 从腾讯云获取防火墙规则
	rules, err := s.tencentClient.ListFirewallRules(instanceID)
	if err != nil {
		return fmt.Errorf("failed to list firewall rules from Tencent Cloud: %v", err)
	}

	log.Printf("Found %d firewall rules for instance %s", len(rules), instanceID)

	// 这里可以添加同步逻辑，比如：
	// 1. 比较云端和本地的规则
	// 2. 添加云端存在但本地不存在的规则
	// 3. 标记本地存在但云端不存在的规则为失效

	for _, rule := range rules {
		log.Printf("Rule: %s, Port: %s, Protocol: %s, CIDR: %s",
			rule.RuleID, rule.Port, rule.Protocol, rule.CidrBlock)
	}

	return nil
}

// GetInstanceInfo gets information about a cloud instance
func (s *FirewallService) GetInstanceInfo(instanceID string) (*cloud.InstanceInfo, error) {
	if s.tencentClient == nil {
		return nil, fmt.Errorf("TencentCloud client not initialized")
	}

	return s.tencentClient.GetInstance(instanceID)
}

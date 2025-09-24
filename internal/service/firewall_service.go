package service

import (
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/utils"
	"FireFlow/pkg/cloud"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

type FirewallService struct {
	repo          repository.FirewallRepository
	tencentClient *cloud.TencentClient
	configService ConfigService
}

func NewFirewallService(repo repository.FirewallRepository, configService ConfigService) *FirewallService {
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
		configService: configService,
	}
}

// UpdateAllRules is the main logic executed by the cron job.
func (s *FirewallService) UpdateAllRules() {
	log.Println("Starting firewall update job...")

	// 1. Get current public IP using configured URL
	var currentIP string
	var err error

	if s.configService != nil {
		// 获取配置的IP查询URL
		ipFetchURL, configErr := s.configService.GetConfig("ip_fetch_url")
		if configErr != nil || ipFetchURL == "" {
			ipFetchURL = "https://4.ipw.cn" // 默认URL
		}
		currentIP, err = utils.GetPublicIPWithURL(ipFetchURL)
	} else {
		// 降级到默认方法
		currentIP, err = utils.GetPublicIP()
	}

	if err != nil {
		log.Printf("Error getting public IP: %v", err)
		return
	}
	log.Printf("Current public IP is: %s", currentIP)

	// 检查IP合法性（只允许IPv4，禁止IPv6、JSON、报错信息、内容过长等）
	if len(currentIP) > 40 || strings.Contains(currentIP, ":") || strings.ContainsAny(currentIP, "[{") || strings.Contains(strings.ToLower(currentIP), "error") || strings.Contains(strings.ToLower(currentIP), "html") {
		log.Printf("获取到的IP地址不合法，未触发规则更新。")
		return
	}
	// 严格正则校验IPv4
	ipv4Pattern := `^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`
	matched, _ := regexp.MatchString(ipv4Pattern, currentIP)
	if !matched {
		log.Printf("获取到的IP地址不是合法IPv4，未触发规则更新。")
		return
	}

	// 2. Get all enabled rules from the database
	rules, err := s.repo.GetAllEnabled()
	if err != nil {
		log.Printf("Error getting firewall rules: %v", err)
		return
	}

	// 3. Iterate and update each rule (无论IP是否变化都要执行)
	for _, rule := range rules {
		// 只处理有备注的规则
		if rule.Remark == "" {
			log.Printf("Skipping rule %d: no remark provided", rule.ID)
			continue
		}

		log.Printf("Processing rule %d (%s) - Current IP: %s, Last IP: %s", rule.ID, rule.Remark, currentIP, rule.LastIP)

		var updateErr error
		switch rule.Provider {
		case "TencentCloud":
			// 使用getTencentClient方法获取客户端，而不是检查全局客户端
			updateErr = s.updateTencentFirewallRule(&rule, currentIP)
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

// createAndUpdateTencentFirewallRule 创建新的防火墙规则并更新数据库
func (s *FirewallService) createAndUpdateTencentFirewallRule(rule *model.FirewallRule, currentIP string) error {
	// 获取腾讯云客户端
	tencentClient, err := s.getTencentClient(rule.CloudConfigID)
	if err != nil {
		return fmt.Errorf("failed to get Tencent Cloud client: %v", err)
	}

	// 构建CIDR块
	cidrBlock := fmt.Sprintf("%s/32", currentIP)

	// 构建防火墙规则规格
	ruleSpec := &cloud.FirewallRuleSpec{
		Protocol:    rule.Protocol,
		Port:        rule.Port,
		CidrBlock:   cidrBlock,
		Action:      "ACCEPT", // 默认允许
		Description: rule.Remark,
	}

	// 在云服务上创建防火墙规则
	result, err := tencentClient.CreateFirewallRule(rule.InstanceID, ruleSpec)
	if err != nil {
		return fmt.Errorf("failed to create firewall rule: %v", err)
	}

	// 更新数据库中的规则信息
	rule.RuleID = result.RuleID
	rule.LastIP = currentIP
	err = s.repo.Update(rule)
	if err != nil {
		log.Printf("Warning: Rule created in cloud but failed to update database: %v", err)
	}

	log.Printf("Successfully created and executed firewall rule %s for instance %s", result.RuleID, rule.InstanceID)
	return nil
}

// updateTencentFirewallRule updates a firewall rule in Tencent Cloud
func (s *FirewallService) updateTencentFirewallRule(rule *model.FirewallRule, newIP string) error {
	// 获取腾讯云客户端
	tencentClient, err := s.getTencentClient(rule.CloudConfigID)
	if err != nil {
		return fmt.Errorf("failed to get Tencent Cloud client: %v", err)
	}

	// 构建规则规格，用于匹配云端规则
	ruleSpec := &cloud.FirewallRuleSpec{
		Protocol:    rule.Protocol,
		Port:        rule.Port,
		CidrBlock:   fmt.Sprintf("%s/32", newIP), // 新的CIDR
		Action:      "ACCEPT",                    // 默认为ACCEPT
		Description: rule.Remark,                 // 使用备注作为描述
	}

	// 使用规则规格来更新规则
	updatedRule, err := tencentClient.UpdateFirewallRule(rule.InstanceID, rule.RuleID, ruleSpec, newIP)
	if err != nil {
		// 如果更新失败且错误信息表明规则不存在，尝试重新创建规则
		if strings.Contains(err.Error(), "not found") {
			log.Printf("Rule not found in cloud, attempting to recreate it")
			return s.createAndUpdateTencentFirewallRule(rule, newIP)
		}
		return err
	}

	// 更新数据库中的规则信息
	if updatedRule != nil {
		rule.RuleID = updatedRule.RuleID
		rule.LastIP = newIP
		if err := s.repo.Update(rule); err != nil {
			log.Printf("Warning: Failed to update rule in database: %v", err)
		}
	}

	return nil
}

// getTencentClient 根据CloudConfigID获取腾讯云客户端
func (s *FirewallService) getTencentClient(cloudConfigID uint) (*cloud.TencentClient, error) {
	// 如果有全局客户端且CloudConfigID为0，使用全局客户端
	if cloudConfigID == 0 && s.tencentClient != nil {
		return s.tencentClient, nil
	}

	// 根据CloudConfigID获取云服务配置
	if s.configService == nil {
		return nil, fmt.Errorf("config service not available")
	}

	cloudConfig, err := s.configService.GetCloudConfigByID(cloudConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cloud config: %v", err)
	}

	// 构建腾讯云配置
	tencentConfig := cloud.TencentConfig{
		SecretId:   cloudConfig.SecretId,
		SecretKey:  cloudConfig.SecretKey,
		Region:     cloudConfig.Region,
		InstanceId: cloudConfig.InstanceId,
	}

	// 创建腾讯云客户端
	return cloud.NewTencentClient(tencentConfig)
}

// The following methods are for the API
func (s *FirewallService) GetAllRules() ([]model.FirewallRule, error) {
	return s.repo.GetAll()
}

// GetEnabledRulesCount 获取启用规则的数量
func (s *FirewallService) GetEnabledRulesCount() (int, error) {
	enabledRules, err := s.repo.GetAllEnabled()
	if err != nil {
		return 0, err
	}
	return len(enabledRules), nil
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
	var currentIP string
	if s.configService != nil {
		// 获取配置的IP查询URL
		ipFetchURL, configErr := s.configService.GetConfig("ip_fetch_url")
		if configErr != nil || ipFetchURL == "" {
			ipFetchURL = "https://4.ipw.cn" // 默认URL
		}
		currentIP, err = utils.GetPublicIPWithURL(ipFetchURL)
	} else {
		// 降级到默认方法
		currentIP, err = utils.GetPublicIP()
	}
	if err != nil {
		return fmt.Errorf("failed to get current IP: %v", err)
	}

	// 执行规则更新
	switch rule.Provider {
	case "TencentCloud":
		// 如果规则ID为空，需要先创建规则
		if rule.RuleID == "" {
			return s.createAndUpdateTencentFirewallRule(rule, currentIP)
		} else {
			return s.updateTencentFirewallRule(rule, currentIP)
		}
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

package service

import (
	"FireFlow/internal/core"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/utils"
	"FireFlow/pkg/cloud"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type FirewallService struct {
	repo          repository.FirewallRepository
	tencentClient *cloud.TencentClient
	aliyunClient  *cloud.AliyunClient
	configService ConfigService
	// 添加互斥锁防止并发更新
	updateMutex sync.RWMutex
	ruleLocks   map[uint]*sync.Mutex // 每个规则的独立锁
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

	// Initialize Aliyun ECS client from config
	aliyunConfig := cloud.AliyunConfig{
		AccessKeyID:      viper.GetString("cloud.aliyun.access_key_id"),
		AccessKeySecret:  viper.GetString("cloud.aliyun.access_key_secret"),
		RegionID:         viper.GetString("cloud.aliyun.region_id"),
		SecurityGroupIds: viper.GetString("cloud.aliyun.security_group_ids"),
	}

	var aliyunClient *cloud.AliyunClient
	if aliyunConfig.AccessKeyID != "" && aliyunConfig.AccessKeySecret != "" {
		var err error
		aliyunClient, err = cloud.NewAliyunClient(aliyunConfig)
		if err != nil {
			log.Printf("Failed to initialize Aliyun ECS client: %v", err)
		} else {
			log.Println("Successfully initialized Aliyun ECS client")
		}
	}

	return &FirewallService{
		repo:          repo,
		tencentClient: tencentClient,
		aliyunClient:  aliyunClient,
		configService: configService,
		ruleLocks:     make(map[uint]*sync.Mutex),
	}
}

// getRuleLock 获取指定规则的锁（如果不存在则创建）
func (s *FirewallService) getRuleLock(ruleID uint) *sync.Mutex {
	s.updateMutex.RLock()
	if lock, exists := s.ruleLocks[ruleID]; exists {
		s.updateMutex.RUnlock()
		return lock
	}
	s.updateMutex.RUnlock()

	s.updateMutex.Lock()
	defer s.updateMutex.Unlock()

	// 双重检查
	if lock, exists := s.ruleLocks[ruleID]; exists {
		return lock
	}

	s.ruleLocks[ruleID] = &sync.Mutex{}
	return s.ruleLocks[ruleID]
}

// UpdateAllRules is the main logic executed by the cron job.
func (s *FirewallService) UpdateAllRules() {
	log.Println("Starting firewall update job...")

	// 获取并验证当前公网IP
	currentIP, err := utils.GetValidatedPublicIP(s.configService)
	if err != nil {
		log.Printf("Error getting/validating public IP: %v", err)
		return
	}
	log.Printf("Current public IP is: %s", currentIP)

	// 2. Get all enabled rules from the database
	rules, err := s.repo.GetAllEnabled()
	if err != nil {
		log.Printf("Error getting firewall rules: %v", err)
		return
	}

	// 3. Iterate and update each rule (云服务提供商会检查IP是否一致，避免不必要的更新)
	for _, rule := range rules {
		// 获取该规则的独立锁
		ruleLock := s.getRuleLock(rule.ID)
		ruleLock.Lock()

		s.processRule(rule, currentIP)

		ruleLock.Unlock()
	}
	log.Println("Firewall update job finished.")
}

// processRule 处理单个规则的更新逻辑
func (s *FirewallService) processRule(rule model.FirewallRule, currentIP string) {
	// 只处理有备注的规则
	if rule.Remark == "" {
		log.Printf("Skipping rule %d: no remark provided", rule.ID)
		return
	}

	// 检查规则是否启用
	if !rule.Enabled {
		log.Printf("Skipping rule %d: rule is disabled", rule.ID)
		return
	}

	// 检查对应的云服务配置是否启用
	if err := s.checkCloudConfigEnabled(rule.Provider); err != nil {
		log.Printf("Skipping rule %d: %v", rule.ID, err)
		return
	}

	log.Printf("Processing rule %d (%s) - Current IP: %s, Last IP: %s", rule.ID, rule.Remark, currentIP, rule.LastIP)

	var updateErr error
	switch rule.Provider {
	case "TencentCloud":
		// 使用getTencentClient方法获取客户端，而不是检查全局客户端
		updateErr = s.updateTencentFirewallRule(&rule, currentIP)
	case "Aliyun":
		updateErr = s.updateAliyunFirewallRule(&rule, currentIP)
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

// checkCloudConfigEnabled 检查指定提供商的云服务配置是否启用
func (s *FirewallService) checkCloudConfigEnabled(provider string) error {
	// 获取该提供商的云服务配置
	config, err := s.configService.GetCloudConfig(provider)
	if err != nil {
		return fmt.Errorf("cloud config for provider %s not found", provider)
	}

	// 检查配置是否启用
	if !config.IsEnabled {
		return fmt.Errorf("cloud config for provider %s is disabled", provider)
	}

	return nil
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

func (s *FirewallService) GetRuleByID(id uint) (*model.FirewallRule, error) {
	return s.repo.GetByID(id)
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

	// 检查规则是否启用
	if !rule.Enabled {
		return fmt.Errorf("rule %d is disabled, skipping execution", id)
	}

	// 检查对应的云服务配置是否启用
	if err := s.checkCloudConfigEnabled(rule.Provider); err != nil {
		return err
	}

	// 获取并验证当前公网IP
	currentIP, err := utils.GetValidatedPublicIP(s.configService)
	if err != nil {
		return fmt.Errorf("failed to get/validate current IP: %v", err)
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
		// 如果规则ID为空，需要先创建规则
		if rule.RuleID == "" {
			return s.createAndUpdateAliyunFirewallRule(rule, currentIP)
		} else {
			return s.updateAliyunFirewallRule(rule, currentIP)
		}
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

// ============= 阿里云相关方法 =============

// updateAliyunFirewallRule 更新阿里云防火墙规则
func (s *FirewallService) updateAliyunFirewallRule(rule *model.FirewallRule, newIP string) error {
	client, err := s.getAliyunClient(rule.CloudConfigID)
	if err != nil {
		return fmt.Errorf("failed to get Aliyun client: %v", err)
	}

	// 创建防火墙规则规格
	ruleSpec := &cloud.FirewallRuleSpec{
		Port:        rule.Port,
		Protocol:    rule.Protocol,
		CidrBlock:   fmt.Sprintf("%s/32", newIP),
		Action:      "ACCEPT",
		Description: rule.Remark,
	}

	// 更新规则
	result, err := client.UpdateFirewallRule(rule.InstanceID, rule.RuleID, ruleSpec, newIP)
	if err != nil {
		// 如果更新失败，可能是规则已被手动删除，尝试重新创建
		errStr := err.Error()
		if strings.Contains(errStr, "InvalidSecurityGroupRule.NotFound") ||
			strings.Contains(errStr, "rule does not exist") ||
			strings.Contains(errStr, "not found") {
			log.Printf("Rule %s not found in cloud, attempting to recreate", rule.RuleID)

			// 尝试重新创建规则
			result, createErr := client.CreateFirewallRule(rule.InstanceID, ruleSpec)
			if createErr != nil {
				return fmt.Errorf("failed to recreate Aliyun firewall rule after rule not found: %v", createErr)
			}

			// 更新数据库中的规则ID
			rule.RuleID = result.RuleID
			if err := s.repo.Update(rule); err != nil {
				log.Printf("Failed to update rule ID in database: %v", err)
			}

			log.Printf("Successfully recreated Aliyun firewall rule %s for instance %s: %s",
				rule.RuleID, rule.InstanceID, newIP)
			return nil
		}
		return fmt.Errorf("failed to update Aliyun firewall rule: %v", err)
	}

	// 更新数据库中的规则ID（如果有变化）
	if result.RuleID != rule.RuleID {
		rule.RuleID = result.RuleID
		if err := s.repo.Update(rule); err != nil {
			log.Printf("Failed to update rule ID in database: %v", err)
		}
	}

	log.Printf("Successfully updated Aliyun firewall rule %s for instance %s: %s -> %s",
		rule.RuleID, rule.InstanceID, rule.LastIP, newIP)

	return nil
}

// createAndUpdateAliyunFirewallRule 创建并更新阿里云防火墙规则
func (s *FirewallService) createAndUpdateAliyunFirewallRule(rule *model.FirewallRule, currentIP string) error {
	client, err := s.getAliyunClient(rule.CloudConfigID)
	if err != nil {
		return fmt.Errorf("failed to get Aliyun client: %v", err)
	}

	// 创建防火墙规则规格
	ruleSpec := &cloud.FirewallRuleSpec{
		Port:        rule.Port,
		Protocol:    rule.Protocol,
		CidrBlock:   fmt.Sprintf("%s/32", currentIP),
		Action:      "ACCEPT",
		Description: rule.Remark,
	}

	// 创建规则
	result, err := client.CreateFirewallRule(rule.InstanceID, ruleSpec)
	if err != nil {
		return fmt.Errorf("failed to create Aliyun firewall rule: %v", err)
	}

	// 更新数据库中的规则ID和IP
	rule.RuleID = result.RuleID
	if err := s.repo.Update(rule); err != nil {
		log.Printf("Failed to update rule ID in database: %v", err)
	}

	if err := s.repo.UpdateIP(rule.ID, currentIP); err != nil {
		log.Printf("Failed to update IP in database: %v", err)
	}

	log.Printf("Successfully created Aliyun firewall rule %s for instance %s with IP %s",
		result.RuleID, rule.InstanceID, currentIP)

	return nil
}

// CreateAliyunFirewallRule 创建新的阿里云防火墙规则并保存到数据库
func (s *FirewallService) CreateAliyunFirewallRule(instanceID, port, cidrBlock, protocol, description string) error {
	if s.aliyunClient == nil {
		return fmt.Errorf("Aliyun client not initialized")
	}

	// 创建防火墙规则规格
	ruleSpec := &cloud.FirewallRuleSpec{
		Port:        port,
		Protocol:    protocol,
		CidrBlock:   cidrBlock,
		Action:      "ACCEPT",
		Description: description,
	}

	// 创建规则
	result, err := s.aliyunClient.CreateFirewallRule(instanceID, ruleSpec)
	if err != nil {
		return fmt.Errorf("failed to create Aliyun firewall rule: %v", err)
	}

	// 保存到数据库
	rule := &model.FirewallRule{
		RuleID:     result.RuleID,
		Remark:     description,
		InstanceID: instanceID,
		Port:       port,
		Protocol:   protocol,
		Provider:   "Aliyun",
		Enabled:    true,
	}

	if err := s.repo.Create(rule); err != nil {
		// 如果数据库保存失败，尝试删除云端规则
		if deleteErr := s.aliyunClient.DeleteFirewallRule(instanceID, result.RuleID); deleteErr != nil {
			log.Printf("Failed to rollback Aliyun firewall rule creation: %v", deleteErr)
		}
		return fmt.Errorf("failed to save rule to database: %v", err)
	}

	log.Printf("Successfully created and saved Aliyun firewall rule: %s", result.RuleID)
	return nil
}

// getAliyunClient 获取阿里云客户端
func (s *FirewallService) getAliyunClient(cloudConfigID uint) (*cloud.AliyunClient, error) {
	// 如果有全局客户端，直接使用
	if s.aliyunClient != nil {
		return s.aliyunClient, nil
	}

	// 否则根据配置ID创建客户端
	if s.configService == nil {
		return nil, fmt.Errorf("config service not available")
	}

	config, err := s.configService.GetCloudConfigByID(cloudConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cloud config: %v", err)
	}

	if config.Provider != "Aliyun" {
		return nil, fmt.Errorf("invalid provider: expected Aliyun, got %s", config.Provider)
	}

	// 解析阿里云配置
	aliyunConfig := cloud.AliyunConfig{
		AccessKeyID:      config.SecretId,
		AccessKeySecret:  config.SecretKey,
		RegionID:         config.Region,
		SecurityGroupIds: config.InstanceId, // 在云配置中，实例ID字段用于存储安全组ID
	}

	return cloud.NewAliyunClient(aliyunConfig)
}

// GetAliyunInstanceInfo 获取阿里云实例信息
func (s *FirewallService) GetAliyunInstanceInfo(instanceID string, cloudConfigID uint) (*cloud.InstanceInfo, error) {
	client, err := s.getAliyunClient(cloudConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Aliyun client: %v", err)
	}

	return client.GetInstance(instanceID)
}

// UpdateSingleRule 更新单个规则的IP
func (s *FirewallService) UpdateSingleRule(ruleID uint) error {
	// 获取规则的独立锁
	ruleLock := s.getRuleLock(ruleID)
	ruleLock.Lock()
	defer ruleLock.Unlock()

	log.Printf("Starting update for rule %d...", ruleID)

	// 获取规则信息
	rule, err := s.repo.GetByID(ruleID)
	if err != nil {
		log.Printf("Error getting rule %d: %v", ruleID, err)
		return err
	}

	if !rule.Enabled {
		log.Printf("Rule %d is disabled, skipping update", ruleID)
		return nil
	}

	// 获取并验证当前公网IP
	currentIP, err := utils.GetValidatedPublicIP(s.configService)
	if err != nil {
		log.Printf("Error getting/validating public IP for rule %d: %v", ruleID, err)
		return err
	}

	log.Printf("Current public IP for rule %d: %s", ruleID, currentIP)

	// 使用共用的处理逻辑
	s.processRule(*rule, currentIP)

	log.Printf("Finished update for rule %d", ruleID)
	return nil
}

// StartRuleUpdateJobs 为所有启用的规则启动定时任务
func (s *FirewallService) StartRuleUpdateJobs(cronManager interface{}) error {
	// 类型断言
	cm, ok := cronManager.(*core.CronManager)
	if !ok {
		return fmt.Errorf("invalid cronManager type")
	}

	// 获取间隔时间配置
	intervalStr, err := s.configService.GetConfig("ip_check_interval")
	if err != nil || intervalStr == "" {
		intervalStr = "5" // 默认5分钟
	}

	var intervalMinutes int
	if _, err := fmt.Sscanf(intervalStr, "%d", &intervalMinutes); err != nil {
		intervalMinutes = 5 // 默认5分钟
	}

	// 获取所有启用的规则
	rules, err := s.repo.GetAllEnabled()
	if err != nil {
		return fmt.Errorf("failed to get enabled rules: %v", err)
	}

	// 为每个规则创建定时任务
	for _, rule := range rules {
		ruleID := rule.ID
		err := cm.StartRuleUpdateJob(ruleID, intervalMinutes, func() {
			s.UpdateSingleRule(ruleID)
		})
		if err != nil {
			log.Printf("Failed to start update job for rule %d: %v", ruleID, err)
		}
	}

	log.Printf("Started update jobs for %d enabled rules", len(rules))
	return nil
}

// StartSingleRuleUpdateJob 为单个规则启动定时任务
func (s *FirewallService) StartSingleRuleUpdateJob(ruleID uint, cronManager interface{}) error {
	// 类型断言
	cm, ok := cronManager.(*core.CronManager)
	if !ok {
		return fmt.Errorf("invalid cronManager type")
	}

	// 获取间隔时间配置
	intervalStr, err := s.configService.GetConfig("ip_check_interval")
	if err != nil || intervalStr == "" {
		intervalStr = "5" // 默认5分钟
	}

	var intervalMinutes int
	if _, err := fmt.Sscanf(intervalStr, "%d", &intervalMinutes); err != nil {
		intervalMinutes = 5 // 默认5分钟
	}

	// 创建定时任务
	return cm.StartRuleUpdateJob(ruleID, intervalMinutes, func() {
		s.UpdateSingleRule(ruleID)
	})
}

// StopSingleRuleUpdateJob 停止单个规则的定时任务
func (s *FirewallService) StopSingleRuleUpdateJob(ruleID uint, cronManager interface{}) {
	// 类型断言
	cm, ok := cronManager.(*core.CronManager)
	if !ok {
		log.Printf("Invalid cronManager type for stopping rule %d job", ruleID)
		return
	}

	cm.StopRuleUpdateJob(ruleID)
}

// SyncCloudRules 同步云端规则，清理本地数据库中已经不存在于云端的规则
func (s *FirewallService) SyncCloudRules() error {
	// 获取所有本地规则
	rules, err := s.repo.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get local rules: %v", err)
	}

	var deletedCount int
	for _, rule := range rules {
		var shouldDelete bool
		switch rule.Provider {
		case "TencentCloud":
			shouldDelete, err = s.syncTencentRule(&rule)
		case "Aliyun":
			shouldDelete, err = s.syncAliyunRule(&rule)
		default:
			continue
		}

		if err != nil {
			log.Printf("Failed to sync rule %d (%s): %v", rule.ID, rule.RuleID, err)
			continue
		}

		if shouldDelete {
			deletedCount++
		}
	}

	if deletedCount > 0 {
		log.Printf("Sync completed: deleted %d rules that no longer exist in cloud", deletedCount)
	} else {
		log.Printf("Sync completed: all local rules exist in cloud")
	}

	return nil
}

// syncAliyunRule 同步单个阿里云规则，返回是否应该删除该规则
func (s *FirewallService) syncAliyunRule(rule *model.FirewallRule) (bool, error) {
	client, err := s.getAliyunClient(rule.CloudConfigID)
	if err != nil {
		return false, fmt.Errorf("failed to get Aliyun client: %v", err)
	}

	// 获取云端规则列表
	cloudRules, err := client.ListFirewallRules(rule.InstanceID)
	if err != nil {
		return false, fmt.Errorf("failed to list cloud rules: %v", err)
	}

	// 检查本地规则是否存在于云端
	found := false
	for _, cloudRule := range cloudRules {
		if cloudRule.RuleID == rule.RuleID {
			found = true
			break
		}
	}

	// 如果云端不存在该规则，从本地数据库删除
	if !found {
		log.Printf("Rule %s (ID: %d) not found in cloud, deleting from local database", rule.RuleID, rule.ID)
		if err := s.repo.Delete(rule.ID); err != nil {
			return false, fmt.Errorf("failed to delete rule from database: %v", err)
		}
		return true, nil
	}

	return false, nil
}

// syncTencentRule 同步单个腾讯云规则，返回是否应该删除该规则
func (s *FirewallService) syncTencentRule(rule *model.FirewallRule) (bool, error) {
	// TODO: 实现腾讯云规则同步逻辑
	return false, nil
}

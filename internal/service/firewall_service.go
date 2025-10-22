package service

import (
	"FireFlow/internal/logger"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/utils"
	"FireFlow/pkg/cloud"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

type FirewallService struct {
	repo          repository.FirewallRepository
	tencentClient *cloud.TencentClient
	aliyunClient  *cloud.AliyunClient
	huaweiClient  *cloud.HuaweiClient
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
			logger.Errorf("Failed to initialize Tencent Cloud client: %v", err)
		} else {
			// logger.Println("Successfully initialized Tencent Cloud client")
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
			logger.Errorf("Failed to initialize Aliyun ECS client: %v", err)
		} else {
			// logger.Println("Successfully initialized Aliyun ECS client")
		}
	}

	// Initialize Huawei Cloud client from config
	huaweiConfig := &cloud.HuaweiConfig{
		Region:    viper.GetString("cloud.huawei.region"),
		AK:        viper.GetString("cloud.huawei.ak"),
		SK:        viper.GetString("cloud.huawei.sk"),
		ProjectID: viper.GetString("cloud.huawei.project_id"),
	}

	var huaweiClient *cloud.HuaweiClient
	if huaweiConfig.AK != "" && huaweiConfig.SK != "" {
		var err error
		huaweiClient, err = cloud.NewHuaweiClient(huaweiConfig)
		if err != nil {
			logger.Errorf("Failed to initialize Huawei Cloud client: %v", err)
		} else {
			// logger.Println("Successfully initialized Huawei Cloud client")
		}
	}

	return &FirewallService{
		repo:          repo,
		tencentClient: tencentClient,
		aliyunClient:  aliyunClient,
		huaweiClient:  huaweiClient,
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
	// 获取当前的时间间隔配置
	intervalStr, err := s.configService.GetConfig("ip_check_interval")
	if err != nil || intervalStr == "" {
		intervalStr = "30" // 默认30分钟
	}

	// 获取并验证当前公网IP
	currentIP, err := utils.GetValidatedPublicIP(s.configService)
	if err != nil {
		logger.Errorf("Error getting/validating public IP: %v", err)
		return
	}
	logger.Printf("Current public IP is: %s", currentIP)

	// 获取所有启用的规则
	rules, err := s.repo.GetAllEnabled()
	if err != nil {
		logger.Errorf("Error getting firewall rules: %v", err)
		return
	}

	// 逐个处理规则
	for _, rule := range rules {
		// 获取该规则的独立锁
		ruleLock := s.getRuleLock(rule.ID)
		ruleLock.Lock()

		s.processRule(rule, currentIP)

		ruleLock.Unlock()
	}
	// logger.Printf("Firewall update job finished. (interval: %s minutes)", intervalStr)
}

// processRule 处理单个规则的更新逻辑
func (s *FirewallService) processRule(rule model.FirewallRule, currentIP string) {
	// 只处理有备注的规则
	if rule.Remark == "" {
		logger.Warnf("Skipping rule %d: no remark provided", rule.ID)
		return
	}

	// 检查规则是否启用
	if !rule.Enabled {
		logger.Warnf("Skipping rule %d: rule is disabled", rule.ID)
		return
	}

	// 检查对应的云服务配置是否启用
	if err := s.checkCloudConfigEnabled(rule.Provider); err != nil {
		logger.Errorf("Skipping rule %d: %v", rule.ID, err)
		return
	}

	// logger.Printf("Processing rule %d (%s) - Current IP: %s, Last IP: %s", rule.ID, rule.Remark, currentIP, rule.LastIP)

	var updateErr error
	switch rule.Provider {
	case "TencentCloud":
		updateErr = s.updateTencentFirewallRule(&rule, currentIP)
	case "Aliyun":
		updateErr = s.updateAliyunFirewallRule(&rule, currentIP)
	case "HuaweiCloud":
		updateErr = s.updateHuaweiFirewallRule(&rule, currentIP)
	default:
		updateErr = fmt.Errorf("unsupported provider: %s", rule.Provider)
	}

	if updateErr != nil {
		logger.Errorf("Failed to update rule %d: %v", rule.ID, updateErr)
	} else {
		if err := s.repo.UpdateIP(rule.ID, currentIP); err != nil {
			logger.Errorf("Failed to update IP in database for rule %d: %v", rule.ID, err)
		} else {
			logger.Printf("Successfully updated rule %d to IP %s", rule.ID, currentIP)
		}
	}
}

// CheckIfShouldRunNow 检查是否应该立即运行更新任务
func (s *FirewallService) CheckIfShouldRunNow(intervalMinutes int) (bool, error) {
	// 获取最早的更新时间
	oldestTime, err := s.repo.GetOldestUpdatedTime()
	if err != nil {
		return false, fmt.Errorf("failed to get oldest updated time: %v", err)
	}

	// 如果没有任何更新记录，应该立即执行
	if oldestTime == nil {
		logger.Println("No previous update records found, should run immediately")
		return true, nil
	}

	// 计算距离现在的时间差
	timeSinceUpdate := time.Since(*oldestTime)
	intervalDuration := time.Duration(intervalMinutes) * time.Minute

	shouldRun := timeSinceUpdate >= intervalDuration

	if shouldRun {
		logger.Printf("Oldest update was %v ago (interval: %v), should run immediately", timeSinceUpdate.Round(time.Minute), intervalDuration)
	}
	// else {
	// 	logger.Printf("Oldest update was %v ago (interval: %v), will wait for scheduled time",	timeSinceUpdate.Round(time.Minute), intervalDuration)
	// }

	return shouldRun, nil
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
func (s *FirewallService) createAndUpdateTencentFirewallRule(rule *model.FirewallRule, currentIP string) (*cloud.FirewallRuleResult, error) {
	// 获取腾讯云客户端
	tencentClient, err := s.getTencentClient(rule.CloudConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Tencent Cloud client: %v", err)
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
		return nil, fmt.Errorf("failed to create firewall rule: %v", err)
	}

	// 检查返回的IP是否与当前IP一致
	if result != nil && result.CidrBlock != "" {
		// 从CIDR块中提取IP（移除/32后缀）
		resultIP := strings.TrimSuffix(result.CidrBlock, "/32")
		if resultIP == currentIP {
			logger.Printf("Rule %d (%s): IP未变动 (当前IP: %s)，规则已更新", rule.ID, rule.Remark, currentIP)
		} else {
			logger.Printf("Rule %d (%s): IP已更新 (从 %s 到 %s)", rule.ID, rule.Remark, rule.LastIP, resultIP)
		}

		// 使用返回的实际IP更新数据库
		rule.LastIP = resultIP
	} else {
		// 如果没有返回IP信息，使用请求的IP
		rule.LastIP = currentIP
	}

	// 更新数据库中的规则信息
	err = s.repo.Update(rule)
	if err != nil {
		logger.Warnf("Warning: Rule created in cloud but failed to update database: %v", err)
	}

	// logger.Printf("Successfully created and executed firewall rule for instance %s", rule.InstanceID)
	return result, nil
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
	_, err = tencentClient.UpdateFirewallRule(rule.InstanceID, ruleSpec, newIP)
	if err != nil {
		// 如果更新失败且错误信息表明规则不存在，尝试重新创建规则
		if strings.Contains(err.Error(), "not found") {
			logger.Errorf("Rule not found in cloud, attempting to recreate it")
			_, err = s.createAndUpdateTencentFirewallRule(rule, newIP)
			return err
		}
		return err
	}

	// 更新数据库中的规则信息
	rule.LastIP = newIP
	if err := s.repo.Update(rule); err != nil {
		logger.Warnf("Warning: Failed to update rule in database: %v", err)
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

	// 根据Type字段验证配置类型
	var serverTypeDesc string
	switch cloudConfig.Type {
	case 0:
		serverTypeDesc = "CVM"
	case 1:
		serverTypeDesc = "轻量应用服务器"
	default:
		serverTypeDesc = "未知类型"
	}

	// 构建腾讯云配置
	tencentConfig := cloud.TencentConfig{
		SecretId:   cloudConfig.SecretId,
		SecretKey:  cloudConfig.SecretKey,
		Type:       cloudConfig.Type,
		Region:     cloudConfig.Region,
		InstanceId: cloudConfig.InstanceId,
	}

	// 创建腾讯云客户端
	client, err := cloud.NewTencentClient(tencentConfig)
	if err != nil {
		return nil, fmt.Errorf("创建腾讯云客户端失败 (%s): %v", serverTypeDesc, err)
	}
	return client, nil
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
	// 如果规则有CloudConfigID，自动填充ProjectID
	if rule.CloudConfigID > 0 && s.configService != nil {
		cloudConfig, err := s.configService.GetCloudConfigByID(rule.CloudConfigID)
		if err == nil && cloudConfig.ProjectID != "" {
			rule.ProjectID = cloudConfig.ProjectID
		}
	}

	return s.repo.Create(rule)
}

func (s *FirewallService) DeleteRule(id uint) error {
	return s.repo.Delete(id)
}

func (s *FirewallService) UpdateRule(rule *model.FirewallRule) error {
	return s.repo.Update(rule)
}

func (s *FirewallService) ExecuteRule(id uint) (map[string]interface{}, error) {
	// 获取规则
	rule, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get rule: %v", err)
	}

	// 检查规则是否启用
	if !rule.Enabled {
		return nil, fmt.Errorf("rule %d is disabled, skipping execution", id)
	}

	// 检查对应的云服务配置是否启用
	if err := s.checkCloudConfigEnabled(rule.Provider); err != nil {
		return nil, err
	}

	// 获取并验证当前公网IP
	currentIP, err := utils.GetValidatedPublicIP(s.configService)
	if err != nil {
		return nil, fmt.Errorf("failed to get/validate current IP: %v", err)
	}

	// 检查IP是否发生变化
	result := map[string]interface{}{
		"current_ip": currentIP,
	}

	// 执行规则更新，获取云服务返回的结果
	var cloudResult *cloud.FirewallRuleResult
	var updateErr error
	switch rule.Provider {
	case "TencentCloud":
		cloudResult, updateErr = s.createAndUpdateTencentFirewallRule(rule, currentIP)
	case "Aliyun":
		cloudResult, updateErr = s.createAndUpdateAliyunFirewallRule(rule, currentIP)
	case "HuaweiCloud":
		cloudResult, updateErr = s.createAndUpdateHuaweiFirewallRule(rule, currentIP)
	default:
		updateErr = fmt.Errorf("unsupported provider: %s", rule.Provider)
	}

	if updateErr != nil {
		result["message"] = fmt.Sprintf("更新防火墙规则失败: %v", updateErr)
		result["status"] = "error"
		return result, updateErr
	}

	// 比较当前IP与云服务返回的IP
	if cloudResult != nil && cloudResult.CidrBlock != "" {
		// 从CIDR块中提取IP（移除/32后缀）
		cloudIP := strings.TrimSuffix(cloudResult.CidrBlock, "/32")

		if cloudIP == currentIP {
			result["message"] = "IP未变动"
			result["ip_changed"] = false
			result["status"] = "unchanged"
			result["cloud_ip"] = cloudIP
		} else {
			result["message"] = fmt.Sprintf("IP已更新，防火墙规则已同步 (当前IP: %s, 云端IP: %s)", currentIP, cloudIP)
			result["status"] = "updated"
			result["ip_changed"] = true
			result["cloud_ip"] = cloudIP
		}
	} else {
		result["message"] = "防火墙规则已更新，但未获取到云端IP信息"
		result["ip_changed"] = true
		result["status"] = "updated"
	}

	return result, nil
}

// CreateTencentFirewallRule creates a new firewall rule in Tencent Cloud and saves it to database
func (s *FirewallService) CreateTencentFirewallRule(instanceID, port, cidrBlock, protocol, description string) error {
	if s.tencentClient == nil {
		return fmt.Errorf("create firewall rule failed, tencent cloud client not initialized")
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
	_, err := s.tencentClient.CreateFirewallRule(instanceID, ruleSpec)
	if err != nil {
		return fmt.Errorf("failed to create firewall rule in Tencent Cloud: %v", err)
	}

	// 保存到数据库
	rule := &model.FirewallRule{
		Provider:   "TencentCloud",
		InstanceID: instanceID,
		Port:       port,
		LastIP:     cidrBlock,
		Enabled:    true,
		Remark:     description,
	}

	return s.repo.Create(rule)
}

// GetInstanceInfo gets information about a cloud instance
func (s *FirewallService) GetInstanceInfo(instanceID string) (*cloud.InstanceInfo, error) {
	if s.tencentClient == nil {
		return nil, fmt.Errorf("get instance info failed, tencent cloud client not initialized")
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

	_, err = client.CreateFirewallRule(rule.InstanceID, ruleSpec)
	if err != nil {
		return fmt.Errorf("创建/更新阿里云防火墙规则失败: %v", err)
	}

	// logger.Printf("Successfully updated Aliyun firewall rule for instance %s, IP: %s", rule.InstanceID, newIP)

	return nil
}

// createAndUpdateAliyunFirewallRule 创建并更新阿里云防火墙规则
func (s *FirewallService) createAndUpdateAliyunFirewallRule(rule *model.FirewallRule, currentIP string) (*cloud.FirewallRuleResult, error) {
	client, err := s.getAliyunClient(rule.CloudConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Aliyun client: %v", err)
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
		return nil, fmt.Errorf("failed to create Aliyun firewall rule: %v", err)
	}

	// 检查返回的IP是否与当前IP一致
	var updateIP string
	if result != nil && result.CidrBlock != "" {
		// 从CIDR块中提取IP（移除/32后缀）
		resultIP := strings.TrimSuffix(result.CidrBlock, "/32")
		if resultIP == currentIP {
			logger.Printf("Rule %d (%s): IP unchanged (Current IP: %s), Rule has been updated", rule.ID, rule.Remark, currentIP)
		} else {
			logger.Printf("Rule %d (%s): IP updated (From %s to %s)", rule.ID, rule.Remark, rule.LastIP, resultIP)
		}
		updateIP = resultIP
	} else {
		// 如果没有返回IP信息，使用请求的IP
		updateIP = currentIP
	}

	// 更新数据库中的IP
	if err := s.repo.UpdateIP(rule.ID, updateIP); err != nil {
		logger.Errorf("Failed to update IP in database: %v", err)
	}

	// logger.Printf("Successfully created Aliyun firewall rule for instance %s, IP: %s", rule.InstanceID, updateIP)

	return result, nil
}

// CreateAliyunFirewallRule 创建新的阿里云防火墙规则并保存到数据库
func (s *FirewallService) CreateAliyunFirewallRule(instanceID, port, cidrBlock, protocol, description string) error {
	if s.aliyunClient == nil {
		return fmt.Errorf("create firewall rule failed, aliyun client not initialized")
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
	_, err := s.aliyunClient.CreateFirewallRule(instanceID, ruleSpec)
	if err != nil {
		return fmt.Errorf("failed to create Aliyun firewall rule: %v", err)
	}

	// 保存到数据库
	rule := &model.FirewallRule{
		Remark:     description,
		InstanceID: instanceID,
		Port:       port,
		Protocol:   protocol,
		Provider:   "Aliyun",
		Enabled:    true,
	}

	if err := s.repo.Create(rule); err != nil {
		return fmt.Errorf("failed to save rule to database: %v", err)
	}

	// logger.Printf("Successfully created and saved Aliyun firewall rule for instance %s", instanceID)
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

	// 根据Type字段验证配置类型
	var serverTypeDesc string
	switch config.Type {
	case 0:
		serverTypeDesc = "ECS"
	case 1:
		serverTypeDesc = "轻量应用服务器"
	default:
		serverTypeDesc = "未知类型"
	}

	// 解析阿里云配置
	aliyunConfig := cloud.AliyunConfig{
		AccessKeyID:      config.SecretId,
		AccessKeySecret:  config.SecretKey,
		RegionID:         config.Region,
		Type:             config.Type,
		SecurityGroupIds: config.InstanceId, // 在云配置中，实例ID字段用于存储安全组ID
	}

	client, err := cloud.NewAliyunClient(aliyunConfig)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云客户端失败 (%s): %v", serverTypeDesc, err)
	}
	return client, nil
}

// GetAliyunInstanceInfo 获取阿里云实例信息
func (s *FirewallService) GetAliyunInstanceInfo(instanceID string, cloudConfigID uint) (*cloud.InstanceInfo, error) {
	client, err := s.getAliyunClient(cloudConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Aliyun client: %v", err)
	}

	return client.GetInstance(instanceID)
}

// getHuaweiClient 获取华为云客户端
func (s *FirewallService) getHuaweiClient(cloudConfigID uint) (*cloud.HuaweiClient, error) {
	// 如果有全局客户端，直接使用
	if s.huaweiClient != nil {
		return s.huaweiClient, nil
	}

	// 否则根据配置ID创建客户端
	if s.configService == nil {
		return nil, fmt.Errorf("config service not available")
	}

	config, err := s.configService.GetCloudConfigByID(cloudConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cloud config: %v", err)
	}

	if config.Provider != "HuaweiCloud" {
		return nil, fmt.Errorf("invalid provider: expected HuaweiCloud, got %s", config.Provider)
	}

	// 根据Type字段验证配置类型（华为云类型字段无效，但仍显示）
	var serverTypeDesc string
	switch config.Type {
	case 0:
		serverTypeDesc = "ECS/Flexus"
	case 1:
		serverTypeDesc = "其他"
	default:
		serverTypeDesc = "华为云未知类型"
	}

	// 解析华为云配置
	huaweiConfig := &cloud.HuaweiConfig{
		Region:          config.Region,
		AK:              config.SecretId,
		SK:              config.SecretKey,
		ProjectID:       config.ProjectID,
		SecurityGroupID: config.InstanceId, // 在云配置中，实例ID字段用于存储安全组ID
	}

	client, err := cloud.NewHuaweiClient(huaweiConfig)
	if err != nil {
		return nil, fmt.Errorf("创建华为云客户端失败 (%s): %v", serverTypeDesc, err)
	}
	return client, nil
}

// updateHuaweiFirewallRule 更新华为云防火墙规则
func (s *FirewallService) updateHuaweiFirewallRule(rule *model.FirewallRule, newIP string) error {
	client, err := s.getHuaweiClient(rule.CloudConfigID)
	if err != nil {
		return fmt.Errorf("failed to get Huawei Cloud client: %v", err)
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
	result, err := client.UpdateFirewallRule(rule.InstanceID, ruleSpec, newIP)
	if err != nil {
		// 如果更新失败，可能是规则已被手动删除，尝试重新创建
		errStr := err.Error()
		if strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "rule does not exist") {
			logger.Warnf("Rule ID %d not found in cloud, attempting to recreate", rule.ID)

			// 尝试重新创建规则
			_, createErr := client.CreateFirewallRule(rule.InstanceID, ruleSpec)
			if createErr != nil {
				return fmt.Errorf("failed to recreate Huawei Cloud firewall rule after rule not found: %v", createErr)
			}

			// 更新数据库中的规则信息
			if err := s.repo.Update(rule); err != nil {
				logger.Errorf("Failed to update rule in database: %v", err)
			}

			logger.Printf("Successfully recreated Huawei Cloud firewall rule for instance %s: %s",
				rule.InstanceID, newIP)
			return nil
		}
		return fmt.Errorf("failed to update Huawei Cloud firewall rule: %v", err)
	}

	// 更新数据库中的规则信息
	if result != nil {
		if err := s.repo.Update(rule); err != nil {
			logger.Warnf("Warning: Failed to update rule in database: %v", err)
		}
	}

	return nil
}

// createAndUpdateHuaweiFirewallRule 创建并更新华为云防火墙规则
func (s *FirewallService) createAndUpdateHuaweiFirewallRule(rule *model.FirewallRule, currentIP string) (*cloud.FirewallRuleResult, error) {
	client, err := s.getHuaweiClient(rule.CloudConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Huawei Cloud client: %v", err)
	}

	// 创建防火墙规则规格
	ruleSpec := &cloud.FirewallRuleSpec{
		Port:        rule.Port,
		Protocol:    rule.Protocol,
		CidrBlock:   fmt.Sprintf("%s/32", currentIP),
		Action:      "ACCEPT",
		Description: rule.Remark,
	}

	// 创建或更新规则
	result, err := client.UpdateFirewallRule(rule.InstanceID, ruleSpec, currentIP)
	if err != nil {
		return nil, fmt.Errorf("failed to create/update Huawei Cloud firewall rule: %v", err)
	}

	// 更新数据库中的IP
	if err := s.repo.UpdateIP(rule.ID, currentIP); err != nil {
		logger.Errorf("Failed to update IP in database: %v", err)
	}

	logger.Printf("Successfully created/updated Huawei Cloud firewall rule for instance %s with IP %s",
		rule.InstanceID, currentIP)

	return result, nil
}

// GetHuaweiInstanceInfo 获取华为云实例信息
func (s *FirewallService) GetHuaweiInstanceInfo(instanceID string, cloudConfigID uint) (*cloud.InstanceInfo, error) {
	client, err := s.getHuaweiClient(cloudConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Huawei Cloud client: %v", err)
	}

	return client.GetInstance(instanceID)
}

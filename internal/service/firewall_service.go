package service

import (
	"FireFlow/internal/logger"
	"FireFlow/internal/model"
	"FireFlow/internal/repository"
	"FireFlow/internal/utils"
	"FireFlow/pkg/cloud"
	"errors"
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
	azureClient   *cloud.AzureClient
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
	if _, err := s.SyncAllRules(); err != nil {
		logger.Errorf("Firewall sync failed: %v", err)
	}
}

type SyncResult struct {
	CurrentIP    string `json:"current_ip"`
	UpdatedRules int    `json:"updated_rules"`
	FailedRules  int    `json:"failed_rules"`
}

// SyncAllRules fetches the IP once and reports actual update outcomes.
func (s *FirewallService) SyncAllRules() (SyncResult, error) {
	result := SyncResult{}
	// 获取并验证当前公网IP
	currentIP, err := utils.GetValidatedPublicIP(s.configService)
	if err != nil {
		return result, err
	}
	result.CurrentIP = currentIP
	logger.Printf("Current public IP is: %s", currentIP)

	// 获取所有启用的规则
	rules, err := s.repo.GetAllEnabled()
	if err != nil {
		return result, fmt.Errorf("getting firewall rules: %w", err)
	}

	var failures []error
	// 逐个处理规则
	for _, rule := range rules {
		// 获取该规则的独立锁
		ruleLock := s.getRuleLock(rule.ID)
		ruleLock.Lock()

		// Re-read after acquiring the lock to avoid using stale rule data.
		latest, updateErr := s.repo.GetByID(rule.ID)
		if updateErr == nil {
			updateErr = s.processRule(*latest, currentIP)
		}

		ruleLock.Unlock()
		if updateErr != nil {
			result.FailedRules++
			failures = append(failures, fmt.Errorf("rule %d: %w", rule.ID, updateErr))
		} else {
			result.UpdatedRules++
		}
	}
	return result, errors.Join(failures...)
}

// processRule 处理单个规则的更新逻辑
func (s *FirewallService) processRule(rule model.FirewallRule, currentIP string) error {
	// 只处理有备注的规则
	if rule.Remark == "" {
		return fmt.Errorf("no remark provided")
	}

	// 检查规则是否启用
	if !rule.Enabled {
		return fmt.Errorf("rule is disabled")
	}

	// 检查对应的云服务配置是否启用
	if err := s.checkCloudConfigEnabled(&rule); err != nil {
		return err
	}

	// logger.Printf("Processing rule %d (%s) - Current IP: %s, Last IP: %s", rule.ID, rule.Remark, currentIP, rule.LastIP)

	var updateErr error
	switch rule.CloudConfig.Provider {
	case "TencentCloud":
		updateErr = s.updateTencentFirewallRule(&rule, currentIP)
	case "Aliyun":
		updateErr = s.updateAliyunFirewallRule(&rule, currentIP)
	case "HuaweiCloud":
		updateErr = s.updateHuaweiFirewallRule(&rule, currentIP)
	case "Azure":
		updateErr = s.updateAzureFirewallRule(&rule, currentIP)
	default:
		updateErr = fmt.Errorf("unsupported provider: %s", rule.CloudConfig.Provider)
	}

	if updateErr != nil {
		return updateErr
	}
	if err := s.repo.UpdateIP(rule.ID, currentIP); err != nil {
		return fmt.Errorf("saving updated IP: %w", err)
	}
	logger.Printf("Successfully updated rule %d to IP %s", rule.ID, currentIP)
	return nil
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
func (s *FirewallService) checkCloudConfigEnabled(rule *model.FirewallRule) error {
	if s.configService == nil {
		return fmt.Errorf("config service not available")
	}
	// 获取该提供商的云服务配置
	if err := s.applyCloudConfig(rule); err != nil {
		return err
	}
	config := &rule.CloudConfig

	if !config.IsEnabled {
		return fmt.Errorf("cloud config %d for provider %s is disabled", config.ID, rule.CloudConfig.Provider)
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
	result, err := tencentClient.CreateFirewallRule(rule.CloudConfig.InstanceId, ruleSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall rule: %v", err)
	}

	// 检查返回的IP是否与当前IP一致
	if result != nil && result.CidrBlock != "" {
		// 从CIDR块中提取IP（移除/32后缀）
		resultIP := strings.TrimSuffix(result.CidrBlock, "/32")
		if !result.Changed {
			logger.Printf("Rule %d (%s): IP未变动 (当前IP: %s)，规则已更新", rule.ID, rule.Remark, currentIP)
		} else {
			logger.Printf("Rule %d (%s): IP已更新 (从 %s 到 %s)", rule.ID, rule.Remark, strings.TrimSuffix(result.PreviousCidrBlock, "/32"), resultIP)
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

	// logger.Printf("Successfully created and executed firewall rule for instance %s", rule.CloudConfig.InstanceId)
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
	_, err = tencentClient.UpdateFirewallRule(rule.CloudConfig.InstanceId, ruleSpec, newIP)
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
	if err := s.applyCloudConfig(rule); err != nil {
		return err
	}

	return s.repo.Create(rule)
}

func (s *FirewallService) DeleteRule(id uint) error {
	return s.repo.Delete(id)
}

func (s *FirewallService) SetRuleEnabled(id uint, enabled bool) error {
	lock := s.getRuleLock(id)
	lock.Lock()
	defer lock.Unlock()
	return s.repo.SetEnabled(id, enabled)
}

func (s *FirewallService) UpdateRule(rule *model.FirewallRule) error {
	lock := s.getRuleLock(rule.ID)
	lock.Lock()
	defer lock.Unlock()
	previous, err := s.repo.GetByID(rule.ID)
	if err != nil {
		return err
	}
	if err := s.applyCloudConfig(rule); err != nil {
		return err
	}
	if rule.CloudConfigID != previous.CloudConfigID {
		rule.LastIP = ""
	} else {
		rule.LastIP = previous.LastIP
	}
	return s.repo.Update(rule)
}

func (s *FirewallService) applyCloudConfig(rule *model.FirewallRule) error {
	if rule.CloudConfigID == 0 {
		return fmt.Errorf("cloud_config_id is required")
	}
	if s.configService == nil {
		return fmt.Errorf("config service not available")
	}
	config, err := s.configService.GetCloudConfigByID(rule.CloudConfigID)
	if err != nil {
		return fmt.Errorf("invalid cloud config ID: %w", err)
	}
	rule.CloudConfig = *config
	return nil
}

func (s *FirewallService) ExecuteRule(id uint) (map[string]interface{}, error) {
	lock := s.getRuleLock(id)
	lock.Lock()
	defer lock.Unlock()
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
	if err := s.checkCloudConfigEnabled(rule); err != nil {
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
	switch rule.CloudConfig.Provider {
	case "TencentCloud":
		cloudResult, updateErr = s.createAndUpdateTencentFirewallRule(rule, currentIP)
	case "Aliyun":
		cloudResult, updateErr = s.createAndUpdateAliyunFirewallRule(rule, currentIP)
	case "HuaweiCloud":
		cloudResult, updateErr = s.createAndUpdateHuaweiFirewallRule(rule, currentIP)
	case "Azure":
		cloudResult, updateErr = s.createAndUpdateAzureFirewallRule(rule, currentIP)
	default:
		updateErr = fmt.Errorf("unsupported provider: %s", rule.CloudConfig.Provider)
	}

	if updateErr != nil {
		result["message"] = fmt.Sprintf("更新防火墙规则失败: %v", updateErr)
		result["status"] = "error"
		return result, updateErr
	}

	return firewallExecutionResult(currentIP, cloudResult)
}

func firewallExecutionResult(currentIP string, cloudResult *cloud.FirewallRuleResult) (map[string]interface{}, error) {
	if cloudResult == nil || cloudResult.CidrBlock == "" {
		return nil, fmt.Errorf("cloud provider returned no synchronized IP")
	}
	cloudIP := strings.TrimSuffix(cloudResult.CidrBlock, "/32")
	previousIP := strings.TrimSuffix(cloudResult.PreviousCidrBlock, "/32")
	result := map[string]interface{}{
		"current_ip":  currentIP,
		"cloud_ip":    cloudIP,
		"previous_ip": previousIP,
		"ip_changed":  cloudResult.Changed,
	}
	if cloudIP != currentIP {
		return nil, fmt.Errorf("cloud IP %s does not match current IP %s after synchronization", cloudIP, currentIP)
	}
	if !cloudResult.Changed {
		result["status"] = "unchanged"
		result["message"] = "IP未变动，云端规则已与当前IP一致"
	} else if previousIP == "" {
		result["status"] = "updated"
		result["message"] = "云端规则已创建，并同步当前IP"
	} else {
		result["status"] = "updated"
		result["message"] = fmt.Sprintf("IP已从 %s 更新为 %s", previousIP, cloudIP)
	}
	return result, nil
}

// CreateTencentFirewallRule creates a new firewall rule in Tencent Cloud and saves it to database
func (s *FirewallService) CreateTencentFirewallRule(instanceID, port, cidrBlock, protocol, description string) error {
	return s.createCloudFirewallRule("TencentCloud", instanceID, port, cidrBlock, protocol, description, 0)
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

	_, err = client.CreateFirewallRule(rule.CloudConfig.InstanceId, ruleSpec)
	if err != nil {
		return fmt.Errorf("创建/更新阿里云防火墙规则失败: %v", err)
	}

	// logger.Printf("Successfully updated Aliyun firewall rule for instance %s, IP: %s", rule.CloudConfig.InstanceId, newIP)

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
	result, err := client.CreateFirewallRule(rule.CloudConfig.InstanceId, ruleSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to create Aliyun firewall rule: %v", err)
	}

	// 检查返回的IP是否与当前IP一致
	var updateIP string
	if result != nil && result.CidrBlock != "" {
		// 从CIDR块中提取IP（移除/32后缀）
		resultIP := strings.TrimSuffix(result.CidrBlock, "/32")
		if !result.Changed {
			logger.Printf("Rule %d (%s): IP unchanged (Current IP: %s), Rule has been updated", rule.ID, rule.Remark, currentIP)
		} else {
			logger.Printf("Rule %d (%s): IP updated (From %s to %s)", rule.ID, rule.Remark, strings.TrimSuffix(result.PreviousCidrBlock, "/32"), resultIP)
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

	// logger.Printf("Successfully created Aliyun firewall rule for instance %s, IP: %s", rule.CloudConfig.InstanceId, updateIP)

	return result, nil
}

// CreateAliyunFirewallRule 创建新的阿里云防火墙规则并保存到数据库
func (s *FirewallService) CreateAliyunFirewallRule(instanceID, port, cidrBlock, protocol, description string) error {
	return s.createCloudFirewallRule("Aliyun", instanceID, port, cidrBlock, protocol, description, 0)
}

// getAliyunClient 获取阿里云客户端
func (s *FirewallService) getAliyunClient(cloudConfigID uint) (*cloud.AliyunClient, error) {
	// 如果有全局客户端，直接使用
	if cloudConfigID == 0 && s.aliyunClient != nil {
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
	if cloudConfigID == 0 && s.huaweiClient != nil {
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
	result, err := client.UpdateFirewallRule(rule.CloudConfig.InstanceId, ruleSpec, newIP)
	if err != nil {
		// 如果更新失败，可能是规则已被手动删除，尝试重新创建
		errStr := err.Error()
		if strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "rule does not exist") {
			logger.Warnf("Rule ID %d not found in cloud, attempting to recreate", rule.ID)

			// 尝试重新创建规则
			_, createErr := client.CreateFirewallRule(rule.CloudConfig.InstanceId, ruleSpec)
			if createErr != nil {
				return fmt.Errorf("failed to recreate Huawei Cloud firewall rule after rule not found: %v", createErr)
			}

			// 更新数据库中的规则信息
			if err := s.repo.Update(rule); err != nil {
				logger.Errorf("Failed to update rule in database: %v", err)
			}

			logger.Printf("Successfully recreated Huawei Cloud firewall rule for instance %s: %s",
				rule.CloudConfig.InstanceId, newIP)
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
	result, err := client.UpdateFirewallRule(rule.CloudConfig.InstanceId, ruleSpec, currentIP)
	if err != nil {
		return nil, fmt.Errorf("failed to create/update Huawei Cloud firewall rule: %v", err)
	}

	// 更新数据库中的IP
	if err := s.repo.UpdateIP(rule.ID, currentIP); err != nil {
		logger.Errorf("Failed to update IP in database: %v", err)
	}

	logger.Printf("Successfully created/updated Huawei Cloud firewall rule for instance %s with IP %s",
		rule.CloudConfig.InstanceId, currentIP)

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

// getAzureClient 获取 Azure 客户端
func (s *FirewallService) getAzureClient(cloudConfigID uint) (*cloud.AzureClient, error) {
	// 如果有全局客户端，直接使用
	if cloudConfigID == 0 && s.azureClient != nil {
		return s.azureClient, nil
	}

	// 否则根据配置ID创建客户端
	if s.configService == nil {
		return nil, fmt.Errorf("config service not available")
	}

	config, err := s.configService.GetCloudConfigByID(cloudConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cloud config: %v", err)
	}

	if config.Provider != "Azure" {
		return nil, fmt.Errorf("invalid provider: expected Azure, got %s", config.Provider)
	}

	// 解析 Azure 配置
	azureConfig := cloud.AzureConfig{
		SubscriptionID:    config.SubscriptionID,
		TenantID:          config.TenantID,
		ClientID:          config.SecretId,
		ClientSecret:      config.SecretKey,
		ResourceGroupName: config.ProjectID,
		SecurityGroupName: config.InstanceId, // 在云配置中，实例ID字段用于存储网络安全组名称
		Location:          config.Region,
	}

	client, err := cloud.NewAzureClient(azureConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 Azure 客户端失败: %v", err)
	}
	return client, nil
}

// updateAzureFirewallRule 更新 Azure 防火墙规则
func (s *FirewallService) updateAzureFirewallRule(rule *model.FirewallRule, newIP string) error {
	client, err := s.getAzureClient(rule.CloudConfigID)
	if err != nil {
		return fmt.Errorf("failed to get Azure client: %v", err)
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
	result, err := client.UpdateFirewallRule(rule.CloudConfig.InstanceId, ruleSpec, newIP)
	if err != nil {
		// 如果更新失败，可能是规则已被手动删除，尝试重新创建
		errStr := err.Error()
		if strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "rule does not exist") {
			logger.Warnf("Rule ID %d not found in cloud, attempting to recreate", rule.ID)

			// 尝试重新创建规则
			_, createErr := client.CreateFirewallRule(rule.CloudConfig.InstanceId, ruleSpec)
			if createErr != nil {
				return fmt.Errorf("failed to recreate Azure firewall rule after rule not found: %v", createErr)
			}

			// 更新数据库中的规则信息
			if err := s.repo.Update(rule); err != nil {
				logger.Errorf("Failed to update rule in database: %v", err)
			}

			logger.Printf("Successfully recreated Azure firewall rule for instance %s: %s",
				rule.CloudConfig.InstanceId, newIP)
			return nil
		}
		return fmt.Errorf("failed to update Azure firewall rule: %v", err)
	}

	// 更新数据库中的规则信息
	if result != nil {
		if err := s.repo.Update(rule); err != nil {
			logger.Warnf("Warning: Failed to update rule in database: %v", err)
		}
	}

	return nil
}

// createAndUpdateAzureFirewallRule 创建并更新 Azure 防火墙规则
func (s *FirewallService) createAndUpdateAzureFirewallRule(rule *model.FirewallRule, currentIP string) (*cloud.FirewallRuleResult, error) {
	client, err := s.getAzureClient(rule.CloudConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure client: %v", err)
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
	result, err := client.UpdateFirewallRule(rule.CloudConfig.InstanceId, ruleSpec, currentIP)
	if err != nil {
		return nil, fmt.Errorf("failed to create/update Azure firewall rule: %v", err)
	}

	// 更新数据库中的IP
	if err := s.repo.UpdateIP(rule.ID, currentIP); err != nil {
		logger.Errorf("Failed to update IP in database: %v", err)
	}

	logger.Printf("Successfully created/updated Azure firewall rule for instance %s with IP %s",
		rule.CloudConfig.InstanceId, currentIP)

	return result, nil
}

// CreateAzureFirewallRule 创建新的 Azure 防火墙规则并保存到数据库
func (s *FirewallService) CreateAzureFirewallRule(instanceID, port, cidrBlock, protocol, description string, cloudConfigID uint) error {
	return s.createCloudFirewallRule("Azure", instanceID, port, cidrBlock, protocol, description, cloudConfigID)
}

// GetAzureInstanceInfo 获取 Azure 实例信息
func (s *FirewallService) GetAzureInstanceInfo(instanceID string, cloudConfigID uint) (*cloud.InstanceInfo, error) {
	client, err := s.getAzureClient(cloudConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure client: %v", err)
	}

	return client.GetInstance(instanceID)
}

// Legacy helpers resolve a unique config before issuing a cloud request.
func (s *FirewallService) createCloudFirewallRule(provider, instanceID, port, cidrBlock, protocol, description string, configID uint) error {
	if s.configService == nil {
		return fmt.Errorf("config service not available")
	}
	if configID == 0 {
		configs, err := s.configService.ListCloudConfigs()
		if err != nil {
			return err
		}
		for _, config := range configs {
			if config.Provider == provider && config.InstanceId == instanceID {
				if configID != 0 {
					return fmt.Errorf("multiple cloud configs match instance %s", instanceID)
				}
				configID = config.ID
			}
		}
	}
	rule := model.FirewallRule{CloudConfigID: configID, Port: port, Protocol: protocol, LastIP: strings.TrimSuffix(cidrBlock, "/32"), Enabled: true, Remark: description}
	if err := s.checkCloudConfigEnabled(&rule); err != nil {
		return err
	}
	if rule.CloudConfig.Provider != provider || rule.CloudConfig.InstanceId != instanceID {
		return fmt.Errorf("cloud config does not match provider and instance")
	}
	spec := &cloud.FirewallRuleSpec{Port: port, Protocol: protocol, CidrBlock: cidrBlock, Action: "ACCEPT", Description: description}
	switch provider {
	case "TencentCloud":
		client, err := s.getTencentClient(configID)
		if err != nil {
			return err
		}
		if _, err := client.CreateFirewallRule(instanceID, spec); err != nil {
			return err
		}
	case "Aliyun":
		client, err := s.getAliyunClient(configID)
		if err != nil {
			return err
		}
		if _, err := client.CreateFirewallRule(instanceID, spec); err != nil {
			return err
		}
	case "Azure":
		client, err := s.getAzureClient(configID)
		if err != nil {
			return err
		}
		if _, err := client.CreateFirewallRule(instanceID, spec); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}
	return s.repo.Create(&rule)
}

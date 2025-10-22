package cloud

import (
	"fmt"
	"strconv"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	ecs20140526 "github.com/alibabacloud-go/ecs-20140526/v7/client"
	swas20200601 "github.com/alibabacloud-go/swas-open-20200601/v3/client"
	"github.com/alibabacloud-go/tea/tea"
)

type AliyunConfig struct {
	AccessKeyID      string `json:"accessKeyId" binding:"required"`
	AccessKeySecret  string `json:"accessKeySecret" binding:"required"`
	Type             int    `json:"type" binding:"required"` // 0：ECS，1：轻量云
	RegionID         string `json:"regionId" binding:"required"`
	SecurityGroupIds string `json:"securityGroupIds" binding:"required"`
}

type AliyunClient struct {
	EcsClient  *ecs20140526.Client
	SwasClient *swas20200601.Client
	config     AliyunConfig
}

// SecurityGroupInfo 安全组信息结构体
type SecurityGroupInfo struct {
	SecurityGroupID string `json:"security_group_id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	VpcID           string `json:"vpc_id"`
	Region          string `json:"region"`
	RulesCount      int    `json:"rules_count"`
}

// NewAliyunClient 创建阿里云客户端
func NewAliyunClient(config AliyunConfig) (*AliyunClient, error) {
	// 验证配置
	if config.AccessKeyID == "" || config.AccessKeySecret == "" {
		return nil, fmt.Errorf("accessKeyId and accessKeySecret are required")
	}

	if config.RegionID == "" {
		config.RegionID = "cn-hangzhou" // 默认杭州区域
	}

	// logger.Printf("Initializing Aliyun ECS client with AccessKeyID: %s, Region: %s", maskAliyunSecretId(config.AccessKeyID), config.RegionID)

	// 创建客户端配置
	clientConfig := &openapi.Config{
		AccessKeyId:     tea.String(config.AccessKeyID),
		AccessKeySecret: tea.String(config.AccessKeySecret),
	}
	// 设置地域
	clientConfig.Endpoint = tea.String(fmt.Sprintf("ecs.%s.aliyuncs.com", config.RegionID))

	switch config.Type {
	case 0:
		// ECS
		clientConfig.Endpoint = tea.String(fmt.Sprintf("ecs.%s.aliyuncs.com", config.RegionID))
		// 初始化ECS客户端
		ecsClient, err := ecs20140526.NewClient(clientConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Aliyun ECS client: %v", err)
		}
		return &AliyunClient{
			EcsClient: ecsClient,
			config:    config,
		}, nil
	case 1:
		// 轻量云
		clientConfig.Endpoint = tea.String(fmt.Sprintf("swas.%s.aliyuncs.com", config.RegionID))
		// 初始化轻量云客户端
		swasClient, err := swas20200601.NewClient(clientConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Aliyun SWAS client: %v", err)
		}
		return &AliyunClient{
			SwasClient: swasClient,
			config:     config,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported Aliyun client type: %d", config.Type)
	}
}

// GetConfig 获取阿里云客户端配置
func (ac *AliyunClient) GetConfig() AliyunConfig {
	return ac.config
}

// maskAliyunSecretId 隐藏密钥ID的敏感部分
// func maskAliyunSecretId(secretId string) string {
// 	if len(secretId) <= 8 {
// 		return "****"
// 	}
// 	return secretId[:4] + "****" + secretId[len(secretId)-4:]
// }

// 实现 CloudProvider 接口
func (ac *AliyunClient) GetInstance(instanceID string) (*InstanceInfo, error) {
	switch ac.config.Type {
	case 0:
		// ECS 云服务器
		return ac.getECSInstance(instanceID)
	case 1:
		// SWAS 轻量应用服务器
		return ac.getSWASInstance(instanceID)
	default:
		return nil, fmt.Errorf("unsupported Aliyun client type: %d", ac.config.Type)
	}
}

// getECSInstance 获取ECS实例信息
func (ac *AliyunClient) getECSInstance(instanceID string) (*InstanceInfo, error) {
	if ac.EcsClient == nil {
		return nil, fmt.Errorf("ECS client not initialized")
	}

	request := &ecs20140526.DescribeInstancesRequest{
		RegionId:    tea.String(ac.config.RegionID),
		InstanceIds: tea.String(fmt.Sprintf(`["%s"]`, instanceID)),
	}

	response, err := ac.EcsClient.DescribeInstances(request)
	if err != nil {
		return nil, fmt.Errorf("failed to describe ECS instance: %v", err)
	}

	if response.Body.Instances == nil || len(response.Body.Instances.Instance) == 0 {
		return nil, fmt.Errorf("ECS instance %s not found", instanceID)
	}

	instance := response.Body.Instances.Instance[0]
	info := &InstanceInfo{
		InstanceID:   tea.StringValue(instance.InstanceId),
		InstanceName: tea.StringValue(instance.InstanceName),
		Status:       tea.StringValue(instance.Status),
		Provider:     "Aliyun",
		Region:       ac.config.RegionID,
	}

	// 获取公网IP
	if instance.PublicIpAddress != nil && len(instance.PublicIpAddress.IpAddress) > 0 {
		info.PublicIP = tea.StringValue(instance.PublicIpAddress.IpAddress[0])
	}

	// 获取私网IP
	if instance.VpcAttributes != nil && len(instance.VpcAttributes.PrivateIpAddress.IpAddress) > 0 {
		info.PrivateIP = tea.StringValue(instance.VpcAttributes.PrivateIpAddress.IpAddress[0])
	} else if instance.InnerIpAddress != nil && len(instance.InnerIpAddress.IpAddress) > 0 {
		info.PrivateIP = tea.StringValue(instance.InnerIpAddress.IpAddress[0])
	}

	return info, nil
}

// getSWASInstance 获取SWAS轻量应用服务器实例信息
func (ac *AliyunClient) getSWASInstance(instanceID string) (*InstanceInfo, error) {
	if ac.SwasClient == nil {
		return nil, fmt.Errorf("SWAS client not initialized")
	}

	request := &swas20200601.ListInstancesRequest{
		RegionId:    tea.String(ac.config.RegionID),
		InstanceIds: tea.String(fmt.Sprintf(`["%s"]`, instanceID)),
	}

	response, err := ac.SwasClient.ListInstances(request)
	if err != nil {
		return nil, fmt.Errorf("failed to describe SWAS instance: %v", err)
	}

	if response.Body.Instances == nil {
		return nil, fmt.Errorf("SWAS instance %s not found", instanceID)
	}

	instance := response.Body.Instances[0]
	info := &InstanceInfo{
		InstanceID:   tea.StringValue(instance.InstanceId),
		InstanceName: tea.StringValue(instance.InstanceName),
		Status:       tea.StringValue(instance.Status),
		Provider:     "Aliyun",
		Region:       ac.config.RegionID,
	}

	// 获取公网IP
	if instance.PublicIpAddress != nil {
		info.PublicIP = tea.StringValue(instance.PublicIpAddress)
	}

	// 获取私网IP
	if instance.InnerIpAddress != nil {
		info.PrivateIP = tea.StringValue(instance.InnerIpAddress)
	}

	return info, nil
}

func (ac *AliyunClient) CreateFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
	switch ac.config.Type {
	case 0:
		// ECS 云服务器
		return ac.createECSFirewallRule(instanceID, rule)
	case 1:
		// SWAS 轻量应用服务器
		return ac.createSWASFirewallRule(instanceID, rule)
	default:
		return nil, fmt.Errorf("unsupported Aliyun client type: %d", ac.config.Type)
	}
}

// createECSFirewallRule 创建ECS防火墙规则
func (ac *AliyunClient) createECSFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
	// 获取实例的安全组
	securityGroupId, err := ac.getInstanceSecurityGroup(instanceID)
	if err != nil {
		return nil, err
	}

	// 解析端口范围
	portRange, err := ac.parsePortRange(rule.Port, rule.Protocol)
	if err != nil {
		return nil, fmt.Errorf("invalid port range: %v", err)
	}

	return ac.createSingleRule(instanceID, securityGroupId, rule, portRange)
}

// createSWASFirewallRule 创建SWAS防火墙规则
func (ac *AliyunClient) createSWASFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
	if ac.SwasClient == nil {
		return nil, fmt.Errorf("SWAS client not initialized")
	}

	// 检查是否已存在相同的规则
	existingRules, err := ac.checkFirewallRuleExists(instanceID, rule, rule.Port)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}

	// 如果存在相同的规则
	if existingRules != nil && len(*existingRules) > 0 {
		// 检查是否有IP相同的规则
		var matched = false
		for _, existingRule := range *existingRules {
			if existingRule.CidrBlock == rule.CidrBlock {
				// 如果IP相同，直接返回现有规则
				matched = true
			} else {
				matched = false
			}
		}

		if matched {
			return &(*existingRules)[0], nil
		}

		// 如果IP不同，批量删除所有旧规则
		var rulesToDelete []*FirewallRuleResult
		for _, existingRule := range *existingRules {
			rulesToDelete = append(rulesToDelete, &existingRule)
		}
		err = ac.deleteSWASFirewallRules(instanceID, rulesToDelete)
		if err != nil {
			return nil, fmt.Errorf("failed to delete existing rules before creating new one: %v", err)
		}
	}

	portRange, err := ac.parseSWASPortRange(rule.Port, rule.Protocol)
	rule.Protocol = ac.convertSWASProtocol(rule.Protocol)
	if err != nil {
		return nil, fmt.Errorf("invalid SWAS port range: %v", err)
	}

	var firewallrules []*swas20200601.CreateFirewallRulesRequestFirewallRules
	// 创建多个数组项
	for _, pr := range portRange {
		firewallrule := &swas20200601.CreateFirewallRulesRequestFirewallRules{
			Port:         tea.String(pr),
			RuleProtocol: tea.String(strings.ToUpper(rule.Protocol)),
			SourceCidrIp: tea.String(rule.CidrBlock),
			Remark:       tea.String(rule.Description),
		}
		firewallrules = append(firewallrules, firewallrule)
	}

	request := &swas20200601.CreateFirewallRulesRequest{
		RegionId:      tea.String(ac.config.RegionID),
		InstanceId:    tea.String(instanceID),
		FirewallRules: firewallrules,
	}

	_, err = ac.SwasClient.CreateFirewallRules(request)
	if err != nil {
		return nil, fmt.Errorf("failed to create SWAS firewall rule: %v", err)
	}

	// 返回创建的规则信息
	return &FirewallRuleResult{
		Port:        rule.Port,
		Protocol:    rule.Protocol,
		CidrBlock:   rule.CidrBlock,
		Action:      rule.Action,
		Description: rule.Description,
		Provider:    "Aliyun",
		InstanceID:  instanceID,
	}, nil
}

// checkFirewallRuleExists 检查防火墙规则是否已存在
func (ac *AliyunClient) checkFirewallRuleExists(instanceID string, rule *FirewallRuleSpec, portRange string) (*[]FirewallRuleResult, error) {
	// 获取现有规则列表
	existingRules, err := ac.ListFirewallRules(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing rules: %v", err)
	}

	// 将portRange直接转换为map
	expectedPortsMap := ac.convertPortRange(portRange)

	// 检查是否存在相同的规则
	var matchedRules []FirewallRuleResult
	for _, existing := range existingRules {
		if existing.Protocol == strings.ToUpper(rule.Protocol) &&
			expectedPortsMap[existing.Port] &&
			existing.Description == rule.Description {
			matchedRules = append(matchedRules, *existing)
		}
	}

	if len(matchedRules) > 0 {
		return &matchedRules, nil
	}

	return nil, nil
}

// createSingleRule 创建单个防火墙规则
func (ac *AliyunClient) createSingleRule(instanceID, securityGroupId string, rule *FirewallRuleSpec, portRange string) (*FirewallRuleResult, error) {
	// 检查是否已存在相同的规则
	existingRules, err := ac.checkFirewallRuleExists(instanceID, rule, portRange)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}

	// 如果存在相同的规则
	if existingRules != nil && len(*existingRules) > 0 {
		// 检查是否有IP相同的规则
		for _, existingRule := range *existingRules {
			if existingRule.CidrBlock == rule.CidrBlock {
				return &existingRule, nil
			}
		}

		// 如果IP不同，删除所有旧规则
		// logger.Printf("Rule exists with different IP, deleting old rules first")
		for _, existingRule := range *existingRules {
			err = ac.DeleteFirewallRuleBySpec(instanceID, &existingRule)
			if err != nil {
				return nil, fmt.Errorf("failed to delete existing rule before creating new one: %v", err)
			}
		}
		// logger.Printf("Successfully deleted existing rules with old IP")
	}

	// 创建新规则
	request := &ecs20140526.AuthorizeSecurityGroupRequest{
		RegionId:        tea.String(ac.config.RegionID),
		SecurityGroupId: tea.String(securityGroupId),
		IpProtocol:      tea.String(strings.ToLower(rule.Protocol)),
		PortRange:       tea.String(portRange),
		SourceCidrIp:    tea.String(rule.CidrBlock),
		Policy:          tea.String(strings.ToLower(rule.Action)),
		Description:     tea.String(rule.Description),
	}

	_, err = ac.EcsClient.AuthorizeSecurityGroup(request)
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall rule: %v", err)
	}

	// 返回创建的规则信息
	return &FirewallRuleResult{
		Port:        rule.Port,
		Protocol:    rule.Protocol,
		CidrBlock:   rule.CidrBlock,
		Action:      rule.Action,
		Description: rule.Description,
		Provider:    "Aliyun",
		InstanceID:  instanceID,
	}, nil
}

// Deprecated: Use DeleteFirewallRuleBySpec instead
func (ac *AliyunClient) DeleteFirewallRule(instanceID, ruleID string) error {
	return fmt.Errorf("DeleteFirewallRule is deprecated, please use DeleteFirewallRuleBySpec instead")
}

// 通过规则规格删除防火墙规则
func (ac *AliyunClient) DeleteFirewallRuleBySpec(instanceID string, rule *FirewallRuleResult) error {
	switch ac.config.Type {
	case 0:
		// ECS 云服务器
		return ac.deleteECSFirewallRule(instanceID, rule)
	case 1:
		// SWAS 轻量应用服务器
		return ac.deleteSWASFirewallRule(instanceID, rule)
	default:
		return fmt.Errorf("unsupported Aliyun client type: %d", ac.config.Type)
	}
}

// deleteECSFirewallRule 删除ECS防火墙规则
func (ac *AliyunClient) deleteECSFirewallRule(instanceID string, rule *FirewallRuleResult) error {
	// 获取实例的安全组
	securityGroupId, err := ac.getInstanceSecurityGroup(instanceID)
	if err != nil {
		return err
	}

	// 解析端口范围
	portRange, err := ac.parsePortRange(rule.Port, rule.Protocol)
	if err != nil {
		return fmt.Errorf("invalid port range: %v", err)
	}

	request := &ecs20140526.RevokeSecurityGroupRequest{
		RegionId:        tea.String(ac.config.RegionID),
		SecurityGroupId: tea.String(securityGroupId),
		IpProtocol:      tea.String(strings.ToLower(rule.Protocol)),
		PortRange:       tea.String(portRange),
		SourceCidrIp:    tea.String(rule.CidrBlock),
	}

	_, err = ac.EcsClient.RevokeSecurityGroup(request)
	if err != nil {
		// 检查是否是规则不存在的错误
		errStr := err.Error()
		if strings.Contains(errStr, "InvalidSecurityGroupRule.NotFound") ||
			strings.Contains(errStr, "The specified security group rule does not exist") ||
			strings.Contains(errStr, "rule does not exist") {
			// 规则已经不存在，认为删除成功
			// logger.Printf("Security group rule already deleted: %s", rule.Description)
			return nil
		}
		return fmt.Errorf("failed to delete ECS firewall rule: %v", err)
	}

	return nil
}

// deleteSWASFirewallRule 删除SWAS防火墙规则
func (ac *AliyunClient) deleteSWASFirewallRule(instanceID string, rule *FirewallRuleResult) error {
	if ac.SwasClient == nil {
		return fmt.Errorf("SWAS client not initialized")
	}

	// 获取SWAS防火墙规则ID
	ruleID, err := ac.findSWASRuleID(instanceID, rule)
	if err != nil {
		return fmt.Errorf("failed to find SWAS rule ID: %v", err)
	}

	if ruleID == "" {
		// 规则不存在，认为删除成功
		return nil
	}

	// 使用批量删除接口，即使只有一个规则也用数组
	request := &swas20200601.DeleteFirewallRulesRequest{
		InstanceId: tea.String(instanceID),
		RegionId:   tea.String(ac.config.RegionID),
		RuleIds:    []*string{tea.String(ruleID)},
	}

	_, err = ac.SwasClient.DeleteFirewallRules(request)
	if err != nil {
		// 检查是否是规则不存在的错误
		errStr := err.Error()
		if strings.Contains(errStr, "NotFound") ||
			strings.Contains(errStr, "does not exist") {
			// 规则已经不存在，认为删除成功
			return nil
		}
		return fmt.Errorf("failed to delete SWAS firewall rule: %v", err)
	}

	return nil
}

// deleteSWASFirewallRules 批量删除SWAS防火墙规则
func (ac *AliyunClient) deleteSWASFirewallRules(instanceID string, rules []*FirewallRuleResult) error {
	if ac.SwasClient == nil {
		return fmt.Errorf("SWAS client not initialized")
	}

	if len(rules) == 0 {
		return nil
	}

	// 使用批量查找方法收集所有规则的ID
	ruleIDStrings, err := ac.findSWASRuleIDs(instanceID, rules)
	if err != nil {
		return fmt.Errorf("failed to find SWAS rule IDs: %v", err)
	}

	// 过滤掉空的规则ID
	var ruleIDs []*string
	for _, ruleID := range ruleIDStrings {
		if ruleID != "" {
			ruleIDs = append(ruleIDs, tea.String(ruleID))
		}
	}

	// 如果没有找到任何规则ID，认为删除成功
	if len(ruleIDs) == 0 {
		return nil
	}

	// 批量删除规则
	request := &swas20200601.DeleteFirewallRulesRequest{
		InstanceId: tea.String(instanceID),
		RegionId:   tea.String(ac.config.RegionID),
		RuleIds:    ruleIDs,
	}

	_, err = ac.SwasClient.DeleteFirewallRules(request)
	if err != nil {
		// 检查是否是规则不存在的错误
		errStr := err.Error()
		if strings.Contains(errStr, "NotFound") ||
			strings.Contains(errStr, "does not exist") {
			// 规则已经不存在，认为删除成功
			return nil
		}
		return fmt.Errorf("failed to delete SWAS firewall rules: %v", err)
	}

	return nil
}

// findSWASRuleIDs 查找匹配的SWAS防火墙规则ID数组
func (ac *AliyunClient) findSWASRuleIDs(instanceID string, targetRules []*FirewallRuleResult) ([]string, error) {
	if ac.SwasClient == nil {
		return nil, fmt.Errorf("SWAS client not initialized")
	}

	if len(targetRules) == 0 {
		return []string{}, nil
	}

	request := &swas20200601.ListFirewallRulesRequest{
		InstanceId: tea.String(instanceID),
		PageSize:   tea.Int32(100),
	}

	response, err := ac.SwasClient.ListFirewallRules(request)
	if err != nil {
		return nil, fmt.Errorf("failed to list SWAS firewall rules: %v", err)
	}

	var ruleIDs []string

	// 为每个目标规则查找匹配的规则ID
	for _, targetRule := range targetRules {
		// 解析目标规则的端口范围
		targetPortRange, err := ac.parseSWASPortRange(targetRule.Port, targetRule.Protocol)
		if err != nil {
			return nil, fmt.Errorf("invalid target port range for rule %s: %v", targetRule.Description, err)
		}

		// 查找匹配的规则
		found := false
		if response.Body.FirewallRules != nil {
			for _, rule := range response.Body.FirewallRules {
				if rule.RuleProtocol != nil && rule.Port != nil && rule.SourceCidrIp != nil {
					// 比较协议（忽略大小写）
					if !strings.EqualFold(*rule.RuleProtocol, targetRule.Protocol) {
						continue
					}

					// 比较端口
					if *rule.Port != targetPortRange[0] {
						continue
					}

					// 比较CIDR
					if *rule.SourceCidrIp != targetRule.CidrBlock {
						continue
					}

					// 找到匹配的规则，添加RuleId
					if rule.RuleId != nil {
						ruleIDs = append(ruleIDs, *rule.RuleId)
						found = true
						break
					}
				}
			}
		}

		// 如果没有找到匹配的规则，添加空字符串
		if !found {
			ruleIDs = append(ruleIDs, "")
		}
	}

	return ruleIDs, nil
}

// findSWASRuleID 查找匹配的SWAS防火墙规则ID（保持兼容性的单个规则版本）
func (ac *AliyunClient) findSWASRuleID(instanceID string, targetRule *FirewallRuleResult) (string, error) {
	ruleIDs, err := ac.findSWASRuleIDs(instanceID, []*FirewallRuleResult{targetRule})
	if err != nil {
		return "", err
	}

	if len(ruleIDs) > 0 {
		return ruleIDs[0], nil
	}

	return "", nil
}

func (ac *AliyunClient) ListFirewallRules(instanceID string) ([]*FirewallRuleResult, error) {
	switch ac.config.Type {
	case 0:
		// ECS 云服务器
		return ac.listECSFirewallRules(instanceID)
	case 1:
		// SWAS 轻量应用服务器
		return ac.listSWASFirewallRules(instanceID)
	default:
		return nil, fmt.Errorf("unsupported Aliyun client type: %d", ac.config.Type)
	}
}

// listECSFirewallRules 获取ECS防火墙规则列表
func (ac *AliyunClient) listECSFirewallRules(instanceID string) ([]*FirewallRuleResult, error) {
	// 获取实例的安全组
	securityGroupId, err := ac.getInstanceSecurityGroup(instanceID)
	if err != nil {
		return nil, err
	}

	request := &ecs20140526.DescribeSecurityGroupAttributeRequest{
		RegionId:        tea.String(ac.config.RegionID),
		SecurityGroupId: tea.String(securityGroupId),
	}

	response, err := ac.EcsClient.DescribeSecurityGroupAttribute(request)
	if err != nil {
		return nil, fmt.Errorf("failed to list ECS firewall rules: %v", err)
	}

	var rules []*FirewallRuleResult
	if response.Body.Permissions != nil {
		for _, permission := range response.Body.Permissions.Permission {
			// 转换端口范围，取第一个值作为显示
			portRanges := ac.convertPortRange(tea.StringValue(permission.PortRange))
			port := tea.StringValue(permission.PortRange) // 使用原始格式
			if len(portRanges) > 0 {
				// 从map中取第一个key作为显示端口
				for firstPort := range portRanges {
					port = firstPort
					break
				}
			}

			rule := &FirewallRuleResult{
				Port:        port,
				Protocol:    strings.ToUpper(tea.StringValue(permission.IpProtocol)),
				CidrBlock:   tea.StringValue(permission.SourceCidrIp),
				Action:      strings.ToUpper(tea.StringValue(permission.Policy)),
				Description: tea.StringValue(permission.Description),
				Provider:    "Aliyun",
				InstanceID:  instanceID,
			}
			rules = append(rules, rule)
		}
	}

	return rules, nil
}

// listSWASFirewallRules 获取SWAS防火墙规则列表
func (ac *AliyunClient) listSWASFirewallRules(instanceID string) ([]*FirewallRuleResult, error) {
	if ac.SwasClient == nil {
		return nil, fmt.Errorf("SWAS client not initialized")
	}

	request := &swas20200601.ListFirewallRulesRequest{
		InstanceId: tea.String(instanceID),
		PageSize:   tea.Int32(100), // 设置最大分页数量
	}

	response, err := ac.SwasClient.ListFirewallRules(request)
	if err != nil {
		return nil, fmt.Errorf("failed to list SWAS firewall rules: %v", err)
	}

	var rules []*FirewallRuleResult
	if response.Body.FirewallRules != nil {
		for _, rule := range response.Body.FirewallRules {
			result := &FirewallRuleResult{
				Port:        tea.StringValue(rule.Port),
				Protocol:    strings.ToUpper(tea.StringValue(rule.RuleProtocol)),
				CidrBlock:   tea.StringValue(rule.SourceCidrIp),
				Action:      "ACCEPT",
				Description: tea.StringValue(rule.Remark),
				Provider:    "Aliyun",
				InstanceID:  instanceID,
			}
			rules = append(rules, result)
		}
	}

	return rules, nil
}

// getInstanceSecurityGroup 获取实例的安全组ID
func (ac *AliyunClient) getInstanceSecurityGroup(instanceID string) (string, error) {
	// 如果配置中指定了安全组ID，直接使用
	if ac.config.SecurityGroupIds != "" {
		// 取第一个安全组ID
		sgIds := strings.Split(ac.config.SecurityGroupIds, ",")
		return strings.TrimSpace(sgIds[0]), nil
	}

	// 否则从实例信息中获取
	request := &ecs20140526.DescribeInstancesRequest{
		RegionId:    tea.String(ac.config.RegionID),
		InstanceIds: tea.String(fmt.Sprintf(`["%s"]`, instanceID)),
	}

	response, err := ac.EcsClient.DescribeInstances(request)
	if err != nil {
		return "", fmt.Errorf("failed to describe instance: %v", err)
	}

	if response.Body.Instances == nil || len(response.Body.Instances.Instance) == 0 {
		return "", fmt.Errorf("instance %s not found", instanceID)
	}

	instance := response.Body.Instances.Instance[0]
	if instance.SecurityGroupIds == nil || len(instance.SecurityGroupIds.SecurityGroupId) == 0 {
		return "", fmt.Errorf("no security group found for instance %s", instanceID)
	}

	return tea.StringValue(instance.SecurityGroupIds.SecurityGroupId[0]), nil
}

// parsePortRange 解析端口范围
func (ac *AliyunClient) parsePortRange(port, protocol string) (string, error) {
	port = strings.TrimSpace(port)
	protocol = strings.ToUpper(strings.TrimSpace(protocol))

	// ICMP、GRE、ALL协议使用-1/-1
	switch protocol {
	case "ICMP", "GRE", "ALL":
		return "-1/-1", nil
	}

	// TCP和UDP协议处理端口
	if protocol == "TCP" || protocol == "UDP" {
		// 处理特殊值
		switch strings.ToUpper(port) {
		case "ALL", "":
			return "1/65535", nil
		}

		// 检查是否是逗号分隔的多个端口，阿里云接口限制
		if strings.Contains(port, ",") {
			// 多端口暂时不支持，返回错误或使用第一个端口
			return "", fmt.Errorf("multiple ports separated by comma are not supported in single rule, please create separate rules for each port: %s", port)
		}

		// 检查是否是端口范围（如 8000-9000）
		if strings.Contains(port, "-") {
			parts := strings.Split(port, "-")
			if len(parts) != 2 {
				return "", fmt.Errorf("invalid port range format: %s", port)
			}

			startPort, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return "", fmt.Errorf("invalid start port: %s", parts[0])
			}

			endPort, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return "", fmt.Errorf("invalid end port: %s", parts[1])
			}

			if startPort < 1 || startPort > 65535 || endPort < 1 || endPort > 65535 {
				return "", fmt.Errorf("port must be between 1 and 65535")
			}

			if startPort > endPort {
				return "", fmt.Errorf("start port must be less than or equal to end port")
			}

			return fmt.Sprintf("%d/%d", startPort, endPort), nil
		}

		// 单个端口
		portNum, err := strconv.Atoi(port)
		if err != nil {
			return "", fmt.Errorf("invalid port number: %s", port)
		}

		if portNum < 1 || portNum > 65535 {
			return "", fmt.Errorf("port must be between 1 and 65535")
		}

		return fmt.Sprintf("%d/%d", portNum, portNum), nil
	}

	// 其他协议默认返回-1/-1
	return "-1/-1", nil
}

// parseSWASPortRange 解析SWAS端口范围
func (ac *AliyunClient) parseSWASPortRange(port, protocol string) ([]string, error) {
	port = strings.TrimSpace(port)
	protocol = strings.ToUpper(strings.TrimSpace(protocol))

	// ICMP、GRE、ALL协议使用-1/-1
	switch protocol {
	case "ICMP":
		return []string{"-1/-1"}, nil
	case "ALL":
		return []string{"1/65535"}, nil
	}
	if protocol == "TCP" || protocol == "UDP" {
		// 处理特殊值
		switch strings.ToUpper(port) {
		case "ALL", "-1/-1":
			return []string{"1/65535"}, nil
		}
		// 检查是否为逗号分隔的多个端口
		if strings.Contains(port, ",") {
			ports := strings.Split(port, ",")
			var portRanges []string
			for _, p := range ports {
				portRange, err := ac.parseSWASPortRange(p, protocol)
				if err != nil {
					return nil, err
				}
				portRanges = append(portRanges, portRange...)
			}
			return portRanges, nil
		}
		// 检查是否是/分隔的端口范围
		if strings.Contains(port, "/") {
			parts := strings.Split(port, "/")
			if len(parts) != 2 {
				return []string{}, fmt.Errorf("invalid port range format: %s", port)
			}
			if parts[0] == parts[1] {
				return []string{parts[0]}, nil // 阿里云轻量不支持斜杠左右相等，转换为单个端口
			}
		}
	}
	return []string{port}, nil
}

// convertSWASProtocol 转换SWAS协议格式
func (ac *AliyunClient) convertSWASProtocol(protocol string) string {
	// 阿里云轻量云不支持ALL协议，自动转换为TCP+UDP
	switch strings.ToUpper(protocol) {
	case "ALL":
		return "TCP+UDP"
	case "TCP", "UDP", "ICMP":
		return protocol
	default:
		return protocol
	}
}

// convertPortRange 将阿里云的端口范围格式转换为map
func (ac *AliyunClient) convertPortRange(portRange string) map[string]bool {
	result := make(map[string]bool)

	// 多端口
	if strings.Contains(portRange, ",") {
		ports := strings.Split(portRange, ",")
		for _, p := range ports {
			subMap := ac.convertPortRange(p)
			for port := range subMap {
				result[port] = true
			}
		}
		return result
	}

	// ICMP、GRE、ALL协议返回-1/-1时转换为ALL
	if portRange == "-1/-1" {
		result["ALL"] = true
		return result
	}

	// TCP/UDP全端口范围
	if portRange == "1/65535" {
		result["ALL"] = true
		return result
	}

	parts := strings.Split(portRange, "/")
	if len(parts) != 2 {
		result[portRange] = true
		return result
	}

	if parts[0] == parts[1] {
		result[parts[0]] = true // 单个端口
		return result
	}

	result[fmt.Sprintf("%s-%s", parts[0], parts[1])] = true // 端口范围
	return result
}

// ValidateSecurityGroup 验证安全组是否存在并返回详细信息
func (ac *AliyunClient) ValidateSecurityGroup(securityGroupId string) (*SecurityGroupInfo, error) {
	request := &ecs20140526.DescribeSecurityGroupAttributeRequest{
		RegionId:        tea.String(ac.config.RegionID),
		SecurityGroupId: tea.String(securityGroupId),
	}

	response, err := ac.EcsClient.DescribeSecurityGroupAttribute(request)
	if err != nil {
		return nil, fmt.Errorf("安全组 %s 不存在或无权限访问: %v", securityGroupId, err)
	}

	// 构造安全组信息
	sgInfo := &SecurityGroupInfo{
		SecurityGroupID: securityGroupId,
		Name:            tea.StringValue(response.Body.SecurityGroupName),
		Description:     tea.StringValue(response.Body.Description),
		VpcID:           tea.StringValue(response.Body.VpcId),
		Region:          ac.config.RegionID,
		RulesCount:      0,
	}

	// 计算规则数量
	if response.Body.Permissions != nil {
		sgInfo.RulesCount = len(response.Body.Permissions.Permission)
	}

	return sgInfo, nil
}

package cloud

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

type AzureConfig struct {
	SubscriptionID    string `json:"subscription_id" binding:"required"`
	TenantID          string `json:"tenant_id" binding:"required"`
	ClientID          string `json:"client_id" binding:"required"`
	ClientSecret      string `json:"client_secret" binding:"required"`
	ResourceGroupName string `json:"resource_group_name" binding:"required"`
	SecurityGroupName string `json:"security_group_name" binding:"required"`
	Location          string `json:"location"` // 区域，如 "eastus"
}

type AzureClient struct {
	config             AzureConfig
	nsgClient          *armnetwork.SecurityGroupsClient
	securityRuleClient *armnetwork.SecurityRulesClient
	ctx                context.Context
}

// NewAzureClient 创建 Azure 客户端
func NewAzureClient(config AzureConfig) (*AzureClient, error) {
	// 验证配置
	if config.SubscriptionID == "" || config.TenantID == "" || config.ClientID == "" || config.ClientSecret == "" {
		return nil, fmt.Errorf("subscription_id, tenant_id, client_id and client_secret are required")
	}

	if config.ResourceGroupName == "" || config.SecurityGroupName == "" {
		return nil, fmt.Errorf("resource_group_name and security_group_name are required")
	}

	if config.Location == "" {
		config.Location = "eastus" // 默认美国东部
	}

	// 创建认证凭据
	cred, err := azidentity.NewClientSecretCredential(
		config.TenantID,
		config.ClientID,
		config.ClientSecret,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credentials: %v", err)
	}

	// 创建网络安全组客户端
	nsgClient, err := armnetwork.NewSecurityGroupsClient(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create network security groups client: %v", err)
	}

	// 创建安全规则客户端
	securityRuleClient, err := armnetwork.NewSecurityRulesClient(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create security rules client: %v", err)
	}

	return &AzureClient{
		config:             config,
		nsgClient:          nsgClient,
		securityRuleClient: securityRuleClient,
		ctx:                context.Background(),
	}, nil
}

// GetConfig 获取 Azure 客户端配置
func (azc *AzureClient) GetConfig() AzureConfig {
	return azc.config
}

// 实现 CloudProvider 接口
func (azc *AzureClient) GetInstance(instanceID string) (*InstanceInfo, error) {
	// Azure 中实例信息获取需要虚拟机客户端，这里简化处理
	// 如果配置中有安全组名称，说明实例是可访问的
	if azc.config.SecurityGroupName == "" {
		return nil, fmt.Errorf("instance %s requires security group name in configuration", instanceID)
	}

	info := &InstanceInfo{
		InstanceID:   instanceID,
		InstanceName: instanceID,
		Status:       "running",
		Provider:     "Azure",
		Region:       azc.config.Location,
	}

	return info, nil
}

func (azc *AzureClient) CreateFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
	// 检查是否已存在相同的规则
	existingRule, err := azc.checkFirewallRuleExists(instanceID, rule)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing rules: %v", err)
	}

	// 如果存在相同的规则
	if existingRule != nil {
		// 如果 IP 相同，直接返回现有规则
		if existingRule.CidrBlock == rule.CidrBlock {
			return existingRule, nil
		}

		// 如果 IP 不同，先删除旧规则
		err = azc.DeleteFirewallRuleBySpec(instanceID, existingRule)
		if err != nil {
			return nil, fmt.Errorf("failed to delete existing rule before creating new one: %v", err)
		}
	}

	// 生成规则名称（使用描述作为规则名称的一部分）
	ruleName := azc.generateRuleName(rule)

	// 解析端口范围
	destPortRange, err := azc.parsePortRange(rule.Port, rule.Protocol)
	if err != nil {
		return nil, fmt.Errorf("invalid port range: %v", err)
	}

	// 获取下一个可用的优先级
	priority, err := azc.getNextAvailablePriority(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get next available priority: %v", err)
	}

	// 创建安全规则
	securityRule := armnetwork.SecurityRule{
		Name: to.Ptr(ruleName),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 azc.convertProtocol(rule.Protocol),
			SourcePortRange:          to.Ptr("*"),
			DestinationPortRange:     destPortRange,
			SourceAddressPrefix:      to.Ptr(rule.CidrBlock),
			DestinationAddressPrefix: to.Ptr("*"),
			Access:                   azc.convertAction(rule.Action),
			Priority:                 to.Ptr(priority),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
			Description:              to.Ptr(rule.Description),
		},
	}

	// 创建规则
	poller, err := azc.securityRuleClient.BeginCreateOrUpdate(
		azc.ctx,
		azc.config.ResourceGroupName,
		azc.config.SecurityGroupName,
		ruleName,
		securityRule,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create security rule: %v", err)
	}

	_, err = poller.PollUntilDone(azc.ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for security rule creation: %v", err)
	}

	result := &FirewallRuleResult{
		Port:        rule.Port,
		Protocol:    rule.Protocol,
		CidrBlock:   rule.CidrBlock,
		Action:      rule.Action,
		Description: rule.Description,
		Provider:    "Azure",
		InstanceID:  instanceID,
	}

	return result, nil
}

func (azc *AzureClient) DeleteFirewallRuleBySpec(instanceID string, rule *FirewallRuleResult) error {
	// 查找要删除的规则名称
	ruleName, err := azc.findSecurityRuleName(instanceID, rule)
	if err != nil {
		return err
	}

	if ruleName == "" {
		// 规则不存在，可能已被删除
		return nil
	}

	// 删除安全规则
	poller, err := azc.securityRuleClient.BeginDelete(
		azc.ctx,
		azc.config.ResourceGroupName,
		azc.config.SecurityGroupName,
		ruleName,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to delete security rule: %v", err)
	}

	_, err = poller.PollUntilDone(azc.ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to wait for security rule deletion: %v", err)
	}

	return nil
}

func (azc *AzureClient) UpdateFirewallRule(instanceID string, ruleSpec *FirewallRuleSpec, newIP string) (*FirewallRuleResult, error) {
	// 先查询现有规则
	existingRules, err := azc.ListFirewallRules(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing rules: %v", err)
	}

	// 查找匹配的规则
	var targetRule *FirewallRuleResult
	for _, rule := range existingRules {
		if rule.Protocol == ruleSpec.Protocol &&
			rule.Port == ruleSpec.Port &&
			rule.Description == ruleSpec.Description {
			targetRule = rule
			break
		}
	}

	if targetRule == nil {
		// 规则不存在，直接创建新规则
		newRule := *ruleSpec
		newRule.CidrBlock = fmt.Sprintf("%s/32", newIP)
		return azc.CreateFirewallRule(instanceID, &newRule)
	}

	// 检查 IP 是否已经一致
	newCidrBlock := fmt.Sprintf("%s/32", newIP)
	if targetRule.CidrBlock == newCidrBlock {
		return targetRule, nil
	}

	// 删除旧规则
	err = azc.DeleteFirewallRuleBySpec(instanceID, targetRule)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old rule: %v", err)
	}

	// 创建新规则
	newRule := *ruleSpec
	newRule.CidrBlock = newCidrBlock
	return azc.CreateFirewallRule(instanceID, &newRule)
}

func (azc *AzureClient) ListFirewallRules(instanceID string) ([]*FirewallRuleResult, error) {
	// 获取安全组
	nsg, err := azc.nsgClient.Get(
		azc.ctx,
		azc.config.ResourceGroupName,
		azc.config.SecurityGroupName,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get network security group: %v", err)
	}

	var rules []*FirewallRuleResult
	if nsg.Properties != nil && nsg.Properties.SecurityRules != nil {
		for _, rule := range nsg.Properties.SecurityRules {
			// 只处理入方向规则
			if rule.Properties != nil &&
				rule.Properties.Direction != nil &&
				*rule.Properties.Direction == armnetwork.SecurityRuleDirectionInbound {

				port := azc.convertPortRangeBack(rule.Properties.DestinationPortRange)
				protocol := azc.convertProtocolBack(rule.Properties.Protocol)
				cidrBlock := ""
				if rule.Properties.SourceAddressPrefix != nil {
					cidrBlock = *rule.Properties.SourceAddressPrefix
				}
				action := azc.convertActionBack(rule.Properties.Access)
				description := ""
				if rule.Properties.Description != nil {
					description = *rule.Properties.Description
				}

				result := &FirewallRuleResult{
					Port:        port,
					Protocol:    protocol,
					CidrBlock:   cidrBlock,
					Action:      action,
					Description: description,
					Provider:    "Azure",
					InstanceID:  instanceID,
				}
				rules = append(rules, result)
			}
		}
	}

	return rules, nil
}

// 辅助方法

// generateRuleName 生成规则名称
func (azc *AzureClient) generateRuleName(rule *FirewallRuleSpec) string {
	// 使用协议-端口-描述的组合生成规则名称
	// 名称必须以字母或数字开头，以字母、数字或下划线结尾，并且只能包含字母、数字、下划线、句点或连字符。
	name := fmt.Sprintf("FireFlow_%s_%s",
		strings.ToUpper(rule.Protocol),
		strings.ReplaceAll(rule.Port, "/", "-"))

	// 限制名称长度
	if len(name) > 60 {
		name = name[:60]
	}

	return name
}

// parsePortRange 解析端口范围
func (azc *AzureClient) parsePortRange(port, protocol string) (*string, error) {
	port = strings.TrimSpace(port)
	protocol = strings.ToUpper(strings.TrimSpace(protocol))

	// ICMP、GRE、ALL 协议使用 *
	switch protocol {
	case "ICMP", "GRE", "ALL":
		return to.Ptr("*"), nil
	}

	// TCP 和 UDP 协议处理端口
	if protocol == "TCP" || protocol == "UDP" {
		// 处理特殊值
		switch strings.ToUpper(port) {
		case "ALL", "":
			return to.Ptr("*"), nil
		}

		// 检查是否是逗号分隔的多个端口，azure支持逗号分隔多个端口，但为了简化处理，这里不支持
		if strings.Contains(port, ",") {
			return nil, fmt.Errorf("multiple ports separated by comma are not supported, please create separate rules: %s", port)
		}

		// 检查是否是端口范围（如 8000-9000）
		if strings.Contains(port, "-") {
			parts := strings.Split(port, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid port range format: %s", port)
			}

			startPort, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid start port: %s", parts[0])
			}

			endPort, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid end port: %s", parts[1])
			}

			if startPort < 1 || startPort > 65535 || endPort < 1 || endPort > 65535 {
				return nil, fmt.Errorf("port must be between 1 and 65535")
			}

			if startPort > endPort {
				return nil, fmt.Errorf("start port must be less than or equal to end port")
			}

			return to.Ptr(port), nil
		}

		// 单个端口
		portNum, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid port number: %s", port)
		}

		if portNum < 1 || portNum > 65535 {
			return nil, fmt.Errorf("port must be between 1 and 65535")
		}

		return to.Ptr(port), nil
	}

	// 其他协议使用 *
	return to.Ptr("*"), nil
}

// convertProtocol 转换协议为 Azure 格式
func (azc *AzureClient) convertProtocol(protocol string) *armnetwork.SecurityRuleProtocol {
	switch strings.ToUpper(protocol) {
	case "TCP":
		return to.Ptr(armnetwork.SecurityRuleProtocolTCP)
	case "UDP":
		return to.Ptr(armnetwork.SecurityRuleProtocolUDP)
	case "ICMP":
		return to.Ptr(armnetwork.SecurityRuleProtocolIcmp)
	case "ALL":
		return to.Ptr(armnetwork.SecurityRuleProtocolAsterisk)
	default:
		return to.Ptr(armnetwork.SecurityRuleProtocolTCP)
	}
}

// convertProtocolBack 将 Azure 协议格式转换回通用格式
func (azc *AzureClient) convertProtocolBack(protocol *armnetwork.SecurityRuleProtocol) string {
	if protocol == nil {
		return "ALL"
	}

	switch *protocol {
	case armnetwork.SecurityRuleProtocolTCP:
		return "TCP"
	case armnetwork.SecurityRuleProtocolUDP:
		return "UDP"
	case armnetwork.SecurityRuleProtocolIcmp:
		return "ICMP"
	case armnetwork.SecurityRuleProtocolAsterisk:
		return "ALL"
	default:
		return strings.ToUpper(string(*protocol))
	}
}

// convertAction 转换动作为 Azure 格式
func (azc *AzureClient) convertAction(action string) *armnetwork.SecurityRuleAccess {
	switch strings.ToUpper(action) {
	case "ACCEPT", "ALLOW":
		return to.Ptr(armnetwork.SecurityRuleAccessAllow)
	case "DROP", "DENY":
		return to.Ptr(armnetwork.SecurityRuleAccessDeny)
	default:
		return to.Ptr(armnetwork.SecurityRuleAccessAllow)
	}
}

// convertActionBack 将 Azure 动作格式转换回通用格式
func (azc *AzureClient) convertActionBack(access *armnetwork.SecurityRuleAccess) string {
	if access == nil {
		return "ACCEPT"
	}

	switch *access {
	case armnetwork.SecurityRuleAccessAllow:
		return "ACCEPT"
	case armnetwork.SecurityRuleAccessDeny:
		return "DROP"
	default:
		return "ACCEPT"
	}
}

// convertPortRangeBack 将 Azure 端口范围转换回通用格式
func (azc *AzureClient) convertPortRangeBack(portRange *string) string {
	if portRange == nil || *portRange == "*" {
		return "ALL"
	}
	return *portRange
}

// getNextAvailablePriority 获取下一个可用的优先级
func (azc *AzureClient) getNextAvailablePriority(instanceID string) (int32, error) {
	// 获取现有规则
	nsg, err := azc.nsgClient.Get(
		azc.ctx,
		azc.config.ResourceGroupName,
		azc.config.SecurityGroupName,
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to get network security group: %v", err)
	}

	// Azure 优先级范围：100-4096
	// 从 1000 开始分配，给系统规则预留空间
	maxPriority := int32(1000)

	if nsg.Properties != nil && nsg.Properties.SecurityRules != nil {
		for _, rule := range nsg.Properties.SecurityRules {
			if rule.Properties != nil && rule.Properties.Priority != nil {
				if *rule.Properties.Priority >= maxPriority {
					maxPriority = *rule.Properties.Priority + 1
				}
			}
		}
	}

	// 确保不超过最大值
	if maxPriority > 4096 {
		return 0, fmt.Errorf("no available priority (max priority exceeded)")
	}

	return maxPriority, nil
}

// findSecurityRuleName 查找安全规则名称
func (azc *AzureClient) findSecurityRuleName(instanceID string, rule *FirewallRuleResult) (string, error) {
	nsg, err := azc.nsgClient.Get(
		azc.ctx,
		azc.config.ResourceGroupName,
		azc.config.SecurityGroupName,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get network security group: %v", err)
	}

	if nsg.Properties != nil && nsg.Properties.SecurityRules != nil {
		for _, sgRule := range nsg.Properties.SecurityRules {
			if azc.matchRule(sgRule, rule) && sgRule.Name != nil {
				return *sgRule.Name, nil
			}
		}
	}

	return "", nil
}

// matchRule 检查安全规则是否匹配
func (azc *AzureClient) matchRule(sgRule *armnetwork.SecurityRule, rule *FirewallRuleResult) bool {
	if sgRule.Properties == nil {
		return false
	}

	// 检查方向（只处理入方向）
	if sgRule.Properties.Direction == nil || *sgRule.Properties.Direction != armnetwork.SecurityRuleDirectionInbound {
		return false
	}

	// 检查协议
	if azc.convertProtocolBack(sgRule.Properties.Protocol) != strings.ToUpper(rule.Protocol) {
		return false
	}

	// 检查端口
	port := azc.convertPortRangeBack(sgRule.Properties.DestinationPortRange)
	if port != rule.Port {
		return false
	}

	// 检查 CIDR
	if sgRule.Properties.SourceAddressPrefix == nil || *sgRule.Properties.SourceAddressPrefix != rule.CidrBlock {
		return false
	}

	// 检查描述
	if sgRule.Properties.Description != nil && rule.Description != "" {
		if *sgRule.Properties.Description != rule.Description {
			return false
		}
	}

	return true
}

// checkFirewallRuleExists 检查防火墙规则是否存在
func (azc *AzureClient) checkFirewallRuleExists(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
	existingRules, err := azc.ListFirewallRules(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing rules: %v", err)
	}

	// 检查是否存在相同的规则
	for _, existing := range existingRules {
		if existing.Protocol == strings.ToUpper(rule.Protocol) &&
			existing.Port == rule.Port &&
			existing.Description == rule.Description {
			return existing, nil
		}
	}

	return nil, nil
}

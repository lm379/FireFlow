package cloud

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	vpc "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpc/v3"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpc/v3/model"
	vpcregion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpc/v3/region"
)

type HuaweiConfig struct {
	Region          string `json:"region"`
	AK              string `json:"ak"`
	SK              string `json:"sk"`
	ProjectID       string `json:"project_id"`
	SecurityGroupID string `json:"security_group_id"`
}

type HuaweiClient struct {
	config    *HuaweiConfig
	vpcClient *vpc.VpcClient
}

func NewHuaweiClient(config *HuaweiConfig) (*HuaweiClient, error) {
	if config.AK == "" || config.SK == "" {
		return nil, fmt.Errorf("AK and SK are required")
	}

	if config.Region == "" {
		config.Region = "cn-north-4" // 默认华北-北京四
	}

	// 创建认证信息
	auth := basic.NewCredentialsBuilder().
		WithAk(config.AK).
		WithSk(config.SK).
		WithProjectId(config.ProjectID).
		Build()

	// 获取VPC区域信息
	vpcRegion, err := vpcregion.SafeValueOf(config.Region)
	if err != nil {
		return nil, fmt.Errorf("invalid VPC region %s: %v", config.Region, err)
	}

	// 创建默认HTTP配置
	httpConfig := config.DefaultHttpConfig().
		WithIgnoreSSLVerification(false).
		WithTimeout(120 * time.Second)

	// 创建VPC客户端
	vpcClient := vpc.NewVpcClient(
		vpc.VpcClientBuilder().
			WithRegion(vpcRegion).
			WithCredential(auth).
			WithHttpConfig(httpConfig).
			Build())

	return &HuaweiClient{
		config:    config,
		vpcClient: vpcClient,
	}, nil
}

// 实现 CloudProvider 接口
func (hc *HuaweiClient) GetInstance(instanceID string) (*InstanceInfo, error) {
	// 华为云VPC SDK主要用于安全组操作，实例信息获取可以简化
	// 如果配置中有安全组ID，说明实例是可访问的
	if hc.config.SecurityGroupID == "" {
		return nil, fmt.Errorf("instance %s requires security group ID in configuration", instanceID)
	}

	info := &InstanceInfo{
		InstanceID:   instanceID,
		InstanceName: instanceID, // 使用实例ID作为名称
		Status:       "running",  // 假设状态为运行中
		Provider:     "HuaweiCloud",
		Region:       hc.config.Region,
		// PublicIP和PrivateIP在安全组操作中不是必需的
	}

	return info, nil
}

func (hc *HuaweiClient) CreateFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
	// 检查是否已存在相同的规则
	existingRule, err := hc.checkFirewallRuleExists(instanceID, rule)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing rules: %v", err)
	}

	// 如果存在相同的规则
	if existingRule != nil {
		// 如果IP相同，直接返回现有规则
		if existingRule.CidrBlock == rule.CidrBlock {
			if existingRule.Description != rule.Description {
				log.Printf("Rule already exists with same IP but different description (Protocol=%s, Port=%s, CidrBlock=%s). Existing description: %s, New description: %s. Consider updating the rule instead.",
					existingRule.Protocol, existingRule.Port, existingRule.CidrBlock, existingRule.Description, rule.Description)
			} else {
				log.Printf("Rule already exists with same IP (Protocol=%s, Port=%s, CidrBlock=%s), skipping creation",
					existingRule.Protocol, existingRule.Port, existingRule.CidrBlock)
			}
			return existingRule, nil
		}

		// 如果IP不同，先删除旧规则
		log.Printf("Rule exists with different IP (current: %s, new: %s), deleting old rule first",
			existingRule.CidrBlock, rule.CidrBlock)
		err = hc.DeleteFirewallRuleBySpec(instanceID, existingRule)
		if err != nil {
			return nil, fmt.Errorf("failed to delete existing rule before creating new one: %v", err)
		}
	}

	// 获取安全组ID
	securityGroupID, err := hc.getInstanceSecurityGroup(instanceID)
	if err != nil {
		return nil, err
	}

	// 解析端口范围
	portStr, err := hc.parsePortRangeForVPC(rule.Port, rule.Protocol)
	if err != nil {
		return nil, fmt.Errorf("invalid port range: %v", err)
	}

	// 创建安全组规则
	direction := "ingress" // 入方向
	ethertype := "IPv4"

	request := &model.CreateSecurityGroupRuleRequest{
		Body: &model.CreateSecurityGroupRuleRequestBody{
			SecurityGroupRule: &model.CreateSecurityGroupRuleOption{
				Direction:       direction,
				Ethertype:       &ethertype,
				Protocol:        hc.convertProtocol(rule.Protocol),
				Multiport:       portStr,
				RemoteIpPrefix:  &rule.CidrBlock,
				SecurityGroupId: securityGroupID,
				Description:     &rule.Description,
			},
		},
	}

	response, err := hc.vpcClient.CreateSecurityGroupRule(request)
	if err != nil {
		return nil, fmt.Errorf("failed to create security group rule: %v", err)
	}

	if response.SecurityGroupRule == nil {
		return nil, fmt.Errorf("failed to create security group rule: response is nil")
	}

	result := &FirewallRuleResult{
		Port:        rule.Port,
		Protocol:    rule.Protocol,
		CidrBlock:   rule.CidrBlock,
		Action:      rule.Action,
		Description: rule.Description,
		Provider:    "HuaweiCloud",
		InstanceID:  instanceID,
	}

	log.Printf("Successfully created Huawei Cloud security group rule: %+v", result)
	return result, nil
}

func (hc *HuaweiClient) DeleteFirewallRuleBySpec(instanceID string, rule *FirewallRuleResult) error {
	// 获取安全组ID
	securityGroupID, err := hc.getInstanceSecurityGroup(instanceID)
	if err != nil {
		return err
	}

	// 查找要删除的规则
	ruleID, err := hc.findSecurityGroupRuleID(securityGroupID, rule)
	if err != nil {
		return err
	}

	if ruleID == "" {
		// 规则不存在，可能已被删除
		log.Printf("Security group rule not found, may have been already deleted: %s", rule.Description)
		return nil
	}

	// 删除安全组规则
	request := &model.DeleteSecurityGroupRuleRequest{
		SecurityGroupRuleId: ruleID,
	}

	_, err = hc.vpcClient.DeleteSecurityGroupRule(request)
	if err != nil {
		return fmt.Errorf("failed to delete security group rule: %v", err)
	}

	log.Printf("Deleted Huawei Cloud security group rule (proto:%s, port:%s, cidr:%s) for instance %s",
		rule.Protocol, rule.Port, rule.CidrBlock, instanceID)
	return nil
}

func (hc *HuaweiClient) UpdateFirewallRule(instanceID string, ruleSpec *FirewallRuleSpec, newIP string) (*FirewallRuleResult, error) {
	log.Printf("Updating Huawei Cloud firewall rule for instance %s with new IP %s", instanceID, newIP)
	log.Printf("Rule spec: Protocol=%s, Port=%s, Description=%s", ruleSpec.Protocol, ruleSpec.Port, ruleSpec.Description)

	// 先查询现有规则，通过描述匹配规则
	existingRules, err := hc.ListFirewallRules(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing rules: %v", err)
	}

	port, err := hc.parsePortRangeForVPC(ruleSpec.Port, ruleSpec.Protocol)
	if err != nil {
		return nil, fmt.Errorf("invalid port range: %v", err)
	}

	// 查找匹配的规则
	var targetRule *FirewallRuleResult
	for _, rule := range existingRules {
		if rule.Protocol == ruleSpec.Protocol && rule.Port == *port {
			if ruleSpec.Port == "ALL" && rule.Description != ruleSpec.Description {
				log.Printf("Found rule with matching protocol and port but different description (Protocol=%s, Port=%s). Existing description: %s, Target description: %s. Proceeding with update.",
					rule.Protocol, rule.Port, rule.Description, ruleSpec.Description)
			}
			targetRule = rule
			// log.Printf("Found target rule: Description=%s, CidrBlock=%s", rule.Description, rule.CidrBlock)
			break
		}
	}

	if targetRule == nil {
		// 规则不存在，可能已被手动删除，直接创建新规则
		log.Printf("Rule with description '%s' not found in cloud, creating new rule", ruleSpec.Description)
		newRule := *ruleSpec
		newRule.CidrBlock = fmt.Sprintf("%s/32", newIP)
		return hc.CreateFirewallRule(instanceID, &newRule)
	}

	// 检查IP是否已经一致
	newCidrBlock := fmt.Sprintf("%s/32", newIP)
	if targetRule.CidrBlock == newCidrBlock {
		log.Printf("Rule with description '%s' already has the correct IP %s, skipping update", ruleSpec.Description, newIP)
		return targetRule, nil
	}

	log.Printf("IP mismatch detected: current=%s, target=%s, proceeding with update", targetRule.CidrBlock, newCidrBlock)

	err = hc.DeleteFirewallRuleBySpec(instanceID, targetRule)
	if err != nil {
		// 即使删除失败，也记录日志并继续创建新规则
		log.Printf("Warning: failed to delete old rule with description '%s': %v", ruleSpec.Description, err)
	}

	// 更新CIDR块为新IP
	newRule := *ruleSpec
	newRule.CidrBlock = newCidrBlock

	return hc.CreateFirewallRule(instanceID, &newRule)
}

func (hc *HuaweiClient) ListFirewallRules(instanceID string) ([]*FirewallRuleResult, error) {
	// 获取安全组ID
	securityGroupID, err := hc.getInstanceSecurityGroup(instanceID)
	if err != nil {
		return nil, err
	}

	// 查询安全组规则
	sgIds := []string{securityGroupID}
	request := &model.ListSecurityGroupRulesRequest{
		SecurityGroupId: &sgIds,
	}

	response, err := hc.vpcClient.ListSecurityGroupRules(request)
	if err != nil {
		return nil, fmt.Errorf("failed to list security group rules: %v", err)
	}

	var rules []*FirewallRuleResult
	if response.SecurityGroupRules != nil {
		for _, rule := range *response.SecurityGroupRules {
			// 只处理入方向规则
			if rule.Direction == "ingress" {
				result := &FirewallRuleResult{
					Port:        rule.Multiport,
					Protocol:    hc.convertProtocolBack(&rule.Protocol),
					CidrBlock:   rule.RemoteIpPrefix,
					Action:      "ACCEPT", // 华为云安全组规则默认是允许
					Description: rule.Description,
					Provider:    "HuaweiCloud",
					InstanceID:  instanceID,
				}
				rules = append(rules, result)
			}
		}
	}

	return rules, nil
}

// getInstanceSecurityGroup 获取实例的安全组ID
func (hc *HuaweiClient) getInstanceSecurityGroup(instanceID string) (string, error) {
	// 华为云VPC操作必须指定安全组ID
	if hc.config.SecurityGroupID != "" {
		return hc.config.SecurityGroupID, nil
	}

	return "", fmt.Errorf("security group ID is required for instance %s in Huawei Cloud configuration", instanceID)
}

// parsePortRangeForVPC 解析端口范围为VPC格式
func (hc *HuaweiClient) parsePortRangeForVPC(port, protocol string) (*string, error) {
	port = strings.TrimSpace(port)
	protocol = strings.ToUpper(strings.TrimSpace(protocol))

	// ICMP、GRE、ALL协议不需要端口
	switch protocol {
	case "ICMP", "GRE", "ALL":
		return nil, nil
	}

	// TCP和UDP协议处理端口
	if protocol == "TCP" || protocol == "UDP" {
		// 处理特殊值
		switch strings.ToUpper(port) {
		case "ALL", "":
			portStr := "1-65535"
			return &portStr, nil
		}

		// 检查是否是逗号分隔的多个端口
		if strings.Contains(port, ",") {
			// VPC v3支持不连续端口，直接返回
			return &port, nil
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

			return &port, nil
		}

		// 单个端口
		portNum, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid port number: %s", port)
		}

		if portNum < 1 || portNum > 65535 {
			return nil, fmt.Errorf("port must be between 1 and 65535")
		}

		return &port, nil
	}

	// 其他协议不需要端口
	return nil, nil
}

// convertProtocol 将协议转换为华为云格式
func (hc *HuaweiClient) convertProtocol(protocol string) *string {
	switch strings.ToUpper(protocol) {
	case "TCP":
		return func() *string { s := "tcp"; return &s }()
	case "UDP":
		return func() *string { s := "udp"; return &s }()
	case "ICMP":
		return func() *string { s := "icmp"; return &s }()
	case "ALL":
		return nil // 所有协议时不指定协议
	default:
		return func() *string { s := strings.ToLower(protocol); return &s }()
	}
}

// convertProtocolBack 将华为云协议格式转换回通用格式
func (hc *HuaweiClient) convertProtocolBack(protocol *string) string {
	if protocol == nil {
		return "ALL"
	}
	return strings.ToUpper(*protocol)
}

// convertPortRange 将华为云的端口范围转换为通用格式
// func (hc *HuaweiClient) convertPortRangeBack(portMin, portMax *int32) string {
// 	if portMin == nil || portMax == nil {
// 		return "ALL"
// 	}

// 	if *portMin == 1 && *portMax == 65535 {
// 		return "ALL"
// 	}

// 	if *portMin == *portMax {
// 		return fmt.Sprintf("%d", *portMin)
// 	}

// 	return fmt.Sprintf("%d-%d", *portMin, *portMax)
// }

// findSecurityGroupRuleID 查找安全组规则ID
func (hc *HuaweiClient) findSecurityGroupRuleID(securityGroupID string, rule *FirewallRuleResult) (string, error) {
	sgIds := []string{securityGroupID}
	request := &model.ListSecurityGroupRulesRequest{
		SecurityGroupId: &sgIds,
	}

	response, err := hc.vpcClient.ListSecurityGroupRules(request)
	if err != nil {
		return "", fmt.Errorf("failed to list security group rules: %v", err)
	}

	if response.SecurityGroupRules != nil {
		for _, sgRule := range *response.SecurityGroupRules {
			// 只处理入方向规则，通过协议、端口范围、CIDR和描述匹配规则
			if sgRule.Direction == "ingress" && hc.matchRule(sgRule, rule) {
				return sgRule.Id, nil
			}
		}
	}

	return "", nil
}

// matchRule 检查安全组规则是否匹配
func (hc *HuaweiClient) matchRule(sgRule model.SecurityGroupRule, rule *FirewallRuleResult) bool {
	// 检查协议
	if hc.convertProtocolBack(&sgRule.Protocol) != strings.ToUpper(rule.Protocol) {
		return false
	}

	// 检查端口范围
	if sgRule.Multiport != rule.Port {
		return false
	}

	// 检查CIDR
	if sgRule.RemoteIpPrefix != rule.CidrBlock {
		return false
	}

	// 检查描述
	if sgRule.Description != rule.Description {
		return false
	}

	return true
}

// checkFirewallRuleExists 检查防火墙规则是否存在
func (hc *HuaweiClient) checkFirewallRuleExists(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
	existingRules, err := hc.ListFirewallRules(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing rules: %v", err)
	}

	// 检查是否存在相同的规则
	for _, existing := range existingRules {

		// 华为云仅允许一条端口为ALL的规则
		if rule.Port == "ALL" && existing.Port == "1-65535" {
			return existing, nil
		}

		existing.Port = hc.convertProtocolBack(&existing.Protocol)
		// 华为云仅允许一条协议为ALL的规则
		if rule.Protocol == "ALL" && existing.Protocol == "ALL" {
			return existing, nil
		}

		if existing.Protocol == strings.ToUpper(rule.Protocol) &&
			existing.Port == rule.Port &&
			existing.Description == rule.Description {
			return existing, nil
		}
	}

	return nil, nil
}

// DefaultHttpConfig 默认HTTP配置
func (cfg *HuaweiConfig) DefaultHttpConfig() *config.HttpConfig {
	return config.DefaultHttpConfig().
		WithIgnoreSSLVerification(true).
		WithTimeout(300 * time.Second)
}

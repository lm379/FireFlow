package cloud

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	ecs20140526 "github.com/alibabacloud-go/ecs-20140526/v7/client"
	"github.com/alibabacloud-go/tea/tea"
)

type AliyunConfig struct {
	AccessKeyID      string `json:"accessKeyId" binding:"required"`
	AccessKeySecret  string `json:"accessKeySecret" binding:"required"`
	RegionID         string `json:"regionId" binding:"required"`
	SecurityGroupIds string `json:"securityGroupIds" binding:"required"`
}

type AliyunClient struct {
	EcsClient *ecs20140526.Client
	config    AliyunConfig
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

	log.Printf("Initializing Aliyun ECS client with AccessKeyID: %s, Region: %s",
		maskAliyunSecretId(config.AccessKeyID), config.RegionID)

	// 创建客户端配置
	clientConfig := &openapi.Config{
		AccessKeyId:     tea.String(config.AccessKeyID),
		AccessKeySecret: tea.String(config.AccessKeySecret),
	}
	// 设置地域
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
}

// GetConfig 获取阿里云客户端配置
func (ac *AliyunClient) GetConfig() AliyunConfig {
	return ac.config
}

// maskAliyunSecretId 隐藏密钥ID的敏感部分
func maskAliyunSecretId(secretId string) string {
	if len(secretId) <= 8 {
		return "****"
	}
	return secretId[:4] + "****" + secretId[len(secretId)-4:]
}

// 实现 CloudProvider 接口
func (ac *AliyunClient) GetInstance(instanceID string) (*InstanceInfo, error) {
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

func (ac *AliyunClient) CreateFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
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

// createSingleRule 创建单个防火墙规则
func (ac *AliyunClient) createSingleRule(instanceID, securityGroupId string, rule *FirewallRuleSpec, portRange string) (*FirewallRuleResult, error) {
	request := &ecs20140526.AuthorizeSecurityGroupRequest{
		RegionId:        tea.String(ac.config.RegionID),
		SecurityGroupId: tea.String(securityGroupId),
		IpProtocol:      tea.String(strings.ToLower(rule.Protocol)),
		PortRange:       tea.String(portRange),
		SourceCidrIp:    tea.String(rule.CidrBlock),
		Policy:          tea.String(strings.ToLower(rule.Action)),
		Description:     tea.String(rule.Description),
	}

	_, err := ac.EcsClient.AuthorizeSecurityGroup(request)
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall rule: %v", err)
	}

	// 返回创建的规则信息
	return &FirewallRuleResult{
		RuleID:      fmt.Sprintf("%s|%s|%s|%s", securityGroupId, rule.Protocol, portRange, rule.CidrBlock),
		Port:        rule.Port,
		Protocol:    rule.Protocol,
		CidrBlock:   rule.CidrBlock,
		Action:      rule.Action,
		Description: rule.Description,
		Provider:    "Aliyun",
		InstanceID:  instanceID,
	}, nil
}

func (ac *AliyunClient) DeleteFirewallRule(instanceID, ruleID string) error {
	// 解析规则ID获取安全组和规则信息
	parts := strings.Split(ruleID, "|")
	if len(parts) != 4 {
		return fmt.Errorf("invalid rule ID format, expected format: securityGroupId|protocol|portRange|cidrBlock, got: %s", ruleID)
	}

	securityGroupId := parts[0]
	protocol := parts[1]
	portRange := parts[2]
	cidrBlock := parts[3]

	request := &ecs20140526.RevokeSecurityGroupRequest{
		RegionId:        tea.String(ac.config.RegionID),
		SecurityGroupId: tea.String(securityGroupId),
		IpProtocol:      tea.String(strings.ToLower(protocol)),
		PortRange:       tea.String(portRange),
		SourceCidrIp:    tea.String(cidrBlock),
	}

	_, err := ac.EcsClient.RevokeSecurityGroup(request)
	if err != nil {
		// 检查是否是规则不存在的错误
		errStr := err.Error()
		if strings.Contains(errStr, "InvalidSecurityGroupRule.NotFound") ||
			strings.Contains(errStr, "The specified security group rule does not exist") ||
			strings.Contains(errStr, "rule does not exist") {
			// 规则已经不存在，认为删除成功
			log.Printf("Security group rule already deleted: %s", ruleID)
			return nil
		}
		return fmt.Errorf("failed to delete firewall rule: %v", err)
	}

	return nil
}

func (ac *AliyunClient) UpdateFirewallRule(instanceID, ruleID string, ruleSpec *FirewallRuleSpec, newIP string) (*FirewallRuleResult, error) {
	// 阿里云安全组规则不支持直接更新，需要先删除再创建
	err := ac.DeleteFirewallRule(instanceID, ruleID)
	if err != nil {
		// 即使删除失败，也记录日志并继续创建新规则
		// 因为规则可能已经在云端被手动删除了
		log.Printf("Warning: failed to delete old rule %s: %v", ruleID, err)
	}

	// 更新CIDR块为新IP
	newRule := *ruleSpec
	newRule.CidrBlock = fmt.Sprintf("%s/32", newIP)

	return ac.CreateFirewallRule(instanceID, &newRule)
}

func (ac *AliyunClient) ListFirewallRules(instanceID string) ([]*FirewallRuleResult, error) {
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
		return nil, fmt.Errorf("failed to list firewall rules: %v", err)
	}

	var rules []*FirewallRuleResult
	if response.Body.Permissions != nil {
		for _, permission := range response.Body.Permissions.Permission {
			rule := &FirewallRuleResult{
				RuleID: fmt.Sprintf("%s|%s|%s|%s",
					securityGroupId,
					tea.StringValue(permission.IpProtocol),
					tea.StringValue(permission.PortRange),
					tea.StringValue(permission.SourceCidrIp)),
				Port:        ac.convertPortRange(tea.StringValue(permission.PortRange)),
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

// 辅助方法

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

// convertPortRange 将阿里云的端口范围格式转换为通用格式
func (ac *AliyunClient) convertPortRange(portRange string) string {
	// ICMP、GRE、ALL协议返回-1/-1时转换为ALL
	if portRange == "-1/-1" {
		return "ALL"
	}

	// TCP/UDP全端口范围
	if portRange == "1/65535" {
		return "ALL"
	}

	parts := strings.Split(portRange, "/")
	if len(parts) != 2 {
		return portRange
	}

	if parts[0] == parts[1] {
		return parts[0] // 单个端口
	}

	return fmt.Sprintf("%s-%s", parts[0], parts[1]) // 端口范围
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

// SecurityGroupInfo 安全组信息结构体
type SecurityGroupInfo struct {
	SecurityGroupID string `json:"security_group_id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	VpcID           string `json:"vpc_id"`
	Region          string `json:"region"`
	RulesCount      int    `json:"rules_count"`
}

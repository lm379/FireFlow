package cloud

import (
	"fmt"
	"log"
	"strings"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	lighthouse "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/lighthouse/v20200324"
)

type TencentConfig struct {
	SecretId   string `json:"secretId"`
	SecretKey  string `json:"secretKey"`
	Region     string `json:"region"`
	InstanceId string `json:"instanceId"` // 实例ID
}

type TencentClient struct {
	config           TencentConfig
	lighthouseClient *lighthouse.Client
}

// CloudProvider 接口定义
type CloudProvider interface {
	// 获取实例信息
	GetInstance(instanceID string) (*InstanceInfo, error)

	// 创建防火墙规则
	CreateFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error)

	// 删除防火墙规则
	DeleteFirewallRuleBySpec(instanceID string, rule *FirewallRuleResult) error

	// 更新防火墙规则 - 通过规则规格匹配，返回更新后的规则信息
	UpdateFirewallRule(instanceID string, ruleSpec *FirewallRuleSpec, newIP string) (*FirewallRuleResult, error)

	// 获取防火墙规则列表
	ListFirewallRules(instanceID string) ([]*FirewallRuleResult, error)
}

// 实例信息
type InstanceInfo struct {
	InstanceID   string `json:"instance_id"`
	InstanceName string `json:"instance_name"`
	Status       string `json:"status"`
	PublicIP     string `json:"public_ip"`
	PrivateIP    string `json:"private_ip"`
	Provider     string `json:"provider"`
	Region       string `json:"region"`
}

// 防火墙规则规格
type FirewallRuleSpec struct {
	Port          string `json:"port"`                      // 端口或端口范围，如 "80" 或 "8000-9000"
	Protocol      string `json:"protocol"`                  // 协议：TCP/UDP/ICMP
	CidrBlock     string `json:"cidr_block"`                // CIDR块，如 "0.0.0.0/0"
	Ipv6CidrBlock string `json:"ipv6_cidr_block,omitempty"` // IPv6 CIDR块，如 "::/0" 和 CIDR块二选一
	Action        string `json:"action"`                    // 动作：ACCEPT/DROP
	Description   string `json:"description"`               // 备注
}

// 防火墙规则结果
type FirewallRuleResult struct {
	Port        string `json:"port"`
	Protocol    string `json:"protocol"`
	CidrBlock   string `json:"cidr_block"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Provider    string `json:"provider"`
	InstanceID  string `json:"instance_id"`
}

func NewTencentClient(config TencentConfig) (*TencentClient, error) {
	// 验证配置
	if config.SecretId == "" || config.SecretKey == "" {
		return nil, fmt.Errorf("secret_id and secret_key are required")
	}

	if config.Region == "" {
		config.Region = "ap-beijing" // 默认北京区域
	}

	// log.Printf("Initializing Tencent Cloud client with SecretId: %s, Region: %s",
	// 	maskSecretId(config.SecretId), config.Region)

	// 创建认证信息
	credential := common.NewCredential(config.SecretId, config.SecretKey)

	// 初始化Lighthouse客户端
	cpfLighthouse := profile.NewClientProfile()
	cpfLighthouse.HttpProfile.Endpoint = "lighthouse.tencentcloudapi.com"
	lighthouseClient, err := lighthouse.NewClient(credential, config.Region, cpfLighthouse)
	if err != nil {
		return nil, fmt.Errorf("failed to create Lighthouse client: %v", err)
	}

	return &TencentClient{
		config:           config,
		lighthouseClient: lighthouseClient,
	}, nil
}

// 实现 CloudProvider 接口
func (tc *TencentClient) GetInstance(instanceID string) (*InstanceInfo, error) {
	// 从Lighthouse获取实例信息
	if info, err := tc.getLighthouseInstance(instanceID); err == nil {
		return info, nil
	}

	return nil, fmt.Errorf("instance %s not found in Lighthouse", instanceID)
}

func (tc *TencentClient) CreateFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
	return tc.createLighthouseFirewallRule(instanceID, rule)
}

func (tc *TencentClient) DeleteFirewallRuleBySpec(instanceID string, rule *FirewallRuleResult) error {
	return tc.deleteLighthouseFirewallRuleBySpec(instanceID, rule)
}

func (tc *TencentClient) UpdateFirewallRule(instanceID string, ruleSpec *FirewallRuleSpec, newIP string) (*FirewallRuleResult, error) {
	// 构建新的规则规格，使用新的IP
	newRuleSpec := &FirewallRuleSpec{
		Protocol:    ruleSpec.Protocol,
		Port:        ruleSpec.Port,
		CidrBlock:   fmt.Sprintf("%s/32", newIP),
		Action:      ruleSpec.Action,
		Description: ruleSpec.Description,
	}

	// 直接调用CreateFirewallRule，它已经包含了检查现有规则和处理IP变动的逻辑
	return tc.CreateFirewallRule(instanceID, newRuleSpec)
}

func (tc *TencentClient) ListFirewallRules(instanceID string) ([]*FirewallRuleResult, error) {
	return tc.listLighthouseFirewallRules(instanceID)
}

// Lighthouse 相关实现
func (tc *TencentClient) getLighthouseInstance(instanceID string) (*InstanceInfo, error) {
	request := lighthouse.NewDescribeInstancesRequest()
	request.InstanceIds = common.StringPtrs([]string{instanceID})

	response, err := tc.lighthouseClient.DescribeInstances(request)
	if err != nil {
		return nil, fmt.Errorf("failed to describe Lighthouse instance: %v", err)
	}

	if len(response.Response.InstanceSet) == 0 {
		return nil, fmt.Errorf("lighthouse instance %s not found", instanceID)
	}

	instance := response.Response.InstanceSet[0]
	info := &InstanceInfo{
		InstanceID:   *instance.InstanceId,
		InstanceName: *instance.InstanceName,
		Status:       *instance.InstanceState,
		Provider:     "TencentCloud",
		Region:       tc.config.Region,
	}

	if len(instance.PublicAddresses) > 0 {
		info.PublicIP = *instance.PublicAddresses[0]
	}
	if len(instance.PrivateAddresses) > 0 {
		info.PrivateIP = *instance.PrivateAddresses[0]
	}

	return info, nil
}

// 检查规则是否存在
func (tc *TencentClient) checkLighthouseFirewallRuleExists(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
	// 获取现有规则列表
	existingRules, err := tc.listLighthouseFirewallRules(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing rules: %v", err)
	}

	// 检查是否存在相同的规则
	var existingRule *FirewallRuleResult
	for _, existing := range existingRules {
		if existing.Protocol == strings.ToUpper(rule.Protocol) &&
			existing.Port == rule.Port &&
			existing.Description == rule.Description {
			existingRule = existing
			// log.Printf("Found existing rule: Protocol=%s, Port=%s, CidrBlock=%s, Description=%s", existing.Protocol, existing.Port, existing.CidrBlock, existing.Description)
			break
		}
	}

	if existingRule != nil {
		// log.Printf("Found existing rule: %+v", existingRule)
		return existingRule, nil
	}

	return nil, nil
}

// 创建Lighthouse防火墙规则
func (tc *TencentClient) createLighthouseFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
	// 检查是否已存在相同的规则
	existingRule, err := tc.checkLighthouseFirewallRuleExists(instanceID, rule)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}

	// 如果存在相同的规则
	if existingRule != nil {
		// 如果IP相同，直接返回现有规则
		if existingRule.CidrBlock == rule.CidrBlock {
			log.Printf("Rule already exists with same IP (Protocol=%s, Port=%s, CidrBlock=%s), skipping creation",
				existingRule.Protocol, existingRule.Port, existingRule.CidrBlock)
			return existingRule, nil
		}

		// 如果IP不同，先删除旧规则
		log.Printf("Rule exists with different IP (current: %s, new: %s), deleting old rule first",
			existingRule.CidrBlock, rule.CidrBlock)
		err = tc.deleteLighthouseFirewallRuleBySpec(instanceID, existingRule)
		if err != nil {
			return nil, fmt.Errorf("failed to delete existing rule before creating new one: %v", err)
		}
		// log.Printf("Successfully deleted existing rule with old IP")
	}

	// 创建新规则
	request := lighthouse.NewCreateFirewallRulesRequest()
	request.InstanceId = common.StringPtr(instanceID)

	// 构建防火墙规则
	firewallRule := &lighthouse.FirewallRule{
		Protocol:                common.StringPtr(strings.ToUpper(rule.Protocol)),
		Port:                    common.StringPtr(rule.Port),
		CidrBlock:               common.StringPtr(rule.CidrBlock),
		Action:                  common.StringPtr(strings.ToUpper(rule.Action)),
		FirewallRuleDescription: common.StringPtr(rule.Description),
	}

	request.FirewallRules = []*lighthouse.FirewallRule{firewallRule}

	_, err = tc.lighthouseClient.CreateFirewallRules(request)
	if err != nil {
		if sdkError, ok := err.(*errors.TencentCloudSDKError); ok {
			return nil, fmt.Errorf("TencentCloud API Error: Code=%s, Message=%s",
				sdkError.Code, sdkError.Message)
		}
		return nil, fmt.Errorf("failed to create Lighthouse firewall rule: %v", err)
	}

	result := &FirewallRuleResult{
		Port:        rule.Port,
		Protocol:    rule.Protocol,
		CidrBlock:   rule.CidrBlock,
		Action:      rule.Action,
		Description: rule.Description,
		Provider:    "TencentCloud",
		InstanceID:  instanceID,
	}

	log.Printf("Successfully created Lighthouse firewall rule: %+v", result)
	return result, nil
}

// 根据规则规格删除防火墙规则
func (tc *TencentClient) deleteLighthouseFirewallRuleBySpec(instanceID string, rule *FirewallRuleResult) error {
	request := lighthouse.NewDeleteFirewallRulesRequest()
	request.InstanceId = common.StringPtr(instanceID)

	// 构建要删除的规则
	firewallRule := &lighthouse.FirewallRule{
		Protocol:                common.StringPtr(rule.Protocol),
		Port:                    common.StringPtr(rule.Port),
		CidrBlock:               common.StringPtr(rule.CidrBlock),
		Action:                  common.StringPtr(rule.Action),
		FirewallRuleDescription: common.StringPtr(rule.Description),
	}

	request.FirewallRules = []*lighthouse.FirewallRule{firewallRule}

	_, err := tc.lighthouseClient.DeleteFirewallRules(request)
	if err != nil {
		if sdkError, ok := err.(*errors.TencentCloudSDKError); ok {
			return fmt.Errorf("TencentCloud API Error: Code=%s, Message=%s",
				sdkError.Code, sdkError.Message)
		}
		return fmt.Errorf("failed to delete Lighthouse firewall rule: %v", err)
	}

	log.Printf("Deleted Lighthouse firewall rule (proto:%s, port:%s, cidr:%s) for instance %s",
		rule.Protocol, rule.Port, rule.CidrBlock, instanceID)
	return nil
}

func (tc *TencentClient) listLighthouseFirewallRules(instanceID string) ([]*FirewallRuleResult, error) {
	request := lighthouse.NewDescribeFirewallRulesRequest()
	request.InstanceId = common.StringPtr(instanceID)

	response, err := tc.lighthouseClient.DescribeFirewallRules(request)
	if err != nil {
		if sdkError, ok := err.(*errors.TencentCloudSDKError); ok {
			return nil, fmt.Errorf("TencentCloud API Error: Code=%s, Message=%s",
				sdkError.Code, sdkError.Message)
		}
		return nil, fmt.Errorf("failed to list Lighthouse firewall rules: %v", err)
	}

	var results []*FirewallRuleResult
	for _, rule := range response.Response.FirewallRuleSet {
		result := &FirewallRuleResult{
			Port:        *rule.Port,
			Protocol:    *rule.Protocol,
			CidrBlock:   *rule.CidrBlock,
			Action:      *rule.Action,
			Description: *rule.FirewallRuleDescription,
			Provider:    "TencentCloud",
			InstanceID:  instanceID,
		}
		results = append(results, result)
	}

	return results, nil
}

// 掩码SecretId用于日志输出
// func maskSecretId(secretId string) string {
// 	if len(secretId) <= 8 {
// 		return "****"
// 	}
// 	return secretId[:4] + "****" + secretId[len(secretId)-4:]
// }

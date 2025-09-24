package cloud

import (
	"crypto/md5"
	"fmt"
	"log"
	"strings"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
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
	cvmClient        *cvm.Client
	lighthouseClient *lighthouse.Client
}

// CloudProvider 接口定义
type CloudProvider interface {
	// 获取实例信息
	GetInstance(instanceID string) (*InstanceInfo, error)

	// 创建防火墙规则
	CreateFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error)

	// 删除防火墙规则
	DeleteFirewallRule(instanceID, ruleID string) error

	// 更新防火墙规则 - 通过规则规格匹配，返回更新后的规则信息
	UpdateFirewallRule(instanceID, ruleID string, ruleSpec *FirewallRuleSpec, newIP string) (*FirewallRuleResult, error)

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
	RuleID      string `json:"rule_id"`
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

	log.Printf("Initializing Tencent Cloud client with SecretId: %s, Region: %s",
		maskSecretId(config.SecretId), config.Region)

	// 创建认证信息
	credential := common.NewCredential(config.SecretId, config.SecretKey)

	// 创建客户端配置
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "cvm.tencentcloudapi.com"

	// 初始化CVM客户端
	cvmClient, err := cvm.NewClient(credential, config.Region, cpf)
	if err != nil {
		return nil, fmt.Errorf("failed to create CVM client: %v", err)
	}

	// 初始化Lighthouse客户端
	cpfLighthouse := profile.NewClientProfile()
	cpfLighthouse.HttpProfile.Endpoint = "lighthouse.tencentcloudapi.com"
	lighthouseClient, err := lighthouse.NewClient(credential, config.Region, cpfLighthouse)
	if err != nil {
		return nil, fmt.Errorf("failed to create Lighthouse client: %v", err)
	}

	return &TencentClient{
		config:           config,
		cvmClient:        cvmClient,
		lighthouseClient: lighthouseClient,
	}, nil
}

// 实现 CloudProvider 接口
func (tc *TencentClient) GetInstance(instanceID string) (*InstanceInfo, error) {
	// 先尝试从CVM获取实例信息
	if info, err := tc.getCVMInstance(instanceID); err == nil {
		return info, nil
	}

	// 如果CVM中没有找到，尝试从Lighthouse获取
	if info, err := tc.getLighthouseInstance(instanceID); err == nil {
		return info, nil
	}

	return nil, fmt.Errorf("instance %s not found in CVM or Lighthouse", instanceID)
}

func (tc *TencentClient) CreateFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
	// 先判断是CVM还是Lighthouse实例
	if tc.isCVMInstance(instanceID) {
		return nil, fmt.Errorf("CVM firewall rule management not implemented yet")
		// return tc.createCVMFirewallRule(instanceID, rule)
	} else {
		return tc.createLighthouseFirewallRule(instanceID, rule)
	}
}

func (tc *TencentClient) DeleteFirewallRule(instanceID, ruleID string) error {
	if tc.isCVMInstance(instanceID) {
		return fmt.Errorf("CVM firewall rule management not implemented yet")
		// return tc.deleteCVMFirewallRule(instanceID, ruleID)
	} else {
		return tc.deleteLighthouseFirewallRule(instanceID, ruleID)
	}
}

func (tc *TencentClient) UpdateFirewallRule(instanceID, ruleID string, ruleSpec *FirewallRuleSpec, newIP string) (*FirewallRuleResult, error) {
	if tc.isCVMInstance(instanceID) {
		return nil, fmt.Errorf("CVM firewall rule management not implemented yet")
		// return tc.updateCVMFirewallRule(instanceID, ruleID, ruleSpec, newIP)
	} else {
		return tc.updateLighthouseFirewallRule(instanceID, ruleID, ruleSpec, newIP)
	}
}

func (tc *TencentClient) ListFirewallRules(instanceID string) ([]*FirewallRuleResult, error) {
	if tc.isCVMInstance(instanceID) {
		return nil, fmt.Errorf("CVM firewall rule management not implemented yet")
		// return tc.listCVMFirewallRules(instanceID)
	} else {
		return tc.listLighthouseFirewallRules(instanceID)
	}
}

// CVM 相关实现 - 暂未实现
func (tc *TencentClient) getCVMInstance(instanceID string) (*InstanceInfo, error) {
	request := cvm.NewDescribeInstancesRequest()
	request.InstanceIds = common.StringPtrs([]string{instanceID})

	response, err := tc.cvmClient.DescribeInstances(request)
	if err != nil {
		return nil, fmt.Errorf("failed to describe CVM instance: %v", err)
	}

	if len(response.Response.InstanceSet) == 0 {
		return nil, fmt.Errorf("cVM instance %s not found", instanceID)
	}

	instance := response.Response.InstanceSet[0]
	info := &InstanceInfo{
		InstanceID:   *instance.InstanceId,
		InstanceName: *instance.InstanceName,
		Status:       *instance.InstanceState,
		Provider:     "TencentCloud",
		Region:       tc.config.Region,
	}

	if len(instance.PublicIpAddresses) > 0 {
		info.PublicIP = *instance.PublicIpAddresses[0]
	}
	if len(instance.PrivateIpAddresses) > 0 {
		info.PrivateIP = *instance.PrivateIpAddresses[0]
	}

	return info, nil
}

// CVM防火墙规则相关方法暂未实现，使用安全组管理
/*
func (tc *TencentClient) createCVMFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
	// CVM 使用安全组来管理防火墙规则
	// 这里需要先获取实例的安全组，然后添加规则
	// 为了简化，这里返回一个模拟的结果
	log.Printf("Creating CVM firewall rule for instance %s: %+v", instanceID, rule)

	result := &FirewallRuleResult{
		RuleID:      fmt.Sprintf("cvm-rule-%s-%s", instanceID, rule.Port),
		Port:        rule.Port,
		Protocol:    rule.Protocol,
		CidrBlock:   rule.CidrBlock,
		Action:      rule.Action,
		Description: rule.Description,
		Provider:    "TencentCloud",
		InstanceID:  instanceID,
	}

	return result, nil
}

func (tc *TencentClient) deleteCVMFirewallRule(instanceID, ruleID string) error {
	log.Printf("Deleting CVM firewall rule %s for instance %s", ruleID, instanceID)
	return nil
}

func (tc *TencentClient) updateCVMFirewallRule(instanceID, ruleID string, newIP string) error {
	log.Printf("Updating CVM firewall rule %s for instance %s with new IP %s", ruleID, instanceID, newIP)
	return nil
}

func (tc *TencentClient) listCVMFirewallRules(instanceID string) ([]*FirewallRuleResult, error) {
	log.Printf("Listing CVM firewall rules for instance %s", instanceID)
	return []*FirewallRuleResult{}, nil
}
*/

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

func (tc *TencentClient) createLighthouseFirewallRule(instanceID string, rule *FirewallRuleSpec) (*FirewallRuleResult, error) {
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

	_, err := tc.lighthouseClient.CreateFirewallRules(request)
	if err != nil {
		if sdkError, ok := err.(*errors.TencentCloudSDKError); ok {
			return nil, fmt.Errorf("TencentCloud API Error: Code=%s, Message=%s",
				sdkError.Code, sdkError.Message)
		}
		return nil, fmt.Errorf("failed to create Lighthouse firewall rule: %v", err)
	}

	// 使用与列表规则相同的哈希算法生成规则ID
	ruleContent := fmt.Sprintf("%s-%s-%s-%s",
		strings.ToUpper(rule.Protocol),
		rule.Port,
		rule.CidrBlock,
		strings.ToUpper(rule.Action))
	ruleID := fmt.Sprintf("lh-%x", md5.Sum([]byte(ruleContent)))

	result := &FirewallRuleResult{
		RuleID:      ruleID,
		Port:        rule.Port,
		Protocol:    rule.Protocol,
		CidrBlock:   rule.CidrBlock,
		Action:      rule.Action,
		Description: rule.Description,
		Provider:    "TencentCloud",
		InstanceID:  instanceID,
	}

	log.Printf("Created Lighthouse firewall rule: %+v", result)
	return result, nil
}

func (tc *TencentClient) deleteLighthouseFirewallRule(instanceID, ruleID string) error {
	request := lighthouse.NewDeleteFirewallRulesRequest()
	request.InstanceId = common.StringPtr(instanceID)
	request.FirewallRules = []*lighthouse.FirewallRule{
		{
			// 这里需要根据实际的规则ID来删除，可能需要先查询规则详情
		},
	}

	_, err := tc.lighthouseClient.DeleteFirewallRules(request)
	if err != nil {
		if sdkError, ok := err.(*errors.TencentCloudSDKError); ok {
			return fmt.Errorf("TencentCloud API Error: Code=%s, Message=%s",
				sdkError.Code, sdkError.Message)
		}
		return fmt.Errorf("failed to delete Lighthouse firewall rule: %v", err)
	}

	log.Printf("Deleted Lighthouse firewall rule %s for instance %s", ruleID, instanceID)
	return nil
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

func (tc *TencentClient) updateLighthouseFirewallRule(instanceID, ruleID string, ruleSpec *FirewallRuleSpec, newIP string) (*FirewallRuleResult, error) {
	log.Printf("Updating Lighthouse firewall rule for instance %s with new IP %s", instanceID, newIP)
	log.Printf("Rule spec: Protocol=%s, Port=%s, Description=%s", ruleSpec.Protocol, ruleSpec.Port, ruleSpec.Description)

	// 获取所有现有规则
	rules, err := tc.listLighthouseFirewallRules(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing rules: %v", err)
	}

	// 通过备注、协议、端口匹配规则，而不是依赖RuleID
	var targetRule *FirewallRuleResult
	for _, rule := range rules {
		if rule.Protocol == strings.ToUpper(ruleSpec.Protocol) &&
			rule.Port == ruleSpec.Port &&
			rule.Description == ruleSpec.Description {
			targetRule = rule
			log.Printf("Found matching rule by spec: RuleID=%s, CidrBlock=%s", rule.RuleID, rule.CidrBlock)
			break
		}
	}

	if targetRule == nil {
		log.Printf("No matching rule found for Protocol=%s, Port=%s, Description=%s",
			ruleSpec.Protocol, ruleSpec.Port, ruleSpec.Description)
		return nil, fmt.Errorf("rule not found with protocol=%s, port=%s, description=%s",
			ruleSpec.Protocol, ruleSpec.Port, ruleSpec.Description)
	}

	// 构建新的CIDR块
	newCidrBlock := fmt.Sprintf("%s/32", newIP) // 如果IP已经是最新的，就不需要更新
	if targetRule.CidrBlock == newCidrBlock {
		log.Printf("Rule %s already has the correct IP %s", ruleID, newIP)
		return targetRule, nil
	}

	// 删除旧规则并创建新规则（Lighthouse不支持直接更新）
	// 首先创建新规则
	newRuleSpec := &FirewallRuleSpec{
		Protocol:    targetRule.Protocol,
		Port:        targetRule.Port,
		CidrBlock:   newCidrBlock,
		Action:      targetRule.Action,
		Description: targetRule.Description,
	}

	newRule, err := tc.createLighthouseFirewallRule(instanceID, newRuleSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to create new rule: %v", err)
	}

	// 删除旧规则
	err = tc.deleteLighthouseFirewallRuleBySpec(instanceID, targetRule)
	if err != nil {
		log.Printf("Warning: Created new rule but failed to delete old rule: %v", err)
		// 不返回错误，因为新规则已经创建成功
	}

	log.Printf("Successfully updated Lighthouse firewall rule for instance %s", instanceID)
	return newRule, nil
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
		// 使用规则内容生成稳定的ID
		ruleContent := fmt.Sprintf("%s-%s-%s-%s", *rule.Protocol, *rule.Port, *rule.CidrBlock, *rule.Action)
		ruleID := fmt.Sprintf("lh-%x", md5.Sum([]byte(ruleContent)))

		result := &FirewallRuleResult{
			RuleID:      ruleID,
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

// 工具函数
func (tc *TencentClient) isCVMInstance(instanceID string) bool {
	// 根据实例ID格式判断是否为CVM实例
	// CVM实例ID通常以 "ins-" 开头
	// Lighthouse实例ID通常以 "lhins-" 开头
	return strings.HasPrefix(instanceID, "ins-")
}

// 掩码SecretId用于日志输出
func maskSecretId(secretId string) string {
	if len(secretId) <= 8 {
		return "****"
	}
	return secretId[:4] + "****" + secretId[len(secretId)-4:]
}

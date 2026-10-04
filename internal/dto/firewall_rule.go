package dto

import (
	"FireFlow/internal/model"
	"time"
)

type FirewallRuleResponse struct {
	ID            uint      `json:"ID"`
	CreatedAt     time.Time `json:"CreatedAt"`
	UpdatedAt     time.Time `json:"UpdatedAt"`
	Provider      string    `json:"provider"`
	CloudConfigID uint      `json:"cloud_config_id"`
	InstanceID    string    `json:"instance_id"`
	Port          string    `json:"port"`
	Protocol      string    `json:"protocol"`
	LastIP        string    `json:"last_ip"`
	ProjectID     string    `json:"project_id"`
	Enabled       bool      `json:"enabled"`
	Remark        string    `json:"remark"`
}

func FirewallRule(rule model.FirewallRule) FirewallRuleResponse {
	return FirewallRuleResponse{
		ID: rule.ID, CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt,
		Provider: rule.CloudConfig.Provider, CloudConfigID: rule.CloudConfigID, InstanceID: rule.CloudConfig.InstanceId,
		Port: rule.Port, Protocol: rule.Protocol, LastIP: rule.LastIP, ProjectID: rule.CloudConfig.ProjectID,
		Enabled: rule.Enabled, Remark: rule.Remark,
	}
}

func FirewallRules(rules []model.FirewallRule) []FirewallRuleResponse {
	result := make([]FirewallRuleResponse, 0, len(rules))
	for _, rule := range rules {
		result = append(result, FirewallRule(rule))
	}
	return result
}

type FirewallRuleRequest struct {
	CloudConfigID uint   `json:"cloud_config_id"`
	Port          string `json:"port"`
	Protocol      string `json:"protocol"`
	Enabled       *bool  `json:"enabled"`
	Remark        string `json:"remark"`
}

func (request FirewallRuleRequest) Model() model.FirewallRule {
	enabled := true
	assign(&enabled, request.Enabled)
	return model.FirewallRule{CloudConfigID: request.CloudConfigID,
		Port: request.Port, Protocol: request.Protocol,
		Enabled: enabled, Remark: request.Remark}
}

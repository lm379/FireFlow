package dto

import (
	"FireFlow/internal/model"
	"time"
)

// CloudConfigResponse deliberately excludes credentials and arbitrary Extra data.
type CloudConfigResponse struct {
	ID             uint      `json:"ID"`
	CreatedAt      time.Time `json:"CreatedAt"`
	UpdatedAt      time.Time `json:"UpdatedAt"`
	Provider       string    `json:"provider"`
	Region         string    `json:"region"`
	Type           int       `json:"type"`
	InstanceID     string    `json:"instance_id"`
	ProjectID      string    `json:"project_id"`
	TenantID       string    `json:"tenant_id"`
	SubscriptionID string    `json:"subscription_id"`
	IsDefault      bool      `json:"is_default"`
	IsEnabled      bool      `json:"is_enabled"`
	Description    string    `json:"description"`
}

func CloudConfig(config model.CloudProviderConfig) CloudConfigResponse {
	return CloudConfigResponse{
		ID: config.ID, CreatedAt: config.CreatedAt, UpdatedAt: config.UpdatedAt,
		Provider: config.Provider, Region: config.Region, Type: config.Type,
		InstanceID: config.InstanceId, ProjectID: config.ProjectID,
		TenantID: config.TenantID, SubscriptionID: config.SubscriptionID,
		IsDefault: config.IsDefault, IsEnabled: config.IsEnabled, Description: config.Description,
	}
}

func CloudConfigs(configs []model.CloudProviderConfig) []CloudConfigResponse {
	result := make([]CloudConfigResponse, 0, len(configs))
	for _, config := range configs {
		result = append(result, CloudConfig(config))
	}
	return result
}

// Pointers distinguish omitted fields from explicit zero values on updates.
type CloudConfigRequest struct {
	Provider       *string `json:"provider"`
	SecretID       *string `json:"secret_id"`
	SecretKey      *string `json:"secret_key"`
	Region         *string `json:"region"`
	Type           *int    `json:"type"`
	InstanceID     *string `json:"instance_id"`
	ProjectID      *string `json:"project_id"`
	TenantID       *string `json:"tenant_id"`
	SubscriptionID *string `json:"subscription_id"`
	Extra          *string `json:"extra"`
	IsDefault      *bool   `json:"is_default"`
	IsEnabled      *bool   `json:"is_enabled"`
	Description    *string `json:"description"`
}

func (request CloudConfigRequest) ApplyTo(config *model.CloudProviderConfig) {
	assign(&config.Provider, request.Provider)
	// Blank credentials mean keep the stored value; they are never sent back.
	if request.SecretID != nil && *request.SecretID != "" {
		config.SecretId = *request.SecretID
	}
	if request.SecretKey != nil && *request.SecretKey != "" {
		config.SecretKey = *request.SecretKey
	}
	assign(&config.Region, request.Region)
	assign(&config.Type, request.Type)
	assign(&config.InstanceId, request.InstanceID)
	assign(&config.ProjectID, request.ProjectID)
	assign(&config.TenantID, request.TenantID)
	assign(&config.SubscriptionID, request.SubscriptionID)
	assign(&config.Extra, request.Extra)
	assign(&config.IsDefault, request.IsDefault)
	assign(&config.IsEnabled, request.IsEnabled)
	assign(&config.Description, request.Description)
}

func assign[T any](target *T, value *T) {
	if value != nil {
		*target = *value
	}
}

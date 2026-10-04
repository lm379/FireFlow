package dto

import (
	"FireFlow/internal/model"
	"time"
)

// PublicConfigKey is an allowlist: unknown settings may contain credentials.
func PublicConfigKey(key string) bool {
	switch key {
	case "ip_fetch_url", "ip_check_interval", "cron_enabled":
		return true
	default:
		return false
	}
}

func PublicConfigValue(key, value string) *string {
	if !PublicConfigKey(key) {
		return nil
	}
	return &value
}

type ConfigValueResponse struct {
	Key   string  `json:"key"`
	Value *string `json:"value,omitempty"`
}

type ConfigItemResponse struct {
	ID          uint      `json:"ID"`
	CreatedAt   time.Time `json:"CreatedAt"`
	UpdatedAt   time.Time `json:"UpdatedAt"`
	ConfigKey   string    `json:"config_key"`
	ConfigValue *string   `json:"config_value,omitempty"`
	ConfigType  string    `json:"config_type"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	IsEnabled   bool      `json:"is_enabled"`
}

func ConfigItems(configs []model.ConfigItem) []ConfigItemResponse {
	result := make([]ConfigItemResponse, 0, len(configs))
	for _, config := range configs {
		result = append(result, ConfigItemResponse{ID: config.ID, CreatedAt: config.CreatedAt,
			UpdatedAt: config.UpdatedAt, ConfigKey: config.ConfigKey,
			ConfigValue: PublicConfigValue(config.ConfigKey, config.ConfigValue), ConfigType: config.ConfigType,
			Category: config.Category, Description: config.Description, IsEnabled: config.IsEnabled})
	}
	return result
}

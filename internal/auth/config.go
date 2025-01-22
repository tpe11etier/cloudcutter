package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Environment struct {
	RoleID      string   `json:"roleId"`
	ProfileTags []string `json:"profileTags"`
}

type OpalConfig struct {
	Environments map[string]Environment `json:"environments"`
}

// DefaultOpalConfig returns the built-in configuration
func DefaultOpalConfig() OpalConfig {
	return OpalConfig{
		Environments: map[string]Environment{
			"dev": {
				RoleID: getEnvOrDefault("OPAL_DEV_ROLE_ID", "492ce125-9c7a-435e-b550-d4ccc259133e"),
				ProfileTags: []string{
					"dev",
					"development",
					"opal_dev",
					"opal-dev",
				},
			},
			"prod": {
				RoleID: getEnvOrDefault("OPAL_PROD_ROLE_ID", "fca565bc-1965-40b8-88fd-dd40b41e6770"),
				ProfileTags: []string{
					"prod",
					"production",
					"opal_prod",
					"opal-prod",
				},
			},
		},
	}
}

// LoadOpalConfig loads from file if exists, falls back to default
func LoadOpalConfig() OpalConfig {
	// Try to load from user config first
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(homeDir, ".cloudcutter", "opal.json")
		if data, err := os.ReadFile(configPath); err == nil {
			var config OpalConfig
			if err := json.Unmarshal(data, &config); err == nil {
				if isValidConfig(config) {
					return config
				}
			}
		}
	}

	return DefaultOpalConfig()
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func isValidConfig(config OpalConfig) bool {
	if config.Environments == nil {
		return false
	}

	for name, env := range config.Environments {
		if name == "" || env.RoleID == "" || len(env.ProfileTags) == 0 {
			return false
		}
	}

	return true
}

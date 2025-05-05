package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// OpalConfig represents the configuration for Opal
type OpalConfig struct {
	Environments map[string]OpalEnvironment `json:"environments"`
}

// OpalEnvironment represents an Opal environment configuration
type OpalEnvironment struct {
	RoleID      string   `json:"roleId"`
	ProfileTags []string `json:"profileTags"`
}

// DefaultOpalConfig returns the default Opal configuration
func DefaultOpalConfig() OpalConfig {
	// Default role IDs - obfuscated for security
	// These values will be overridden in production via environment variables
	devRoleID := "********-****-****-****-************"  // Obfuscated ID
	prodRoleID := "********-****-****-****-************" // Obfuscated ID

	// Override with environment variables if set
	if envDevRoleID := os.Getenv("OPAL_DEV_ROLE_ID"); envDevRoleID != "" {
		devRoleID = envDevRoleID
	}

	if envProdRoleID := os.Getenv("OPAL_PROD_ROLE_ID"); envProdRoleID != "" {
		prodRoleID = envProdRoleID
	}

	return OpalConfig{
		Environments: map[string]OpalEnvironment{
			"dev": {
				RoleID:      devRoleID,
				ProfileTags: []string{"dev", "development", "opal_dev", "opal-dev"},
			},
			"prod": {
				RoleID:      prodRoleID,
				ProfileTags: []string{"prod", "production", "opal_prod", "opal-prod"},
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

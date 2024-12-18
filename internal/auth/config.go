package auth

import (
	"os"
)

type OpalConfig struct {
	DevRoleID  string
	ProdRoleID string
}

func LoadOpalConfig() OpalConfig {
	return OpalConfig{
		DevRoleID:  getEnvOrDefault("OPAL_DEV_ROLE_ID", "492ce125-9c7a-435e-b550-d4ccc259133e"),
		ProdRoleID: getEnvOrDefault("OPAL_PROD_ROLE_ID", "fca565bc-1965-40b8-88fd-dd40b41e6770"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultOpalConfig(t *testing.T) {
	// Clear environment variables before test
	os.Unsetenv("OPAL_DEV_ROLE_ID")
	os.Unsetenv("OPAL_PROD_ROLE_ID")

	config := DefaultOpalConfig()

	// Test default values
	tests := []struct {
		name         string
		env          string
		roleID       string
		tagCount     int
		expectedTags []string
	}{
		{
			name:         "dev environment",
			env:          "dev",
			roleID:       "492ce125-9c7a-435e-b550-d4ccc259133e",
			tagCount:     4,
			expectedTags: []string{"dev", "development", "opal_dev", "opal-dev"},
		},
		{
			name:         "prod environment",
			env:          "prod",
			roleID:       "fca565bc-1965-40b8-88fd-dd40b41e6770",
			tagCount:     4,
			expectedTags: []string{"prod", "production", "opal_prod", "opal-prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, ok := config.Environments[tt.env]
			if !ok {
				t.Errorf("environment %s not found in config", tt.env)
				return
			}

			if env.RoleID != tt.roleID {
				t.Errorf("expected roleID %s, got %s", tt.roleID, env.RoleID)
			}

			if len(env.ProfileTags) != tt.tagCount {
				t.Errorf("expected %d profile tags, got %d", tt.tagCount, len(env.ProfileTags))
			}

			if !reflect.DeepEqual(env.ProfileTags, tt.expectedTags) {
				t.Errorf("expected profile tags %v, got %v", tt.expectedTags, env.ProfileTags)
			}
		})
	}
}

func TestDefaultOpalConfigWithEnvVars(t *testing.T) {
	// Set environment variables
	customDevRole := "custom-dev-role"
	customProdRole := "custom-prod-role"
	os.Setenv("OPAL_DEV_ROLE_ID", customDevRole)
	os.Setenv("OPAL_PROD_ROLE_ID", customProdRole)
	defer func() {
		os.Unsetenv("OPAL_DEV_ROLE_ID")
		os.Unsetenv("OPAL_PROD_ROLE_ID")
	}()

	config := DefaultOpalConfig()

	// Test environment variable override
	tests := []struct {
		name     string
		env      string
		expected string
	}{
		{
			name:     "dev role from env",
			env:      "dev",
			expected: customDevRole,
		},
		{
			name:     "prod role from env",
			env:      "prod",
			expected: customProdRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if config.Environments[tt.env].RoleID != tt.expected {
				t.Errorf("expected roleID %s, got %s", tt.expected, config.Environments[tt.env].RoleID)
			}
		})
	}
}

func TestLoadOpalConfig(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "cloudcutter-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create .cloudcutter directory
	ccDir := filepath.Join(tempDir, ".cloudcutter")
	if err := os.Mkdir(ccDir, 0755); err != nil {
		t.Fatalf("failed to create .cloudcutter dir: %v", err)
	}

	// Test cases
	tests := []struct {
		name          string
		configContent string
		expectDefault bool
	}{
		{
			name: "valid custom config",
			configContent: `{
                "environments": {
                    "custom": {
                        "roleId": "custom-role",
                        "profileTags": ["custom", "test"]
                    }
                }
            }`,
			expectDefault: false,
		},
		{
			name:          "invalid json",
			configContent: `{invalid json}`,
			expectDefault: true,
		},
		{
			name:          "empty file",
			configContent: "",
			expectDefault: true,
		},
		{
			name: "missing required fields",
			configContent: `{
                "environments": {
                    "test": {}
                }
            }`,
			expectDefault: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare test environment
			configPath := filepath.Join(ccDir, "opal.json")
			if tt.configContent != "" {
				if err := os.WriteFile(configPath, []byte(tt.configContent), 0644); err != nil {
					t.Fatalf("failed to write config file: %v", err)
				}
			}

			// Mock home directory for test
			originalHome := os.Getenv("HOME")
			os.Setenv("HOME", tempDir)
			defer os.Setenv("HOME", originalHome)

			// Clear any existing environment variables
			os.Unsetenv("OPAL_DEV_ROLE_ID")
			os.Unsetenv("OPAL_PROD_ROLE_ID")

			config := LoadOpalConfig()

			if tt.expectDefault {
				// Should match default config
				defaultConfig := DefaultOpalConfig()
				if !configsEqual(config, defaultConfig) {
					t.Errorf("configs don't match\ngot: %+v\nwant: %+v", config, defaultConfig)
				}
			} else {
				// Test that we got a valid custom config
				var expected OpalConfig
				if err := json.Unmarshal([]byte(tt.configContent), &expected); err != nil {
					t.Fatalf("failed to parse expected config: %v", err)
				}
				if !configsEqual(config, expected) {
					t.Errorf("configs don't match\ngot: %+v\nwant: %+v", config, expected)
				}
			}

			// Clean up
			os.Remove(configPath)
		})
	}
}

// Helper function to compare configs
func configsEqual(a, b OpalConfig) bool {
	if len(a.Environments) != len(b.Environments) {
		return false
	}

	for name, envA := range a.Environments {
		envB, exists := b.Environments[name]
		if !exists {
			return false
		}
		if envA.RoleID != envB.RoleID {
			return false
		}
		if len(envA.ProfileTags) != len(envB.ProfileTags) {
			return false
		}
		for i := range envA.ProfileTags {
			if envA.ProfileTags[i] != envB.ProfileTags[i] {
				return false
			}
		}
	}
	return true
}

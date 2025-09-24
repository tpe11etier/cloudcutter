//go:build integration
// +build integration

package vault

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests that require a running Vault instance
// Run with: go test -tags=integration ./internal/services/vault/

const (
	defaultVaultAddr  = "http://127.0.0.1:8200"
	defaultVaultToken = "myroot"
)

func getVaultConfig() (string, string) {
	addr := os.Getenv("VAULT_ADDR")
	if addr == "" {
		addr = defaultVaultAddr
	}

	token := os.Getenv("VAULT_TOKEN")
	if token == "" {
		token = defaultVaultToken
	}

	return addr, token
}

func TestIntegration_VaultService_FullWorkflow(t *testing.T) {
	addr, token := getVaultConfig()
	service := NewService()
	ctx := context.Background()

	// Test Health Check
	t.Run("health check", func(t *testing.T) {
		health, err := service.Health(ctx, addr)
		require.NoError(t, err)

		assert.True(t, health.Initialized, "Vault should be initialized")
		assert.False(t, health.Sealed, "Vault should be unsealed")
		assert.NotEmpty(t, health.Version, "Version should not be empty")
	})

	// Test List Mounts
	t.Run("list mounts", func(t *testing.T) {
		mounts, err := service.ListMounts(ctx, addr, token)
		require.NoError(t, err)

		assert.NotEmpty(t, mounts, "Should have at least some mounts")

		// Check for expected default mounts
		assert.Contains(t, mounts, "secret/", "Should have secret/ mount")
		assert.Contains(t, mounts, "sys/", "Should have sys/ mount")

		// Verify secret mount is KV v2
		secretMount := mounts["secret/"]
		assert.Equal(t, "kv", secretMount.Type)
		if secretMount.Options != nil {
			assert.Equal(t, "2", secretMount.Options["version"])
		}
	})

	// Test Empty Path Listing
	t.Run("list secrets from empty path", func(t *testing.T) {
		secrets, err := service.ListSecrets(ctx, addr, token, "secret/")
		require.NoError(t, err)
		// Should return empty slice, not error, for empty paths
		assert.NotNil(t, secrets)
	})
}

func TestIntegration_VaultService_SecretOperations(t *testing.T) {
	addr, token := getVaultConfig()
	service := NewService()
	ctx := context.Background()

	// Test secret operations with test data
	// Note: This test assumes the test secrets exist

	// Note: This test assumes the test secrets exist or creates them via HTTP
	// In a real integration test, you might want to create secrets first

	t.Run("get existing secret", func(t *testing.T) {
		// Try to get a test secret (assuming test data exists)
		secret, err := service.GetSecret(ctx, addr, token, "secret/myapp/config")

		if err != nil {
			t.Skipf("Skipping test - secret not found: %v", err)
			return
		}

		require.NoError(t, err)
		assert.NotNil(t, secret)
		assert.NotEmpty(t, secret.Data)
		assert.Equal(t, "secret/myapp/config", secret.Path)
		assert.Equal(t, "secret", secret.MountType)
	})

	t.Run("get nonexistent secret", func(t *testing.T) {
		_, err := service.GetSecret(ctx, addr, token, "secret/nonexistent-path-12345")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault API error")
	})

	t.Run("list secrets with existing data", func(t *testing.T) {
		secrets, err := service.ListSecrets(ctx, addr, token, "secret/")
		require.NoError(t, err)

		// Should not be nil
		assert.NotNil(t, secrets)

		// If we have test data, check for it
		if len(secrets) > 0 {
			t.Logf("Found secrets: %v", secrets)
		}
	})

	t.Run("list secrets from subdirectory", func(t *testing.T) {
		// This will either return secrets or an empty list
		secrets, err := service.ListSecrets(ctx, addr, token, "secret/myapp/")
		require.NoError(t, err)
		assert.NotNil(t, secrets)

		if len(secrets) > 0 {
			t.Logf("Found secrets in myapp/: %v", secrets)
		}
	})
}

func TestIntegration_VaultService_ErrorHandling(t *testing.T) {
	service := NewService()
	ctx := context.Background()

	t.Run("invalid vault address", func(t *testing.T) {
		_, err := service.Health(ctx, "http://invalid-vault-address:8200")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to make request")
	})

	t.Run("invalid token", func(t *testing.T) {
		addr, _ := getVaultConfig()

		_, err := service.ListMounts(ctx, addr, "invalid-token-12345")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vault API error")
	})

	t.Run("timeout handling", func(t *testing.T) {
		// Create a service with very short timeout
		shortTimeoutService := &Service{
			client: &http.Client{
				Timeout: 1 * time.Nanosecond, // Extremely short timeout
			},
		}

		addr, _ := getVaultConfig()
		_, err := shortTimeoutService.Health(ctx, addr)
		assert.Error(t, err)
	})
}

func TestIntegration_VaultService_ConcurrentAccess(t *testing.T) {
	addr, token := getVaultConfig()
	service := NewService()

	// Test concurrent access to ensure thread safety
	t.Run("concurrent health checks", func(t *testing.T) {
		const numGoroutines = 10

		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				ctx := context.Background()
				_, err := service.Health(ctx, addr)
				results <- err
			}()
		}

		// Collect results
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent health check should succeed")
		}
	})

	t.Run("concurrent mount listings", func(t *testing.T) {
		const numGoroutines = 5

		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				ctx := context.Background()
				_, err := service.ListMounts(ctx, addr, token)
				results <- err
			}()
		}

		// Collect results
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent mount listing should succeed")
		}
	})
}

// Helper function to check if Vault is available
func isVaultAvailable(t *testing.T) bool {
	addr, _ := getVaultConfig()
	service := NewService()
	ctx := context.Background()

	_, err := service.Health(ctx, addr)
	return err == nil
}

// TestMain can be used to setup/teardown for integration tests
func TestMain(m *testing.M) {
	// You could add setup/teardown logic here
	// For example, starting/stopping Vault, creating test data, etc.

	code := m.Run()

	// Cleanup logic would go here

	os.Exit(code)
}

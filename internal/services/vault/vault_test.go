package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewService(t *testing.T) {
	service := NewService()

	assert.NotNil(t, service)
	assert.NotNil(t, service.client)
	assert.Equal(t, 30*time.Second, service.client.Timeout)
}

func TestService_Health(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
		expectedHealth *Health
	}{
		{
			name: "successful health check",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/v1/sys/health", r.URL.Path)

				health := Health{
					Initialized: true,
					Sealed:      false,
					Standby:     false,
					Version:     "1.20.3",
					ClusterName: "vault-cluster-test",
					ClusterID:   "test-cluster-id",
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(health)
			},
			expectedHealth: &Health{
				Initialized: true,
				Sealed:      false,
				Standby:     false,
				Version:     "1.20.3",
				ClusterName: "vault-cluster-test",
				ClusterID:   "test-cluster-id",
			},
		},
		{
			name: "server error",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal server error"))
			},
			expectError: true, // Server error should return error
		},
		{
			name: "invalid json response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("invalid json"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			service := NewService()
			ctx := context.Background()

			health, err := service.Health(ctx, server.URL)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, health)
			} else {
				assert.NoError(t, err)
				if tt.expectedHealth != nil {
					assert.Equal(t, tt.expectedHealth, health)
				}
			}
		})
	}
}

func TestService_ListMounts(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		token          string
		expectError    bool
		expectedMounts map[string]*Mount
	}{
		{
			name:  "successful mounts listing",
			token: "test-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/v1/sys/mounts", r.URL.Path)
				assert.Equal(t, "test-token", r.Header.Get("X-Vault-Token"))

				response := struct {
					Data map[string]*Mount `json:"data"`
				}{
					Data: map[string]*Mount{
						"secret/": {
							Type:        "kv",
							Description: "key/value secret storage",
							Options:     map[string]string{"version": "2"},
							Local:       false,
							SealWrap:    false,
						},
						"sys/": {
							Type:        "system",
							Description: "system endpoints used for control, policy and debugging",
							Local:       false,
							SealWrap:    true,
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			},
			expectedMounts: map[string]*Mount{
				"secret/": {
					Type:        "kv",
					Description: "key/value secret storage",
					Options:     map[string]string{"version": "2"},
					Local:       false,
					SealWrap:    false,
				},
				"sys/": {
					Type:        "system",
					Description: "system endpoints used for control, policy and debugging",
					Local:       false,
					SealWrap:    true,
				},
			},
		},
		{
			name:  "unauthorized access",
			token: "invalid-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"errors":["permission denied"]}`))
			},
			expectError: true,
		},
		{
			name:  "invalid json response",
			token: "test-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("invalid json"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			service := NewService()
			ctx := context.Background()

			mounts, err := service.ListMounts(ctx, server.URL, tt.token)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, mounts)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMounts, mounts)
			}
		})
	}
}

func TestService_ListSecrets(t *testing.T) {
	tests := []struct {
		name            string
		serverResponse  func(w http.ResponseWriter, r *http.Request)
		path            string
		token           string
		expectError     bool
		expectedSecrets []string
	}{
		{
			name:  "successful secrets listing - root path",
			path:  "secret/",
			token: "test-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "LIST", r.Method)
				assert.Equal(t, "/v1/secret/metadata/", r.URL.Path)
				assert.Equal(t, "test-token", r.Header.Get("X-Vault-Token"))

				response := struct {
					Data struct {
						Keys []string `json:"keys"`
					} `json:"data"`
				}{
					Data: struct {
						Keys []string `json:"keys"`
					}{
						Keys: []string{"database", "myapp/"},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			},
			expectedSecrets: []string{"database", "myapp/"},
		},
		{
			name:  "successful secrets listing - subdirectory",
			path:  "secret/myapp/",
			token: "test-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "LIST", r.Method)
				assert.Equal(t, "/v1/secret/metadata/myapp/", r.URL.Path)

				response := struct {
					Data struct {
						Keys []string `json:"keys"`
					} `json:"data"`
				}{
					Data: struct {
						Keys []string `json:"keys"`
					}{
						Keys: []string{"config", "api"},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			},
			expectedSecrets: []string{"config", "api"},
		},
		{
			name:  "empty path - no secrets",
			path:  "secret/empty/",
			token: "test-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"errors":[]}`))
			},
			expectedSecrets: []string{},
		},
		{
			name:  "path not found with actual errors",
			path:  "secret/nonexistent/",
			token: "test-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"errors":["path does not exist"]}`))
			},
			expectError: true,
		},
		{
			name:  "unauthorized access",
			path:  "secret/",
			token: "invalid-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"errors":["permission denied"]}`))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			service := NewService()
			ctx := context.Background()

			secrets, err := service.ListSecrets(ctx, server.URL, tt.token, tt.path)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, secrets)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSecrets, secrets)
			}
		})
	}
}

func TestService_GetSecret(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		path           string
		token          string
		expectError    bool
		expectedSecret *Secret
	}{
		{
			name:  "successful secret retrieval",
			path:  "secret/myapp/config",
			token: "test-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/v1/secret/data/myapp/config", r.URL.Path)
				assert.Equal(t, "test-token", r.Header.Get("X-Vault-Token"))

				response := struct {
					Data struct {
						Data     map[string]interface{} `json:"data"`
						Metadata *SecretMetadata        `json:"metadata,omitempty"`
					} `json:"data"`
				}{
					Data: struct {
						Data     map[string]interface{} `json:"data"`
						Metadata *SecretMetadata        `json:"metadata,omitempty"`
					}{
						Data: map[string]interface{}{
							"username": "admin",
							"password": "secret123",
						},
						Metadata: &SecretMetadata{
							CreatedTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
							Version:     1,
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			},
			expectedSecret: &Secret{
				Data: map[string]interface{}{
					"username": "admin",
					"password": "secret123",
				},
				Metadata: &SecretMetadata{
					CreatedTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
					Version:     1,
				},
				Path:      "secret/myapp/config",
				MountType: "secret",
			},
		},
		{
			name:  "secret not found",
			path:  "secret/nonexistent",
			token: "test-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"errors":["secret not found"]}`))
			},
			expectError: true,
		},
		{
			name:        "invalid path format - no mount separator",
			path:        "invalidpath",
			token:       "test-token",
			expectError: true,
		},
		{
			name:  "unauthorized access",
			path:  "secret/restricted",
			token: "invalid-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"errors":["permission denied"]}`))
			},
			expectError: true,
		},
		{
			name:  "invalid json response",
			path:  "secret/test",
			token: "test-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("invalid json"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.path, "/") && tt.expectError {
				// Test invalid path format without server
				service := NewService()
				ctx := context.Background()

				secret, err := service.GetSecret(ctx, "http://example.com", tt.token, tt.path)

				assert.Error(t, err)
				assert.Nil(t, secret)
				assert.Contains(t, err.Error(), "invalid path format")
				return
			}

			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			service := NewService()
			ctx := context.Background()

			secret, err := service.GetSecret(ctx, server.URL, tt.token, tt.path)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, secret)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSecret, secret)
			}
		})
	}
}

// Test context cancellation
func TestService_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{}})
	}))
	defer server.Close()

	service := NewService()

	// Create a context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Test each method with cancelled context
	t.Run("Health with cancelled context", func(t *testing.T) {
		_, err := service.Health(ctx, server.URL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("ListMounts with cancelled context", func(t *testing.T) {
		_, err := service.ListMounts(ctx, server.URL, "token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("ListSecrets with cancelled context", func(t *testing.T) {
		_, err := service.ListSecrets(ctx, server.URL, "token", "secret/")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("GetSecret with cancelled context", func(t *testing.T) {
		_, err := service.GetSecret(ctx, server.URL, "token", "secret/test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

// Benchmark tests
func BenchmarkService_Health(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		health := Health{
			Initialized: true,
			Sealed:      false,
			Standby:     false,
			Version:     "1.20.3",
		}
		json.NewEncoder(w).Encode(health)
	}))
	defer server.Close()

	service := NewService()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.Health(ctx, server.URL)
	}
}

func BenchmarkService_ListMounts(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := struct {
			Data map[string]*Mount `json:"data"`
		}{
			Data: map[string]*Mount{
				"secret/": {Type: "kv", Description: "test"},
				"sys/":    {Type: "system", Description: "test"},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	service := NewService()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.ListMounts(ctx, server.URL, "token")
	}
}

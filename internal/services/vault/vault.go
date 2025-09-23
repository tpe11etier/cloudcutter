package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// Interface defines the contract for Vault operations
type Interface interface {
	// Health returns the health status of the Vault instance
	Health(ctx context.Context, addr string) (*Health, error)
	
	// ListSecrets lists all secrets in a given path
	ListSecrets(ctx context.Context, addr, token, path string) ([]string, error)
	
	// GetSecret retrieves a secret from Vault
	GetSecret(ctx context.Context, addr, token, path string) (*Secret, error)
	
	// ListMounts lists all mounted secret engines
	ListMounts(ctx context.Context, addr, token string) (map[string]*Mount, error)
}

// Health represents Vault health status
type Health struct {
	Initialized bool   `json:"initialized"`
	Sealed      bool   `json:"sealed"`
	Standby     bool   `json:"standby"`
	Version     string `json:"version"`
	ClusterName string `json:"cluster_name"`
	ClusterID   string `json:"cluster_id"`
}

// Secret represents a Vault secret
type Secret struct {
	Data map[string]interface{} `json:"data"`
	Metadata *SecretMetadata `json:"metadata,omitempty"`
	Path string `json:"path"`
	MountType string `json:"mount_type"`
}

// SecretMetadata contains metadata about a secret
type SecretMetadata struct {
	CreatedTime  time.Time `json:"created_time"`
	CustomMetadata map[string]string `json:"custom_metadata,omitempty"`
	DeleteTimeAfter string `json:"delete_time_after,omitempty"`
	Version      int      `json:"version"`
}

// Mount represents a mounted secret engine
type Mount struct {
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Config      map[string]interface{} `json:"config"`
	Options     map[string]string `json:"options"`
	Local       bool              `json:"local"`
	SealWrap    bool              `json:"seal_wrap"`
	ExternalEntropyAccess bool    `json:"external_entropy_access"`
	PluginVersion string          `json:"plugin_version,omitempty"`
	RunningVersion string         `json:"running_version,omitempty"`
	RunningSha256 string          `json:"running_sha256,omitempty"`
	DeprecationStatus string      `json:"deprecation_status,omitempty"`
}

// Service implements the Vault interface
type Service struct {
	client *http.Client
}

// NewService creates a new Vault service
func NewService() *Service {
	return &Service{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Health returns the health status of the Vault instance
func (s *Service) Health(ctx context.Context, addr string) (*Health, error) {
	url := fmt.Sprintf("%s/v1/sys/health", addr)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var health Health
	if err := json.Unmarshal(body, &health); err != nil {
		return nil, fmt.Errorf("failed to unmarshal health response: %w", err)
	}
	
	return &health, nil
}

// ListSecrets lists all secrets in a given path
func (s *Service) ListSecrets(ctx context.Context, addr, token, path string) ([]string, error) {
	// For KV v2 engines, we need to use the metadata endpoint
	// Path format: "secret/myapp/" -> mount="secret", subPath="myapp/"
	cleanPath := strings.TrimPrefix(path, "/")
	cleanPath = strings.TrimSuffix(cleanPath, "/") // Remove trailing slash
	
	// Split into mount and subpath
	pathParts := strings.SplitN(cleanPath, "/", 2)
	mount := pathParts[0]
	subPath := ""
	if len(pathParts) > 1 {
		subPath = pathParts[1] + "/"
	}
	
	url := fmt.Sprintf("%s/v1/%s/metadata/%s", addr, mount, subPath)
	
	req, err := http.NewRequestWithContext(ctx, "LIST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("X-Vault-Token", token)
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("vault API error (HTTP %d): %s", resp.StatusCode, string(body))
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var response struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets response: %w", err)
	}
	
	return response.Data.Keys, nil
}

// GetSecret retrieves a secret from Vault
func (s *Service) GetSecret(ctx context.Context, addr, token, path string) (*Secret, error) {
	// For KV v2 engines, we need to use the data endpoint
	// Path format: "secret/myapp/config" -> mount="secret", secretPath="myapp/config"
	cleanPath := strings.TrimPrefix(path, "/")
	pathParts := strings.SplitN(cleanPath, "/", 2)
	if len(pathParts) != 2 {
		return nil, fmt.Errorf("invalid path format: %s", path)
	}
	
	mount := pathParts[0]
	secretPath := pathParts[1]
	url := fmt.Sprintf("%s/v1/%s/data/%s", addr, mount, secretPath)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("X-Vault-Token", token)
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("vault API error: %s", string(body))
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var response struct {
		Data struct {
			Data     map[string]interface{} `json:"data"`
			Metadata *SecretMetadata `json:"metadata,omitempty"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret response: %w", err)
	}
	
	return &Secret{
		Data:     response.Data.Data,
		Metadata: response.Data.Metadata,
		Path:     path,
		MountType: mount,
	}, nil
}

// ListMounts lists all mounted secret engines
func (s *Service) ListMounts(ctx context.Context, addr, token string) (map[string]*Mount, error) {
	url := fmt.Sprintf("%s/v1/sys/mounts", addr)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("X-Vault-Token", token)
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("vault API error: %s", string(body))
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var response struct {
		Data map[string]*Mount `json:"data"`
	}
	
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal mounts response: %w", err)
	}
	
	return response.Data, nil
}
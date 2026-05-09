package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all"
)

const (
	DefaultDragosBaseURL      = "https://tpelletier-sitestore.dev.platform.dragos.cloud"
	DefaultDragosIndexPattern = "events*"
	DefaultDragosKbnVersion   = "8.19.2"
	// DefaultDragosProviderID is the UUID Dragos uses for the platform's own
	// password-based identity provider. Other providers (SAML/OIDC) would have
	// different UUIDs — exposed in dragos.json so this can be overridden.
	DefaultDragosProviderID = "00000000-0000-0000-0000-000000000002"
)

// DragosConfig holds connection settings for the Dragos platform.
//
// Token resolution order: env var → ~/.cloudcutter/dragos.json → browser
// cookie store. The first source that yields a non-empty token wins. Browser
// is the preferred source in practice — the user is already SSO'd in their
// browser and refreshing Kibana there yields a fresh JWT that cloudcutter
// picks up automatically on the next 401 retry.
//
// Other settings (BaseURL, IndexPattern, KbnVersion) follow defaults → file
// → env override semantics.
type DragosConfig struct {
	BaseURL      string `json:"baseUrl,omitempty"`
	IndexPattern string `json:"indexPattern,omitempty"`
	AuthToken    string `json:"authToken,omitempty"`
	KbnVersion   string `json:"kbnVersion,omitempty"`
	ProviderID   string `json:"providerId,omitempty"`
	// AuthSource records where AuthToken was obtained ("env", "file",
	// "browser", "login"). Diagnostic only.
	AuthSource string `json:"-"`
}

func DefaultDragosConfig() DragosConfig {
	return DragosConfig{
		BaseURL:      DefaultDragosBaseURL,
		IndexPattern: DefaultDragosIndexPattern,
		KbnVersion:   DefaultDragosKbnVersion,
		ProviderID:   DefaultDragosProviderID,
	}
}

// DragosConfigPath returns the on-disk location used for dragos.json.
// Returns the empty string if the home directory can't be resolved.
func DragosConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".cloudcutter", "dragos.json")
}

// SaveDragosToken persists the JWT and the connection settings it was issued
// against to ~/.cloudcutter/dragos.json. baseURL/providerID/indexPattern/
// kbnVersion are stored alongside the token so a token issued against an
// env-overridden baseURL doesn't get orphaned the next time the user starts
// without that env var set. The file is created with 0600 permissions because
// the JWT is a credential.
func SaveDragosToken(token string, cfg DragosConfig) error {
	path := DragosConfigPath()
	if path == "" {
		return fmt.Errorf("could not resolve home directory")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	existing := DragosConfig{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing) // tolerate corruption — we're about to overwrite
	}

	existing.AuthToken = strings.TrimSpace(token)
	if cfg.BaseURL != "" {
		existing.BaseURL = cfg.BaseURL
	}
	if cfg.IndexPattern != "" {
		existing.IndexPattern = cfg.IndexPattern
	}
	if cfg.KbnVersion != "" {
		existing.KbnVersion = cfg.KbnVersion
	}
	if cfg.ProviderID != "" {
		existing.ProviderID = cfg.ProviderID
	}

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func LoadDragosConfig() (DragosConfig, error) {
	cfg := DefaultDragosConfig()

	configPath := ""
	fileToken := ""
	if homeDir, err := os.UserHomeDir(); err == nil {
		configPath = filepath.Join(homeDir, ".cloudcutter", "dragos.json")
		if data, readErr := os.ReadFile(configPath); readErr == nil {
			var fileCfg DragosConfig
			if err := json.Unmarshal(data, &fileCfg); err != nil {
				return cfg, fmt.Errorf("invalid %s: %w", configPath, err)
			}
			if fileCfg.BaseURL != "" {
				cfg.BaseURL = fileCfg.BaseURL
			}
			if fileCfg.IndexPattern != "" {
				cfg.IndexPattern = fileCfg.IndexPattern
			}
			if fileCfg.AuthToken != "" {
				fileToken = fileCfg.AuthToken
			}
			if fileCfg.KbnVersion != "" {
				cfg.KbnVersion = fileCfg.KbnVersion
			}
			if fileCfg.ProviderID != "" {
				cfg.ProviderID = fileCfg.ProviderID
			}
		}
	}

	if v := os.Getenv("DRAGOS_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("DRAGOS_INDEX_PATTERN"); v != "" {
		cfg.IndexPattern = v
	}
	if v := os.Getenv("DRAGOS_KBN_VERSION"); v != "" {
		cfg.KbnVersion = v
	}

	// Token resolution: env > file. The browser-cookie path (kooky) is
	// disabled — we want the in-app login modal to be the only auth surface,
	// and silently picking up whatever stale cookie is in Chrome short-
	// circuited that. Re-enable by uncommenting the `default:` branch below.
	switch {
	case os.Getenv("DRAGOS_AUTH_TOKEN") != "":
		cfg.AuthToken = os.Getenv("DRAGOS_AUTH_TOKEN")
		cfg.AuthSource = "env"
	case fileToken != "":
		cfg.AuthToken = fileToken
		cfg.AuthSource = "file"
		// default:
		// 	token, err := loadDragosCookieFromBrowser(cfg.BaseURL)
		// 	if err == nil && token != "" {
		// 		cfg.AuthToken = token
		// 		cfg.AuthSource = "browser"
		// 	}
	}

	if cfg.AuthToken == "" {
		return cfg, fmt.Errorf("dragos auth token unavailable — log in via the modal, or set DRAGOS_AUTH_TOKEN")
	}

	return cfg, nil
}

// loadDragosCookieFromBrowser reads dragos-auth-token from any locally
// installed browser's cookie store (Chrome, Firefox, Safari, Edge, Brave,
// etc.). Returns the value of the freshest cookie that's applicable to
// baseURL's host. macOS may prompt for Keychain access the first time.
//
// We filter only by cookie name, not by domain — browsers store cookies under
// whatever Domain attribute the server set (often a parent like
// `.platform.dragos.cloud`), and kooky's domain filters don't model the
// "host-matches-cookie-domain" relationship. Filtering on the unique cookie
// name is reliable enough.
func loadDragosCookieFromBrowser(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// kooky returns both an error and partial results when *some* discovered
	// stores fail (e.g. Safari binary cookies file is sandboxed, missing
	// browsers in default paths). Only treat err as fatal if no cookies came
	// back at all.
	cookies, err := kooky.ReadCookies(ctx, kooky.Name("dragos-auth-token"))
	if len(cookies) == 0 {
		if err != nil {
			return "", fmt.Errorf("kooky.ReadCookies: %w", err)
		}
		return "", fmt.Errorf("no dragos-auth-token cookie found in any browser store (is the browser running and Keychain access granted?)")
	}

	host := strings.ToLower(u.Hostname())
	var best *kooky.Cookie
	for _, c := range cookies {
		if c == nil || c.Value == "" {
			continue
		}
		// Skip cookies whose Domain doesn't apply to this host. Cookie domains
		// may have a leading dot for non-host-only cookies.
		cd := strings.ToLower(strings.TrimPrefix(c.Domain, "."))
		if cd != "" && cd != host && !strings.HasSuffix(host, "."+cd) {
			continue
		}
		if best == nil || c.Expires.After(best.Expires) {
			best = c
		}
	}
	if best == nil {
		return "", fmt.Errorf("found %d dragos-auth-token cookie(s) but none apply to host %s", len(cookies), host)
	}
	return best.Value, nil
}

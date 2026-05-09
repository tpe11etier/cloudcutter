package auth

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tpe11etier/cloudcutter/internal/config"
	"github.com/tpe11etier/cloudcutter/internal/environments"
)

// legacyToEnvironment maps a legacy profile name to an Environment that
// describes the same auth+transport+index+timefields the existing
// per-profile code path uses today. It exists so SwitchProfile can pivot
// to SwitchEnvironment without changing user-visible behavior; phase 4
// deletes this function when the manager consumes the Resolver directly.
func (a *Authenticator) legacyToEnvironment(profile, region string) (environments.Environment, error) {
	switch {
	case profile == DragosProfile:
		return dragosEnvironment(profile, region)
	case a.opalProfiles[profile] != "":
		return opalEnvironment(profile, region, a.opalProfiles[profile]), nil
	case profile == "local":
		return localEnvironment(profile, region), nil
	default:
		return standardAWSEnvironment(profile, region), nil
	}
}

func dragosEnvironment(profile, region string) (environments.Environment, error) {
	cfg, err := LoadDragosConfig()
	if err != nil {
		return environments.Environment{}, err
	}

	// Migrate the JWT from the structured dragos.json into a plain-text
	// dragos.token sibling file. loadJWT in SwitchEnvironment expects a
	// flat token file; if we pointed Path at dragos.json directly it would
	// read the whole JSON as the token and produce an invalid Cookie
	// header. Always-overwrite keeps dragos.token in sync with the legacy
	// flow's source of truth (dragos.json, written by SaveDragosToken).
	tokenPath, err := writeDragosTokenFile(cfg.AuthToken)
	if err != nil {
		return environments.Environment{}, err
	}

	return environments.Environment{
		Name:   profile,
		Region: region,
		Auth: config.AuthSpec{
			Type: "jwt",
			Path: tokenPath,
			Env:  "DRAGOS_AUTH_TOKEN",
			Login: &config.LoginSpec{
				URL:        cfg.BaseURL + "/auth/api/v1/login/password",
				BodyFormat: "json",
				BodyFields: []config.FormField{
					{Name: "username", Kind: "text"},
					{Name: "password", Kind: "password"},
				},
				Query: map[string]string{"providerId": cfg.ProviderID},
				TokenExtract: config.TokenExtractSpec{
					From: "cookie",
					Name: "dragos-auth-token",
				},
			},
		},
		Transport: config.TransportSpec{
			Type:      "kibana_proxy",
			BaseURL:   cfg.BaseURL,
			ProxyPath: "/kibana/api/console/proxy",
			TokenHeader: &config.TokenHeaderSpec{
				Name:   "Cookie",
				Format: "dragos-auth-token={token}",
			},
			Headers: map[string]string{
				"kbn-xsrf":    "cloudcutter",
				"kbn-version": cfg.KbnVersion,
			},
			Probe: &config.ProbeSpec{
				Path:       "_cluster/health",
				RejectHTML: true,
			},
		},
		IndexPattern: cfg.IndexPattern,
		TimeFields: []config.TimeField{
			{Name: "createdAt", Format: "date"},
		},
	}, nil
}

func opalEnvironment(profile, region, roleID string) environments.Environment {
	prefix := "dev"
	if profile == "opal_prod" || profile == "prod" || profile == "production" {
		prefix = "prod"
	}
	return environments.Environment{
		Name:   profile,
		Region: region,
		Auth: config.AuthSpec{
			Type: "aws_sdk",
			PreAuth: &config.PreAuthSpec{
				Command: []string{
					"opal", "iam-roles:start",
					"--id", roleID,
					"--profileName", profile,
				},
				DetectSessionExpired: []string{
					"Enter your email",
					"session is invalid or expired",
				},
			},
		},
		Transport: config.TransportSpec{
			Type:        "sigv4",
			Service:     "es",
			URLTemplate: fmt.Sprintf("https://%s-%s-primary-es.darkbytes.io", prefix, region),
		},
		IndexPattern: "main-summary-*",
		TimeFields: []config.TimeField{
			{Name: "unixTime", Format: "unix"},
			{Name: "detectionGeneratedTime", Format: "unix_ms"},
		},
	}
}

func localEnvironment(profile, region string) environments.Environment {
	return environments.Environment{
		Name:   profile,
		Region: region,
		Auth:   config.AuthSpec{Type: "none"},
		Transport: config.TransportSpec{
			Type:    "plain",
			BaseURL: "http://localhost:9200",
		},
	}
}

func standardAWSEnvironment(profile, region string) environments.Environment {
	return environments.Environment{
		Name:   profile,
		Region: region,
		Auth:   config.AuthSpec{Type: "aws_sdk"},
		Transport: config.TransportSpec{
			Type:        "sigv4",
			Service:     "es",
			URLTemplate: fmt.Sprintf("https://dev-%s-primary-es.darkbytes.io", region),
		},
		IndexPattern: "main-summary-*",
		TimeFields: []config.TimeField{
			{Name: "unixTime", Format: "unix"},
			{Name: "detectionGeneratedTime", Format: "unix_ms"},
		},
	}
}

// writeDragosTokenFile writes the given JWT (verbatim, trimmed of leading
// and trailing whitespace) to ~/.cloudcutter/dragos.token at mode 0600 and
// returns the path. Used by the legacy translator to bridge the structured
// dragos.json (which loadJWT can't parse) to a flat token file (which it
// can). Phase 4 deletes this when the manager consumes the Resolver
// directly and the YAML-defined dragos environment points at its own
// token path.
func writeDragosTokenFile(token string) (string, error) {
	jsonPath := DragosConfigPath()
	if jsonPath == "" {
		return "", fmt.Errorf("could not resolve home directory for dragos token cache")
	}
	tokenPath := filepath.Join(filepath.Dir(jsonPath), "dragos.token")
	if err := os.WriteFile(tokenPath, []byte(token), 0o600); err != nil {
		return "", fmt.Errorf("write dragos token cache: %w", err)
	}
	return tokenPath, nil
}

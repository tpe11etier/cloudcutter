package auth

import (
	"fmt"

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
	return environments.Environment{
		Name:   profile,
		Region: region,
		Auth: config.AuthSpec{
			Type: "jwt",
			Path: DragosConfigPath(),
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

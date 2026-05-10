package elastic

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpe11etier/cloudcutter/internal/auth"
	"github.com/tpe11etier/cloudcutter/internal/config"
	"github.com/tpe11etier/cloudcutter/internal/environments"
)

// NewDragosService is the legacy entry point used by services.go's
// InitializeElasticDragos. Phase 3 keeps it working by translating the
// DragosSession into an Environment and delegating to NewServiceFromEnv.
// Phase 4 deletes this once the manager constructs Environment values
// directly.
func NewDragosService(d *auth.DragosSession) (*Service, error) {
	if d == nil {
		return nil, fmt.Errorf("nil DragosSession")
	}
	if d.AuthToken == "" {
		return nil, fmt.Errorf("dragos auth token is empty")
	}
	if d.BaseURL == "" {
		return nil, fmt.Errorf("dragos base URL is empty")
	}

	env := legacyDragosEnvFromSession(d)
	return NewServiceFromEnv(env, aws.Config{}, d.AuthToken)
}

// ReinitializeDragos is the legacy reinit entry point.
func (s *Service) ReinitializeDragos(d *auth.DragosSession) error {
	if d == nil {
		return fmt.Errorf("nil DragosSession")
	}
	env := legacyDragosEnvFromSession(d)
	return s.ReinitializeFromEnv(env, aws.Config{}, d.AuthToken)
}

// legacyDragosEnvFromSession reproduces the phase-2 translator's
// dragosEnvironment shape from a DragosSession (which is what the
// services.InitializeElasticDragos caller has). Kept inline rather than
// imported from internal/auth to avoid the dependency cycle services →
// auth → environments.
func legacyDragosEnvFromSession(d *auth.DragosSession) environments.Environment {
	return environments.Environment{
		Name: auth.DragosProfile,
		Auth: config.AuthSpec{Type: "jwt"},
		Transport: config.TransportSpec{
			Type:      "kibana_proxy",
			BaseURL:   d.BaseURL,
			ProxyPath: "/kibana/api/console/proxy",
			TokenHeader: &config.TokenHeaderSpec{
				Name:   "Cookie",
				Format: "dragos-auth-token={token}",
			},
			Headers: map[string]string{
				"kbn-xsrf":    "cloudcutter",
				"kbn-version": d.KbnVersion,
			},
			Probe: &config.ProbeSpec{
				Path:       "_cluster/health",
				RejectHTML: true,
			},
		},
		IndexPattern: d.IndexPattern,
	}
}

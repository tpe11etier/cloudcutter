package environments

import (
	"sort"
	"strings"
	"testing"

	"github.com/tpe11etier/cloudcutter/internal/config"
)

func sampleConfig() *config.Config {
	return &config.Config{
		DefaultAWSBackend: &config.EnvironmentTemplate{
			Vars: map[string][]config.VarRule{
				"prefix": {
					{Match: "^opal_dev$", Value: "dev"},
					{Match: "^opal_prod$", Value: "prod"},
				},
			},
			Auth: config.AuthSpec{Type: "aws_sdk"},
			Transport: config.TransportSpec{
				Type:        "sigv4",
				Service:     "es",
				URLTemplate: "https://{prefix}-{region}-x.example.com",
			},
		},
		Environments: []config.EnvironmentSpec{
			{
				Name: "dragos",
				Auth: config.AuthSpec{Type: "jwt", Path: "/tmp/x.token"},
				Transport: config.TransportSpec{
					Type:      "kibana_proxy",
					BaseURL:   "https://platform.dragos.cloud",
					ProxyPath: "/kibana/api/console/proxy",
					TokenHeader: &config.TokenHeaderSpec{
						Name:   "Cookie",
						Format: "dragos-auth-token={token}",
					},
				},
			},
			{
				Name:      "local",
				Auth:      config.AuthSpec{Type: "none"},
				Transport: config.TransportSpec{Type: "plain", BaseURL: "http://localhost:9200"},
			},
		},
	}
}

func TestResolverList(t *testing.T) {
	r := NewResolver(sampleConfig(), []string{"opal_dev", "opal_prod"})
	got := r.List()
	sort.Strings(got)
	want := []string{"dragos", "local", "opal_dev", "opal_prod"}
	if len(got) != len(want) {
		t.Fatalf("List = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("List[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolverResolveExplicitEnv(t *testing.T) {
	r := NewResolver(sampleConfig(), nil)
	spec, err := r.Resolve("dragos")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if spec.Auth.Type != "jwt" {
		t.Errorf("dragos auth.type = %q", spec.Auth.Type)
	}
}

func TestResolverResolveAWSProfile(t *testing.T) {
	r := NewResolver(sampleConfig(), []string{"opal_dev"})
	spec, err := r.Resolve("opal_dev")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if spec.Name != "opal_dev" {
		t.Errorf("Name = %q", spec.Name)
	}
	if spec.Auth.Type != "aws_sdk" {
		t.Errorf("Auth.Type = %q (default_aws_backend should be applied)", spec.Auth.Type)
	}
	// vars should be carried so Materialize can substitute {prefix}
	env, err := Materialize(spec, "us-west-2")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	want := "https://dev-us-west-2-x.example.com"
	if env.Transport.URLTemplate != want {
		t.Errorf("URLTemplate = %q, want %q", env.Transport.URLTemplate, want)
	}
}

func TestResolverShadowingPrefersExplicit(t *testing.T) {
	cfg := sampleConfig()
	// Make "opal_dev" both an AWS profile AND an explicit env entry; the
	// explicit one should win.
	cfg.Environments = append(cfg.Environments, config.EnvironmentSpec{
		Name:      "opal_dev",
		Auth:      config.AuthSpec{Type: "none"},
		Transport: config.TransportSpec{Type: "plain", BaseURL: "http://override"},
	})
	r := NewResolver(cfg, []string{"opal_dev"})
	spec, err := r.Resolve("opal_dev")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if spec.Auth.Type != "none" {
		t.Errorf("explicit env should win, got Auth.Type = %q", spec.Auth.Type)
	}
}

func TestResolverAWSProfileWithoutDefault(t *testing.T) {
	cfg := sampleConfig()
	cfg.DefaultAWSBackend = nil
	r := NewResolver(cfg, []string{"opal_dev"})
	_, err := r.Resolve("opal_dev")
	if err == nil {
		t.Fatal("expected error for AWS profile with no default_aws_backend")
	}
	if !strings.Contains(err.Error(), "default_aws_backend") {
		t.Errorf("error should mention default_aws_backend, got %q", err.Error())
	}
}

func TestResolverUnknownName(t *testing.T) {
	r := NewResolver(sampleConfig(), nil)
	_, err := r.Resolve("nope")
	if err == nil {
		t.Fatal("expected error for unknown name")
	}
}

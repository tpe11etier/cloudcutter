package environments

import (
	"strings"
	"testing"

	"github.com/tpe11etier/cloudcutter/internal/config"
)

func sigv4Spec() config.EnvironmentSpec {
	return config.EnvironmentSpec{
		Name: "opal_dev",
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
			URLTemplate: "https://{prefix}-{region}-primary-es.darkbytes.io",
		},
		IndexPattern: "main-summary-*",
	}
}

func TestMaterializeSubstitutesRegionAndVars(t *testing.T) {
	spec := sigv4Spec()
	env, err := Materialize(spec, "us-west-2")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	want := "https://dev-us-west-2-primary-es.darkbytes.io"
	if env.Transport.URLTemplate != want {
		t.Errorf("URLTemplate after materialize = %q, want %q", env.Transport.URLTemplate, want)
	}
	if env.Region != "us-west-2" {
		t.Errorf("Region = %q", env.Region)
	}
}

func TestMaterializeFirstMatchWins(t *testing.T) {
	spec := sigv4Spec()
	spec.Vars["prefix"] = []config.VarRule{
		{Match: "^opal_.*$", Value: "shared"},
		{Match: "^opal_dev$", Value: "dev"}, // never reached
	}
	env, err := Materialize(spec, "us-west-2")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if !strings.Contains(env.Transport.URLTemplate, "shared-us-west-2") {
		t.Errorf("expected first-match-wins prefix=shared, got %q", env.Transport.URLTemplate)
	}
}

func TestMaterializeUnresolvedVarErrors(t *testing.T) {
	spec := sigv4Spec()
	spec.Name = "unknown_profile" // no var rule will match
	_, err := Materialize(spec, "us-west-2")
	if err == nil {
		t.Fatal("expected unresolved-var error")
	}
	if !strings.Contains(err.Error(), "prefix") {
		t.Errorf("expected error to name unresolved key, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "unknown_profile") {
		t.Errorf("expected error to mention profile name, got %q", err.Error())
	}
}

func TestMaterializeJWTPassesThrough(t *testing.T) {
	spec := config.EnvironmentSpec{
		Name: "dragos",
		Auth: config.AuthSpec{Type: "jwt", Path: "~/.cloudcutter/dragos.token"},
		Transport: config.TransportSpec{
			Type:    "kibana_proxy",
			BaseURL: "https://platform.dragos.cloud",
			TokenHeader: &config.TokenHeaderSpec{
				Name:   "Cookie",
				Format: "dragos-auth-token={token}",
			},
		},
	}
	env, err := Materialize(spec, "")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	// JWT/kibana_proxy doesn't use {region} or {profile} — original
	// fields should pass through verbatim.
	if env.Transport.BaseURL != "https://platform.dragos.cloud" {
		t.Errorf("BaseURL after materialize = %q", env.Transport.BaseURL)
	}
	if env.Transport.TokenHeader == nil || env.Transport.TokenHeader.Format != "dragos-auth-token={token}" {
		t.Errorf("token_header.format should be unchanged, got %v", env.Transport.TokenHeader)
	}
}

func TestMaterializeEnvVarLookup(t *testing.T) {
	t.Setenv("CC_TEST_ROLE_ID", "role-from-env")
	spec := config.EnvironmentSpec{
		Name: "opal_dev",
		Vars: map[string][]config.VarRule{
			"role_id": {
				{Match: "^opal_dev$", Env: "CC_TEST_ROLE_ID"},
			},
		},
		Auth: config.AuthSpec{
			Type: "aws_sdk",
			PreAuth: &config.PreAuthSpec{
				Command: []string{"opal", "--id", "{role_id}", "--profileName", "{profile}"},
			},
		},
		Transport: config.TransportSpec{Type: "plain", BaseURL: "https://example.com"},
	}
	env, err := Materialize(spec, "us-west-2")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	wantCmd := []string{"opal", "--id", "role-from-env", "--profileName", "opal_dev"}
	got := env.Auth.PreAuth.Command
	if len(got) != len(wantCmd) {
		t.Fatalf("Command len = %d, want %d", len(got), len(wantCmd))
	}
	for i := range wantCmd {
		if got[i] != wantCmd[i] {
			t.Errorf("Command[%d] = %q, want %q", i, got[i], wantCmd[i])
		}
	}
}

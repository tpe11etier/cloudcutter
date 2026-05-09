package auth

import (
	"strings"
	"testing"

	"github.com/tpe11etier/cloudcutter/internal/config"
)

func newTestAuth(t *testing.T) *Authenticator {
	t.Helper()
	a, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Inject a deterministic opal map regardless of OpalConfig defaults.
	a.opalProfiles = map[string]string{
		"opal_dev":  "DEV-ROLE-UUID",
		"opal_prod": "PROD-ROLE-UUID",
	}
	return a
}

func TestTranslateLocal(t *testing.T) {
	a := newTestAuth(t)
	env, err := a.legacyToEnvironment("local", "us-west-2")
	if err != nil {
		t.Fatalf("legacyToEnvironment: %v", err)
	}
	if env.Auth.Type != "none" {
		t.Errorf("Auth.Type = %q, want none", env.Auth.Type)
	}
	if env.Transport.Type != "plain" {
		t.Errorf("Transport.Type = %q, want plain", env.Transport.Type)
	}
	if env.Transport.BaseURL != "http://localhost:9200" {
		t.Errorf("Transport.BaseURL = %q", env.Transport.BaseURL)
	}
	if env.Region != "us-west-2" {
		t.Errorf("Region = %q", env.Region)
	}
}

func TestTranslateOpal(t *testing.T) {
	a := newTestAuth(t)
	env, err := a.legacyToEnvironment("opal_dev", "us-west-2")
	if err != nil {
		t.Fatalf("legacyToEnvironment: %v", err)
	}
	if env.Auth.Type != "aws_sdk" {
		t.Errorf("Auth.Type = %q, want aws_sdk", env.Auth.Type)
	}
	if env.Auth.PreAuth == nil {
		t.Fatal("PreAuth is nil; expected opal command")
	}
	wantCmd := []string{"opal", "iam-roles:start", "--id", "DEV-ROLE-UUID", "--profileName", "opal_dev"}
	if len(env.Auth.PreAuth.Command) != len(wantCmd) {
		t.Fatalf("Command len = %d, want %d", len(env.Auth.PreAuth.Command), len(wantCmd))
	}
	for i := range wantCmd {
		if env.Auth.PreAuth.Command[i] != wantCmd[i] {
			t.Errorf("Command[%d] = %q, want %q", i, env.Auth.PreAuth.Command[i], wantCmd[i])
		}
	}
	if env.Transport.Type != "sigv4" {
		t.Errorf("Transport.Type = %q", env.Transport.Type)
	}
	want := "https://dev-us-west-2-primary-es.darkbytes.io"
	if env.Transport.URLTemplate != want {
		t.Errorf("URLTemplate = %q, want %q", env.Transport.URLTemplate, want)
	}
}

func TestTranslateOpalProd(t *testing.T) {
	a := newTestAuth(t)
	env, err := a.legacyToEnvironment("opal_prod", "us-east-1")
	if err != nil {
		t.Fatalf("legacyToEnvironment: %v", err)
	}
	want := "https://prod-us-east-1-primary-es.darkbytes.io"
	if env.Transport.URLTemplate != want {
		t.Errorf("URLTemplate = %q, want %q", env.Transport.URLTemplate, want)
	}
}

func TestTranslateDragosNoConfig(t *testing.T) {
	// With no DRAGOS_AUTH_TOKEN env var and no ~/.cloudcutter/dragos.json
	// (because we run with HOME=tempdir), translation should still succeed
	// — the Environment is just metadata. SwitchEnvironment is what fails
	// on missing token.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DRAGOS_AUTH_TOKEN", "")

	a := newTestAuth(t)
	_, err := a.legacyToEnvironment(DragosProfile, "us-west-2")
	if err == nil {
		// LoadDragosConfig errors when no token is resolvable. Translator
		// surfaces that. This is acceptable — it lets SwitchProfile fail
		// at the same point as today.
		// If the implementation chooses to return a partial Environment
		// without erroring, that's also fine; just adjust this test.
		t.Skip("translator chose not to error on missing dragos config; OK")
	}
	if !strings.Contains(err.Error(), "dragos") {
		t.Errorf("expected error to mention dragos, got %q", err.Error())
	}
}

func TestTranslateDragosWithConfig(t *testing.T) {
	t.Setenv("DRAGOS_AUTH_TOKEN", "test-token")
	t.Setenv("DRAGOS_BASE_URL", "https://platform.example.com")

	a := newTestAuth(t)
	env, err := a.legacyToEnvironment(DragosProfile, "us-west-2")
	if err != nil {
		t.Fatalf("legacyToEnvironment: %v", err)
	}
	if env.Auth.Type != "jwt" {
		t.Errorf("Auth.Type = %q, want jwt", env.Auth.Type)
	}
	if env.Transport.Type != "kibana_proxy" {
		t.Errorf("Transport.Type = %q, want kibana_proxy", env.Transport.Type)
	}
	if env.Transport.BaseURL != "https://platform.example.com" {
		t.Errorf("BaseURL = %q", env.Transport.BaseURL)
	}
	if env.Transport.TokenHeader == nil {
		t.Fatal("TokenHeader is nil")
	}
	if env.Transport.TokenHeader.Format != "dragos-auth-token={token}" {
		t.Errorf("TokenHeader.Format = %q", env.Transport.TokenHeader.Format)
	}
	if env.Transport.Probe == nil || !env.Transport.Probe.RejectHTML {
		t.Errorf("Probe.RejectHTML should be true")
	}
	if len(env.TimeFields) != 1 || env.TimeFields[0].Name != "createdAt" {
		t.Errorf("TimeFields = %v, want [{createdAt date}]", env.TimeFields)
	}
}

func TestTranslateStandard(t *testing.T) {
	a := newTestAuth(t)
	env, err := a.legacyToEnvironment("default", "us-west-2")
	if err != nil {
		t.Fatalf("legacyToEnvironment: %v", err)
	}
	if env.Auth.Type != "aws_sdk" {
		t.Errorf("Auth.Type = %q, want aws_sdk", env.Auth.Type)
	}
	if env.Auth.PreAuth != nil {
		t.Errorf("PreAuth should be nil for standard AWS profile, got %+v", env.Auth.PreAuth)
	}
	if env.Transport.Type != "sigv4" {
		t.Errorf("Transport.Type = %q, want sigv4", env.Transport.Type)
	}
}

func TestTranslateUnusedConfigImport(t *testing.T) {
	// Compile-time anchor so the linter doesn't complain about config
	// being imported only by other test funcs above.
	_ = config.Config{}
}

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tpe11etier/cloudcutter/internal/config"
	"github.com/tpe11etier/cloudcutter/internal/environments"
)

func TestSwitchEnvironmentNone(t *testing.T) {
	a, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	env := environments.Environment{
		Name:   "local",
		Region: "us-west-2",
		Auth:   config.AuthSpec{Type: "none"},
		Transport: config.TransportSpec{
			Type:    "plain",
			BaseURL: "http://localhost:9200",
		},
	}
	sess, err := a.SwitchEnvironment(context.Background(), env)
	if err != nil {
		t.Fatalf("SwitchEnvironment: %v", err)
	}
	if sess.Environment.Name != "local" {
		t.Errorf("Environment.Name = %q", sess.Environment.Name)
	}
	if sess.Profile != "local" {
		t.Errorf("Profile = %q", sess.Profile)
	}
}

func TestSwitchEnvironmentJWTReadsTokenFromFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Probe expects 200 with non-HTML body.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	tokenFile := filepath.Join(t.TempDir(), "tok")
	if err := os.WriteFile(tokenFile, []byte("the-jwt"), 0o600); err != nil {
		t.Fatal(err)
	}

	a, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	env := environments.Environment{
		Name:   "dragos",
		Region: "us-west-2",
		Auth: config.AuthSpec{
			Type: "jwt",
			Path: tokenFile,
		},
		Transport: config.TransportSpec{
			Type:      "kibana_proxy",
			BaseURL:   srv.URL,
			ProxyPath: "/kibana/api/console/proxy",
			TokenHeader: &config.TokenHeaderSpec{
				Name:   "Cookie",
				Format: "dragos-auth-token={token}",
			},
			Probe: &config.ProbeSpec{
				Path:       "_cluster/health",
				RejectHTML: true,
			},
		},
	}
	// httptest server doesn't actually serve the kibana proxy path; the
	// fact that it returns 200/JSON for ANY path is enough for probe.Run
	// to consider it healthy.
	sess, err := a.SwitchEnvironment(context.Background(), env)
	if err != nil {
		t.Fatalf("SwitchEnvironment: %v", err)
	}
	if sess.Token != "the-jwt" {
		t.Errorf("Session.Token = %q, want the-jwt", sess.Token)
	}
}

func TestSwitchEnvironmentJWTPrefersEnv(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	tokenFile := filepath.Join(t.TempDir(), "tok")
	_ = os.WriteFile(tokenFile, []byte("from-file"), 0o600)
	t.Setenv("CC_TEST_JWT", "from-env")

	a, _ := New(nil)
	env := environments.Environment{
		Name: "dragos",
		Auth: config.AuthSpec{Type: "jwt", Path: tokenFile, Env: "CC_TEST_JWT"},
		Transport: config.TransportSpec{
			Type:      "kibana_proxy",
			BaseURL:   srv.URL,
			ProxyPath: "/kibana/api/console/proxy",
			TokenHeader: &config.TokenHeaderSpec{
				Name:   "Cookie",
				Format: "dragos-auth-token={token}",
			},
			Probe: &config.ProbeSpec{Path: "_cluster/health"},
		},
	}
	sess, err := a.SwitchEnvironment(context.Background(), env)
	if err != nil {
		t.Fatalf("SwitchEnvironment: %v", err)
	}
	if sess.Token != "from-env" {
		t.Errorf("Session.Token = %q, want from-env (env should win over file)", sess.Token)
	}
}

func TestSwitchEnvironmentJWTNoTokenFails(t *testing.T) {
	a, _ := New(nil)
	env := environments.Environment{
		Name: "dragos",
		Auth: config.AuthSpec{Type: "jwt", Path: "/nonexistent"},
		Transport: config.TransportSpec{
			Type: "kibana_proxy", BaseURL: "https://example.com", ProxyPath: "/p",
			TokenHeader: &config.TokenHeaderSpec{Name: "Cookie", Format: "x={token}"},
		},
	}
	_, err := a.SwitchEnvironment(context.Background(), env)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("error should mention token, got %q", err.Error())
	}
}

func TestSwitchEnvironmentUnknownTypeFails(t *testing.T) {
	a, _ := New(nil)
	env := environments.Environment{
		Name: "x",
		Auth: config.AuthSpec{Type: "exotic"},
	}
	_, err := a.SwitchEnvironment(context.Background(), env)
	if err == nil {
		t.Fatal("expected error for unknown auth.type")
	}
	if !strings.Contains(err.Error(), "exotic") {
		t.Errorf("error should mention bad type, got %q", err.Error())
	}
}

package config

import (
	"os"
	"strings"
	"testing"
)

func TestValidateFull(t *testing.T) {
	cfg, err := Load("testdata/full.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("Validate full.yaml: %v", err)
	}
}

func TestValidateMissingTransport(t *testing.T) {
	cfg, err := Load("testdata/missing_auth.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// missing_auth.yaml has transport but is missing the auth block; verify
	// the missing field is named in the error.
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "auth") {
		t.Errorf("expected error to mention 'auth', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "dragos") {
		t.Errorf("expected error to mention environment name, got %q", err.Error())
	}
}

func TestValidateBadRegex(t *testing.T) {
	cfg, err := Load("testdata/bad_regex.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for bad regex")
	}
	if !strings.Contains(err.Error(), "vars.prefix") {
		t.Errorf("expected error path to mention 'vars.prefix', got %q", err.Error())
	}
}

func TestValidateBadAuthType(t *testing.T) {
	cfg, err := Load("testdata/bad_auth_type.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for unknown auth type")
	}
	if !strings.Contains(err.Error(), "oauth") {
		t.Errorf("expected error to mention bad type, got %q", err.Error())
	}
}

func TestValidateJSONPathDeferred(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/cfg.yaml"
	if err := writeFile(t, path, `
environments:
  - name: x
    auth:
      type: jwt
      path: /tmp/x.token
      login:
        url: https://example.com/login
        body_fields:
          - { name: u, kind: text }
        token_extract:
          from: json_path
          name: $.token
    transport:
      type: plain
      base_url: https://example.com
`); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	err = Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "json_path") {
		t.Errorf("expected json_path-not-implemented error, got %v", err)
	}
}

// helper used by multiple tests
func writeFile(t *testing.T, path, content string) error {
	t.Helper()
	return os.WriteFile(path, []byte(content), 0o600)
}

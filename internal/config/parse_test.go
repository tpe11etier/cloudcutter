package config

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestLoadFull(t *testing.T) {
	cfg, err := Load("testdata/full.yaml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DefaultAWSBackend == nil {
		t.Fatal("DefaultAWSBackend is nil")
	}
	if cfg.DefaultAWSBackend.Auth.Type != "aws_sdk" {
		t.Errorf("DefaultAWSBackend.Auth.Type = %q, want aws_sdk", cfg.DefaultAWSBackend.Auth.Type)
	}
	if got := cfg.DefaultAWSBackend.Transport.URLTemplate; got != "https://{prefix}-{region}-primary-es.darkbytes.io" {
		t.Errorf("URLTemplate = %q", got)
	}
	if len(cfg.DefaultAWSBackend.Vars["role_id"]) != 2 {
		t.Errorf("vars.role_id len = %d, want 2", len(cfg.DefaultAWSBackend.Vars["role_id"]))
	}
	if cfg.DefaultAWSBackend.Vars["role_id"][0].Env != "OPAL_DEV_ROLE_ID" {
		t.Errorf("vars.role_id[0].Env = %q", cfg.DefaultAWSBackend.Vars["role_id"][0].Env)
	}
	if len(cfg.Environments) != 2 {
		t.Fatalf("Environments len = %d, want 2", len(cfg.Environments))
	}

	dragos := cfg.Environments[0]
	if dragos.Name != "dragos" {
		t.Errorf("Environments[0].Name = %q", dragos.Name)
	}
	if dragos.Auth.Type != "jwt" {
		t.Errorf("dragos auth.type = %q", dragos.Auth.Type)
	}
	if dragos.Auth.Login == nil {
		t.Fatal("dragos auth.login is nil")
	}
	if dragos.Auth.Login.TokenExtract.From != "cookie" {
		t.Errorf("token_extract.from = %q", dragos.Auth.Login.TokenExtract.From)
	}
	if dragos.Transport.TokenHeader == nil {
		t.Fatal("dragos transport.token_header is nil")
	}
	if dragos.Transport.TokenHeader.Format != "dragos-auth-token={token}" {
		t.Errorf("token_header.format = %q", dragos.Transport.TokenHeader.Format)
	}
}

func TestLoadEmpty(t *testing.T) {
	cfg, err := Load("testdata/empty.yaml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DefaultAWSBackend != nil {
		t.Errorf("expected nil DefaultAWSBackend, got %+v", cfg.DefaultAWSBackend)
	}
	if len(cfg.Environments) != 0 {
		t.Errorf("expected 0 environments, got %d", len(cfg.Environments))
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("testdata/this-does-not-exist.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist wrapped, got %v", err)
	}
}

func TestLoadSyntaxError(t *testing.T) {
	_, err := Load("testdata/syntax_error.yaml")
	if err == nil {
		t.Fatal("expected syntax error")
	}
	// yaml.v3 includes a line number in its errors; surface it.
	if !strings.Contains(err.Error(), "line ") {
		t.Errorf("expected line-number reference in error, got %q", err.Error())
	}
}

func TestLoadEnvExpansion(t *testing.T) {
	t.Setenv("CC_INDEX_OVERRIDE", "custom-*")

	dir := t.TempDir()
	path := dir + "/cfg.yaml"
	if err := os.WriteFile(path, []byte(`
environments:
  - name: x
    auth: { type: none }
    transport: { type: plain, base_url: http://localhost:9200 }
    index_pattern: "${CC_INDEX_OVERRIDE}"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Environments[0].IndexPattern != "custom-*" {
		t.Errorf("index_pattern = %q, want custom-*", cfg.Environments[0].IndexPattern)
	}
}

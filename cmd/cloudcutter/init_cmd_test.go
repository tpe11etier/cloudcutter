package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInitFromLegacyConfigs(t *testing.T) {
	home := t.TempDir()
	cloudcutterDir := filepath.Join(home, ".cloudcutter")
	if err := os.MkdirAll(cloudcutterDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// Existing dragos.json with a token + base URL.
	dragos := `{
  "baseUrl": "https://platform.example.com",
  "indexPattern": "events*",
  "kbnVersion": "8.19.2",
  "providerId": "00000000-0000-0000-0000-000000000002",
  "authToken": "abc.def.ghi"
}`
	if err := os.WriteFile(filepath.Join(cloudcutterDir, "dragos.json"), []byte(dragos), 0o600); err != nil {
		t.Fatal(err)
	}

	// Existing opal.json with two role IDs.
	opal := `{
  "environments": {
    "dev":  { "roleId": "DEV-ROLE-UUID",  "profileTags": ["dev",  "opal_dev"] },
    "prod": { "roleId": "PROD-ROLE-UUID", "profileTags": ["prod", "opal_prod"] }
  }
}`
	if err := os.WriteFile(filepath.Join(cloudcutterDir, "opal.json"), []byte(opal), 0o600); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(cloudcutterDir, "config.yaml")
	tokenPath := filepath.Join(cloudcutterDir, "dragos.token")

	if err := runInit(home, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	cfgBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	cfg := string(cfgBytes)
	if !strings.Contains(cfg, "default_aws_backend:") {
		t.Errorf("expected default_aws_backend block, got: %q", cfg)
	}
	if !strings.Contains(cfg, "OPAL_DEV_ROLE_ID") && !strings.Contains(cfg, "DEV-ROLE-UUID") {
		t.Errorf("expected role-id reference, got: %q", cfg)
	}
	if !strings.Contains(cfg, "name: dragos") {
		t.Errorf("expected dragos environment, got: %q", cfg)
	}
	if !strings.Contains(cfg, "platform.example.com") {
		t.Errorf("expected migrated baseURL, got: %q", cfg)
	}

	tok, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	if strings.TrimSpace(string(tok)) != "abc.def.ghi" {
		t.Errorf("token contents = %q, want %q", strings.TrimSpace(string(tok)), "abc.def.ghi")
	}

	info, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("token file mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestRunInitRefusesIfConfigExists(t *testing.T) {
	home := t.TempDir()
	cloudcutterDir := filepath.Join(home, ".cloudcutter")
	if err := os.MkdirAll(cloudcutterDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(cloudcutterDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("# existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := runInit(home, false)
	if err == nil {
		t.Fatal("expected error when config exists and force=false")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got %q", err.Error())
	}

	got, _ := os.ReadFile(configPath)
	if string(got) != "# existing\n" {
		t.Errorf("config was overwritten: %q", got)
	}
}

func TestRunInitForceOverwrites(t *testing.T) {
	home := t.TempDir()
	cloudcutterDir := filepath.Join(home, ".cloudcutter")
	if err := os.MkdirAll(cloudcutterDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(cloudcutterDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("# old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := runInit(home, true); err != nil {
		t.Fatalf("runInit force: %v", err)
	}

	got, _ := os.ReadFile(configPath)
	if string(got) == "# old\n" {
		t.Errorf("config was not overwritten despite --force")
	}
}

func TestRunInitNoLegacyFiles(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".cloudcutter"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := runInit(home, false); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	cfg, err := os.ReadFile(filepath.Join(home, ".cloudcutter", "config.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// With no legacy data to migrate, runInit writes the same starter
	// content as first-run.
	if !strings.Contains(string(cfg), "name: local") {
		t.Errorf("expected starter content with 'name: local', got %q", string(cfg))
	}
}

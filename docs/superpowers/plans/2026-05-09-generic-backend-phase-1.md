# Generic Backend — Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the YAML config loader, environment resolver, first-run starter-write, and `cloudcutter init` migration tool — without changing any user-visible behavior in the running app.

**Architecture:** Two new packages (`internal/config`, `internal/environments`) and one new cobra subcommand (`cloudcutter init`). Phase 1 has **no callers in the existing code paths** — the new packages are wired up only by tests and the init subcommand. The running app still uses `auth.DragosConfig`, `auth.OpalConfig`, and `auth.SwitchProfile`'s vendor-name branches exactly as before. Phase 2 (next plan) is where the auth path starts consuming `Environment`.

**Tech Stack:** Go 1.x, `gopkg.in/yaml.v3` (already an indirect dep via cobra/viper transitives — vendored in phase 1's first task), existing `github.com/spf13/cobra` and `gopkg.in/ini.v1` already in vendor.

**Spec:** `docs/superpowers/specs/2026-05-09-generic-backend-design.md` — particularly the "Config schema", "Go types & key interfaces", "First-run experience", and "`cloudcutter init`" sections.

---

## File structure

**New packages**

| File | Responsibility |
|---|---|
| `internal/config/types.go` | All public struct types (Config, EnvironmentSpec, AuthSpec, TransportSpec, etc.) — pure data. |
| `internal/config/expand.go` | `${VAR}` and `${VAR:-default}` env expansion. Pure string func. |
| `internal/config/parse.go` | `Load(path string) (*Config, error)` — read YAML, expand env vars, return parsed struct. Surfaces yaml line numbers in errors. |
| `internal/config/validate.go` | `Validate(*Config) error` — schema rules: required fields per `auth.type` / `transport.type`, regex compileability in `vars`, etc. |
| `internal/config/firstrun.go` | `EnsureExists() (wrote bool, err error)` — if `~/.cloudcutter/config.yaml` is absent, write the commented starter and return wrote=true. |
| `internal/config/starter.go` | Embeds the starter `config.yaml` text via `//go:embed`. |
| `internal/config/testdata/*.yaml` | Test fixtures for parse + validate. |
| `internal/environments/environments.go` | `EnvironmentSpec.Materialize(region) (Environment, error)`, `Environment` struct. |
| `internal/environments/resolver.go` | `Resolver` interface + concrete impl: `List()`, `Resolve(name)`. Wraps a `*config.Config` + AWS-profile list. |
| `internal/environments/aws.go` | `DiscoverAWSProfiles(homeDir string) ([]string, error)` — reads `~/.aws/credentials`. |
| `internal/environments/testdata/*` | Fixtures. |

**Modified files**

| File | Change |
|---|---|
| `cmd/cloudcutter/main.go` | Add `cobra` subcommand `init`; existing root command behavior unchanged. |
| `cmd/cloudcutter/init_cmd.go` (NEW) | `cloudcutter init` implementation — reads legacy configs + env vars, writes `~/.cloudcutter/config.yaml` and `~/.cloudcutter/dragos.token`. Has `--force` flag. |
| `go.mod`, `go.sum`, `vendor/` | Add `gopkg.in/yaml.v3` if not already vendored. |

**No changes** in this phase to: `internal/auth/*`, `internal/services/*`, `internal/ui/*`, `cmd/cloudcutter/main.go`'s `runApplication()` body. That comes in phase 2+.

---

## Tasks

### Task 1: Vendor `gopkg.in/yaml.v3` if not already present

**Files:**
- Check: `vendor/modules.txt`
- Modify (if needed): `go.mod`, `go.sum`, `vendor/`

- [ ] **Step 1: Check whether yaml.v3 is already vendored**

Run: `grep '^# gopkg.in/yaml.v3' vendor/modules.txt || echo NOT_VENDORED`

If output is `NOT_VENDORED`, proceed. If it shows the version (e.g., `# gopkg.in/yaml.v3 v3.0.1`), skip steps 2-4 and go to step 5 (just commit nothing, this task is a no-op).

- [ ] **Step 2: Add yaml.v3 dependency**

Run: `go get gopkg.in/yaml.v3@v3.0.1`

Expected: go.mod gets a `require gopkg.in/yaml.v3 v3.0.1` line.

- [ ] **Step 3: Re-vendor**

Run: `go mod tidy && go mod vendor`

- [ ] **Step 4: Confirm yaml.v3 is now vendored**

Run: `grep '^# gopkg.in/yaml.v3' vendor/modules.txt`

Expected: prints a line like `# gopkg.in/yaml.v3 v3.0.1`.

- [ ] **Step 5: Verify build still works**

Run: `go build -mod=vendor ./...`

Expected: succeeds with no output.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum vendor/
git commit -m "Vendor gopkg.in/yaml.v3 for config loader"
```

If yaml.v3 was already vendored, skip this commit.

---

### Task 2: `internal/config/types.go` — declare all schema types

**Files:**
- Create: `internal/config/types.go`

- [ ] **Step 1: Create the types file**

Write `internal/config/types.go`:

```go
// Package config models the user-edited ~/.cloudcutter/config.yaml that
// describes how cloudcutter authenticates to and queries each backend
// environment. Phase 1 only loads + validates the config; runtime
// consumption arrives in phase 2.
package config

// Config is the top-level YAML document.
type Config struct {
	DefaultAWSBackend *EnvironmentTemplate `yaml:"default_aws_backend,omitempty"`
	Environments      []EnvironmentSpec    `yaml:"environments,omitempty"`
}

// EnvironmentTemplate is applied to every auto-discovered ~/.aws profile
// that doesn't have an explicit override under `environments`.
type EnvironmentTemplate struct {
	Vars         map[string][]VarRule `yaml:"vars,omitempty"`
	Auth         AuthSpec             `yaml:"auth"`
	Transport    TransportSpec        `yaml:"transport"`
	IndexPattern string               `yaml:"index_pattern,omitempty"`
	TimeFields   []TimeField          `yaml:"time_fields,omitempty"`
}

// EnvironmentSpec is one entry under `environments`. Region-agnostic — it
// gets materialized into an Environment with substitutions resolved at
// switch-profile time.
type EnvironmentSpec struct {
	Name         string               `yaml:"name"`
	Vars         map[string][]VarRule `yaml:"vars,omitempty"`
	Auth         AuthSpec             `yaml:"auth"`
	Transport    TransportSpec        `yaml:"transport"`
	IndexPattern string               `yaml:"index_pattern,omitempty"`
	TimeFields   []TimeField          `yaml:"time_fields,omitempty"`
}

// AuthSpec describes how to obtain credentials. Type chooses the dispatch:
//
//	none    — no auth (e.g. local docker)
//	aws_sdk — load AWS SDK creds; PreAuth optionally runs first
//	jwt     — fetch a token from env > path > login modal, attached via
//	          the transport's TokenHeader
type AuthSpec struct {
	Type    string       `yaml:"type"`
	Path    string       `yaml:"path,omitempty"`
	Env     string       `yaml:"env,omitempty"`
	Login   *LoginSpec   `yaml:"login,omitempty"`
	PreAuth *PreAuthSpec `yaml:"pre_auth,omitempty"`
}

// PreAuthSpec runs a shell command before AWS SDK auth (used for Opal).
type PreAuthSpec struct {
	Command              []string `yaml:"command"`
	DetectSessionExpired []string `yaml:"detect_session_expired,omitempty"`
}

// LoginSpec describes the in-app login form for jwt auth.
type LoginSpec struct {
	URL          string            `yaml:"url"`
	BodyFormat   string            `yaml:"body_format,omitempty"` // "json" (default) | "form"
	BodyFields   []FormField       `yaml:"body_fields"`
	Query        map[string]string `yaml:"query,omitempty"`
	TokenExtract TokenExtractSpec  `yaml:"token_extract"`
}

type FormField struct {
	Name string `yaml:"name"`
	Kind string `yaml:"kind"` // "text" | "password"
}

type TokenExtractSpec struct {
	From string `yaml:"from"` // "cookie" | "header" | "json_path"
	Name string `yaml:"name"`
}

// TransportSpec describes how each ES request is wrapped.
type TransportSpec struct {
	Type        string            `yaml:"type"` // "plain" | "sigv4" | "kibana_proxy"
	BaseURL     string            `yaml:"base_url,omitempty"`
	URLTemplate string            `yaml:"url_template,omitempty"` // for sigv4
	Service     string            `yaml:"service,omitempty"`      // for sigv4
	ProxyPath   string            `yaml:"proxy_path,omitempty"`   // for kibana_proxy
	TokenHeader *TokenHeaderSpec  `yaml:"token_header,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty"`
	Probe       *ProbeSpec        `yaml:"probe,omitempty"`
}

type TokenHeaderSpec struct {
	Name   string `yaml:"name"`
	Format string `yaml:"format"` // includes literal "{token}"
}

type ProbeSpec struct {
	Path       string `yaml:"path"`
	RejectHTML bool   `yaml:"reject_html,omitempty"`
}

type TimeField struct {
	Name   string `yaml:"name"`
	Format string `yaml:"format"` // "unix" | "unix_ms" | "date"
}

// VarRule is one entry inside a `vars[key]` list. First match (against the
// AWS profile name being resolved) wins. Either Value or Env must be set;
// Env names an environment variable to read.
type VarRule struct {
	Match string `yaml:"match"`
	Value string `yaml:"value,omitempty"`
	Env   string `yaml:"env,omitempty"`
}
```

- [ ] **Step 2: Verify the package compiles**

Run: `go build -mod=vendor ./internal/config`

Expected: succeeds with no output.

- [ ] **Step 3: Commit**

```bash
git add internal/config/types.go
git commit -m "Add config schema types"
```

---

### Task 3: `${VAR}` / `${VAR:-default}` env expansion

**Files:**
- Create: `internal/config/expand.go`
- Test: `internal/config/expand_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/config/expand_test.go`:

```go
package config

import (
	"testing"
)

func TestExpandEnv(t *testing.T) {
	t.Setenv("CC_TEST_A", "alpha")
	t.Setenv("CC_TEST_EMPTY", "")

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no vars", "plain string", "plain string"},
		{"basic var", "x=${CC_TEST_A}", "x=alpha"},
		{"unset, no default", "x=${CC_TEST_NOPE}", "x="},
		{"unset, with default", "x=${CC_TEST_NOPE:-fallback}", "x=fallback"},
		{"empty value treats as set", "x=${CC_TEST_EMPTY:-fallback}", "x=fallback"},
		{"two vars in one string", "${CC_TEST_A}-${CC_TEST_NOPE:-z}", "alpha-z"},
		{"adjacent literals around var", "[${CC_TEST_A}]", "[alpha]"},
		{"escaped dollar passes through", "$$VAR", "$$VAR"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExpandEnv(c.in)
			if got != c.want {
				t.Errorf("ExpandEnv(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `go test -mod=vendor ./internal/config -run TestExpandEnv`

Expected: FAIL — `undefined: ExpandEnv`.

- [ ] **Step 3: Implement `ExpandEnv`**

Write `internal/config/expand.go`:

```go
package config

import (
	"os"
	"regexp"
	"strings"
)

// ExpandEnv replaces ${VAR} and ${VAR:-default} sequences in s with the
// value of VAR (empty string when unset and no default; the default when
// unset OR set-but-empty). Literal `$$` passes through unchanged so users
// can write a literal dollar-sign without escaping.
//
// Anything else starting with `$` is left alone. This is intentionally
// narrower than os.ExpandEnv, which matches `$VAR` without braces and
// surprises users who write paths like `~/$HOME-of-fame`.
func ExpandEnv(s string) string {
	// Replace `$$` with a sentinel that won't appear in input, run the
	// expansion, then restore.
	const sentinel = "\x00CC_DOLLAR\x00"
	s = strings.ReplaceAll(s, "$$", sentinel)
	s = envVarRE.ReplaceAllStringFunc(s, func(match string) string {
		// match looks like ${NAME} or ${NAME:-default}
		inner := match[2 : len(match)-1]
		name, def, hasDef := strings.Cut(inner, ":-")
		val, ok := os.LookupEnv(name)
		if !ok || val == "" {
			if hasDef {
				return def
			}
			return ""
		}
		return val
	})
	return strings.ReplaceAll(s, sentinel, "$$")
}

var envVarRE = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*(?::-[^}]*)?\}`)
```

- [ ] **Step 4: Run the test to confirm it passes**

Run: `go test -mod=vendor ./internal/config -run TestExpandEnv -v`

Expected: PASS for all 8 sub-tests.

- [ ] **Step 5: Commit**

```bash
git add internal/config/expand.go internal/config/expand_test.go
git commit -m "Add ${VAR} and ${VAR:-default} env expansion"
```

---

### Task 4: YAML loader — `Load(path) (*Config, error)`

**Files:**
- Create: `internal/config/parse.go`
- Test: `internal/config/parse_test.go`
- Test fixtures: `internal/config/testdata/full.yaml`, `internal/config/testdata/empty.yaml`, `internal/config/testdata/syntax_error.yaml`

- [ ] **Step 1: Write the test fixture `internal/config/testdata/full.yaml`**

```yaml
default_aws_backend:
  vars:
    role_id:
      - { match: "^opal_dev$",  env: OPAL_DEV_ROLE_ID }
      - { match: "^opal_prod$", env: OPAL_PROD_ROLE_ID }
    prefix:
      - { match: "^opal_dev$",  value: dev }
      - { match: "^opal_prod$", value: prod }
  auth:
    type: aws_sdk
    pre_auth:
      command: ["opal", "iam-roles:start", "--id", "{role_id}", "--profileName", "{profile}"]
      detect_session_expired:
        - "Enter your email"
        - "session is invalid or expired"
  transport:
    type: sigv4
    service: es
    url_template: "https://{prefix}-{region}-primary-es.darkbytes.io"
  index_pattern: "main-summary-*"
  time_fields:
    - { name: unixTime, format: unix }
    - { name: detectionGeneratedTime, format: unix_ms }

environments:
  - name: dragos
    auth:
      type: jwt
      path: ~/.cloudcutter/dragos.token
      env: DRAGOS_AUTH_TOKEN
      login:
        url: https://platform.dragos.cloud/auth/api/v1/login/password
        body_format: json
        body_fields:
          - { name: username, kind: text }
          - { name: password, kind: password }
        query:
          providerId: "00000000-0000-0000-0000-000000000002"
        token_extract:
          from: cookie
          name: dragos-auth-token
    transport:
      type: kibana_proxy
      base_url: https://platform.dragos.cloud
      proxy_path: /kibana/api/console/proxy
      token_header:
        name: Cookie
        format: "dragos-auth-token={token}"
      headers:
        kbn-xsrf: cloudcutter
        kbn-version: "8.19.2"
      probe:
        path: _cluster/health
        reject_html: true
    index_pattern: "events*"
    time_fields:
      - { name: createdAt, format: date }

  - name: local
    auth: { type: none }
    transport:
      type: plain
      base_url: http://localhost:9200
    time_fields: []
```

- [ ] **Step 2: Write the test fixture `internal/config/testdata/empty.yaml`**

```yaml
# empty — no defaults, no environments
```

- [ ] **Step 3: Write the test fixture `internal/config/testdata/syntax_error.yaml`**

```yaml
environments:
  - name: dragos
    auth:
      type: jwt
    transport
      type: plain
```

(Note: missing colon after `transport` on line 5 — YAML syntax error.)

- [ ] **Step 4: Write the failing test `internal/config/parse_test.go`**

```go
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
```

- [ ] **Step 5: Run the tests to confirm they fail**

Run: `go test -mod=vendor ./internal/config`

Expected: FAIL — `undefined: Load`.

- [ ] **Step 6: Implement `Load`**

Write `internal/config/parse.go`:

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads a YAML config file, expands ${VAR} sequences, and unmarshals
// it into a Config. Wraps os.ErrNotExist for missing files and surfaces
// yaml line numbers in syntax errors.
//
// Load does NOT call Validate; callers should run Validate before using
// the result.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	expanded := ExpandEnv(string(raw))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}
```

- [ ] **Step 7: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/config -v`

Expected: PASS for `TestLoadFull`, `TestLoadEmpty`, `TestLoadMissingFile`, `TestLoadSyntaxError`, `TestLoadEnvExpansion`, plus the existing `TestExpandEnv`.

- [ ] **Step 8: Commit**

```bash
git add internal/config/parse.go internal/config/parse_test.go internal/config/testdata/
git commit -m "Add YAML config loader with env expansion"
```

---

### Task 5: Schema validation — `Validate(*Config) error`

**Files:**
- Create: `internal/config/validate.go`
- Test: `internal/config/validate_test.go`
- Test fixtures: `internal/config/testdata/missing_auth.yaml`, `internal/config/testdata/bad_regex.yaml`, `internal/config/testdata/bad_auth_type.yaml`

- [ ] **Step 1: Write the test fixtures**

`internal/config/testdata/missing_auth.yaml`:

```yaml
environments:
  - name: dragos
    transport: { type: plain, base_url: http://localhost:9200 }
```

`internal/config/testdata/bad_regex.yaml`:

```yaml
default_aws_backend:
  vars:
    prefix:
      - { match: "[unclosed", value: dev }
  auth: { type: aws_sdk }
  transport:
    type: sigv4
    service: es
    url_template: "https://{prefix}-{region}-x.example.com"
```

`internal/config/testdata/bad_auth_type.yaml`:

```yaml
environments:
  - name: x
    auth: { type: oauth }
    transport: { type: plain, base_url: http://localhost:9200 }
```

- [ ] **Step 2: Write the failing test `internal/config/validate_test.go`**

```go
package config

import (
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
```

(Note: `os` will need an import — the test file already imports nothing else, so add `"os"` to the import block.)

Adjust the imports to:

```go
import (
	"os"
	"strings"
	"testing"
)
```

- [ ] **Step 3: Run the tests to confirm they fail**

Run: `go test -mod=vendor ./internal/config`

Expected: FAIL — `undefined: Validate` and the `writeFile` helper compiles only after `os` is imported.

- [ ] **Step 4: Implement `Validate`**

Write `internal/config/validate.go`:

```go
package config

import (
	"fmt"
	"regexp"
	"strings"
)

// Validate checks structural and semantic rules on a parsed Config. It
// does NOT verify that auth would succeed at runtime — only that the
// config is internally consistent and uses known enum values.
func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if cfg.DefaultAWSBackend != nil {
		if err := validateTemplate("default_aws_backend", cfg.DefaultAWSBackend); err != nil {
			return err
		}
	}
	for i := range cfg.Environments {
		env := &cfg.Environments[i]
		if env.Name == "" {
			return fmt.Errorf("environments[%d]: missing required field 'name'", i)
		}
		path := fmt.Sprintf("environments[%s]", env.Name)
		if err := validateAuth(path, &env.Auth); err != nil {
			return err
		}
		if err := validateTransport(path, &env.Transport); err != nil {
			return err
		}
		if err := validateVars(path, env.Vars); err != nil {
			return err
		}
		if err := validateTimeFields(path, env.TimeFields); err != nil {
			return err
		}
	}
	return nil
}

func validateTemplate(path string, t *EnvironmentTemplate) error {
	if err := validateAuth(path, &t.Auth); err != nil {
		return err
	}
	if err := validateTransport(path, &t.Transport); err != nil {
		return err
	}
	if err := validateVars(path, t.Vars); err != nil {
		return err
	}
	return validateTimeFields(path, t.TimeFields)
}

func validateAuth(path string, a *AuthSpec) error {
	switch a.Type {
	case "":
		return fmt.Errorf("%s: missing required field 'auth.type'", path)
	case "none":
		// nothing else required
	case "aws_sdk":
		if a.PreAuth != nil && len(a.PreAuth.Command) == 0 {
			return fmt.Errorf("%s.auth.pre_auth: 'command' is required when pre_auth is set", path)
		}
	case "jwt":
		if a.Path == "" && a.Env == "" && a.Login == nil {
			return fmt.Errorf("%s.auth: jwt requires at least one of 'path', 'env', or 'login'", path)
		}
		if a.Login != nil {
			if a.Login.URL == "" {
				return fmt.Errorf("%s.auth.login: 'url' is required", path)
			}
			if len(a.Login.BodyFields) == 0 {
				return fmt.Errorf("%s.auth.login: 'body_fields' must contain at least one field", path)
			}
			for i, f := range a.Login.BodyFields {
				switch f.Kind {
				case "text", "password":
				case "":
					return fmt.Errorf("%s.auth.login.body_fields[%d]: 'kind' is required", path, i)
				default:
					return fmt.Errorf("%s.auth.login.body_fields[%d]: unknown kind %q (want text|password)", path, i, f.Kind)
				}
			}
			switch a.Login.BodyFormat {
			case "", "json", "form":
			default:
				return fmt.Errorf("%s.auth.login: unknown body_format %q (want json|form)", path, a.Login.BodyFormat)
			}
			switch a.Login.TokenExtract.From {
			case "":
				return fmt.Errorf("%s.auth.login.token_extract: 'from' is required", path)
			case "cookie", "header":
				if a.Login.TokenExtract.Name == "" {
					return fmt.Errorf("%s.auth.login.token_extract: 'name' is required for from=%s", path, a.Login.TokenExtract.From)
				}
			case "json_path":
				return fmt.Errorf("%s.auth.login.token_extract: from=json_path not yet implemented", path)
			default:
				return fmt.Errorf("%s.auth.login.token_extract: unknown from %q (want cookie|header|json_path)", path, a.Login.TokenExtract.From)
			}
		}
	default:
		return fmt.Errorf("%s.auth: unknown type %q (want none|aws_sdk|jwt)", path, a.Type)
	}
	return nil
}

func validateTransport(path string, t *TransportSpec) error {
	switch t.Type {
	case "":
		return fmt.Errorf("%s: missing required field 'transport.type'", path)
	case "plain":
		if t.BaseURL == "" {
			return fmt.Errorf("%s.transport: plain requires 'base_url'", path)
		}
	case "sigv4":
		if t.Service == "" {
			return fmt.Errorf("%s.transport: sigv4 requires 'service'", path)
		}
		if t.URLTemplate == "" {
			return fmt.Errorf("%s.transport: sigv4 requires 'url_template'", path)
		}
	case "kibana_proxy":
		if t.BaseURL == "" {
			return fmt.Errorf("%s.transport: kibana_proxy requires 'base_url'", path)
		}
		if t.ProxyPath == "" {
			return fmt.Errorf("%s.transport: kibana_proxy requires 'proxy_path'", path)
		}
		if t.TokenHeader == nil || t.TokenHeader.Name == "" || t.TokenHeader.Format == "" {
			return fmt.Errorf("%s.transport: kibana_proxy requires 'token_header.name' and 'token_header.format'", path)
		}
	default:
		return fmt.Errorf("%s.transport: unknown type %q (want plain|sigv4|kibana_proxy)", path, t.Type)
	}
	return nil
}

func validateVars(path string, vars map[string][]VarRule) error {
	for key, rules := range vars {
		for i, r := range rules {
			if r.Match == "" {
				return fmt.Errorf("%s.vars.%s[%d]: 'match' is required", path, key, i)
			}
			if _, err := regexp.Compile(r.Match); err != nil {
				return fmt.Errorf("%s.vars.%s[%d]: invalid regex %q: %w", path, key, i, r.Match, err)
			}
			if r.Value == "" && r.Env == "" {
				return fmt.Errorf("%s.vars.%s[%d]: must set either 'value' or 'env'", path, key, i)
			}
			if r.Value != "" && r.Env != "" {
				return fmt.Errorf("%s.vars.%s[%d]: 'value' and 'env' are mutually exclusive", path, key, i)
			}
		}
	}
	return nil
}

func validateTimeFields(path string, fields []TimeField) error {
	for i, f := range fields {
		if f.Name == "" {
			return fmt.Errorf("%s.time_fields[%d]: 'name' is required", path, i)
		}
		switch f.Format {
		case "unix", "unix_ms", "date":
		default:
			return fmt.Errorf("%s.time_fields[%d]: unknown format %q (want unix|unix_ms|date)", path, i, f.Format)
		}
	}
	return nil
}
```

Imports for `validate.go`:

```go
import (
	"fmt"
	"regexp"
)
```

- [ ] **Step 5: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/config -v`

Expected: PASS for all validation tests plus existing tests.

- [ ] **Step 6: Commit**

```bash
git add internal/config/validate.go internal/config/validate_test.go internal/config/testdata/
git commit -m "Add config schema validation"
```

---

### Task 6: `internal/environments/environments.go` — Environment + Materialize

**Files:**
- Create: `internal/environments/environments.go`
- Test: `internal/environments/environments_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/environments/environments_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `go test -mod=vendor ./internal/environments`

Expected: FAIL — `package github.com/tpe11etier/cloudcutter/internal/environments not found` (or method/type missing).

- [ ] **Step 3: Implement `Materialize`**

Write `internal/environments/environments.go`:

```go
// Package environments resolves a config.EnvironmentSpec into a fully-
// substituted Environment ready for Auth + Transport construction. The
// split exists because the user can change region in-app; spec is
// region-agnostic, Environment is region-bound.
package environments

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/tpe11etier/cloudcutter/internal/config"
)

// Environment is a fully-resolved description of one backend the user has
// selected. Every URL/command template is already substituted.
type Environment struct {
	Name         string
	Region       string
	Auth         config.AuthSpec
	Transport    config.TransportSpec
	IndexPattern string
	TimeFields   []config.TimeField
}

// Materialize substitutes {region}, {profile}, and any user-defined vars
// into the spec, returning a region-bound Environment. Region may be
// empty for non-AWS environments; templates that reference {region} when
// region is empty produce an explicit error.
func Materialize(spec config.EnvironmentSpec, region string) (Environment, error) {
	subs := map[string]string{
		"profile": spec.Name,
		"region":  region,
	}

	for key, rules := range spec.Vars {
		val, err := resolveVar(spec.Name, key, rules)
		if err != nil {
			return Environment{}, err
		}
		subs[key] = val
	}

	transport := spec.Transport // copy
	if transport.URLTemplate != "" {
		expanded, err := substitute(spec.Name, transport.URLTemplate, subs)
		if err != nil {
			return Environment{}, err
		}
		transport.URLTemplate = expanded
	}
	if transport.BaseURL != "" {
		expanded, err := substitute(spec.Name, transport.BaseURL, subs)
		if err != nil {
			return Environment{}, err
		}
		transport.BaseURL = expanded
	}

	auth := spec.Auth // copy
	if auth.PreAuth != nil {
		cmd := make([]string, len(auth.PreAuth.Command))
		for i, arg := range auth.PreAuth.Command {
			expanded, err := substitute(spec.Name, arg, subs)
			if err != nil {
				return Environment{}, err
			}
			cmd[i] = expanded
		}
		// Don't mutate the caller's PreAuthSpec; replace with a new value.
		newPre := *auth.PreAuth
		newPre.Command = cmd
		auth.PreAuth = &newPre
	}

	return Environment{
		Name:         spec.Name,
		Region:       region,
		Auth:         auth,
		Transport:    transport,
		IndexPattern: spec.IndexPattern,
		TimeFields:   spec.TimeFields,
	}, nil
}

func resolveVar(profileName, key string, rules []config.VarRule) (string, error) {
	for _, r := range rules {
		matched, err := regexp.MatchString(r.Match, profileName)
		if err != nil {
			return "", fmt.Errorf("vars.%s: invalid regex %q: %w", key, r.Match, err)
		}
		if !matched {
			continue
		}
		if r.Env != "" {
			val := os.Getenv(r.Env)
			if val == "" {
				return "", fmt.Errorf("vars.%s: env %s is unset (matched profile %q)", key, r.Env, profileName)
			}
			return val, nil
		}
		return r.Value, nil
	}
	return "", fmt.Errorf("vars.%s: no rule matched profile %q", key, profileName)
}

func substitute(profileName, template string, subs map[string]string) (string, error) {
	out := template
	matches := varRefRE.FindAllStringSubmatch(template, -1)
	for _, m := range matches {
		key := m[1]
		val, ok := subs[key]
		if !ok || val == "" {
			return "", fmt.Errorf("environment %q: template references {%s} but it isn't defined for this profile", profileName, key)
		}
		out = strings.ReplaceAll(out, m[0], val)
	}
	return out, nil
}

var varRefRE = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)
```

- [ ] **Step 4: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/environments -v`

Expected: PASS for all 5 sub-tests.

- [ ] **Step 5: Commit**

```bash
git add internal/environments/environments.go internal/environments/environments_test.go
git commit -m "Add EnvironmentSpec.Materialize for region+vars substitution"
```

---

### Task 7: `internal/environments/aws.go` — discover AWS profiles

**Files:**
- Create: `internal/environments/aws.go`
- Test: `internal/environments/aws_test.go`

- [ ] **Step 1: Write the failing test `internal/environments/aws_test.go`**

```go
package environments

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDiscoverAWSProfiles(t *testing.T) {
	home := t.TempDir()
	awsDir := filepath.Join(home, ".aws")
	if err := os.MkdirAll(awsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	creds := `[default]
aws_access_key_id = AKIA...
aws_secret_access_key = ...

[opal_dev]
aws_access_key_id = AKIA...
aws_secret_access_key = ...

[profile opal_prod]
aws_access_key_id = AKIA...
aws_secret_access_key = ...
`
	if err := os.WriteFile(filepath.Join(awsDir, "credentials"), []byte(creds), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverAWSProfiles(home)
	if err != nil {
		t.Fatalf("DiscoverAWSProfiles: %v", err)
	}
	sort.Strings(got)
	want := []string{"default", "opal_dev", "opal_prod"}
	if len(got) != len(want) {
		t.Fatalf("got %d profiles (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("profile[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDiscoverAWSProfilesMissingFile(t *testing.T) {
	home := t.TempDir()
	got, err := DiscoverAWSProfiles(home)
	if err != nil {
		t.Fatalf("expected nil error for missing creds file, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 profiles for missing creds file, got %v", got)
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `go test -mod=vendor ./internal/environments -run TestDiscoverAWS`

Expected: FAIL — `undefined: DiscoverAWSProfiles`.

- [ ] **Step 3: Implement `DiscoverAWSProfiles`**

Write `internal/environments/aws.go`:

```go
package environments

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// DiscoverAWSProfiles reads ~/.aws/credentials and returns the list of
// profile names found there. A missing credentials file is treated as
// "no AWS profiles" rather than an error.
//
// homeDir is taken as a parameter so tests can supply a temp directory.
// In production callers, pass os.UserHomeDir()'s result.
func DiscoverAWSProfiles(homeDir string) ([]string, error) {
	path := filepath.Join(homeDir, ".aws", "credentials")
	f, err := ini.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var names []string
	for _, section := range f.Sections() {
		name := section.Name()
		if name == ini.DefaultSection && len(section.Keys()) == 0 {
			continue
		}
		// AWS config files prefix non-default profiles with "profile " in
		// ~/.aws/config but not ~/.aws/credentials. Tolerate both.
		name = strings.TrimPrefix(name, "profile ")
		names = append(names, name)
	}
	return names, nil
}
```

- [ ] **Step 4: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/environments -v`

Expected: all environments tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/environments/aws.go internal/environments/aws_test.go
git commit -m "Add AWS profile discovery from ~/.aws/credentials"
```

---

### Task 8: `internal/environments/resolver.go` — Resolver

**Files:**
- Create: `internal/environments/resolver.go`
- Test: `internal/environments/resolver_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/environments/resolver_test.go`:

```go
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
					Type:    "kibana_proxy",
					BaseURL: "https://platform.dragos.cloud",
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
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `go test -mod=vendor ./internal/environments -run TestResolver`

Expected: FAIL — `undefined: NewResolver`.

- [ ] **Step 3: Implement `Resolver`**

Write `internal/environments/resolver.go`:

```go
package environments

import (
	"fmt"

	"github.com/tpe11etier/cloudcutter/internal/config"
)

// Resolver answers two questions: what environments can the picker show
// (List), and how does a chosen name resolve to a concrete spec (Resolve)?
//
// The picker source is the union of config.Environments names and the
// auto-discovered AWS profile names. Explicit `environments[name=X]`
// entries shadow same-named AWS profiles.
type Resolver struct {
	cfg         *config.Config
	awsProfiles []string

	// Index built once at construction time: name → environment spec.
	// AWS-profile-only entries are NOT pre-built; they're synthesized
	// lazily by Resolve so Vars stay live (could change after Reload).
	explicit map[string]*config.EnvironmentSpec
}

func NewResolver(cfg *config.Config, awsProfiles []string) *Resolver {
	r := &Resolver{cfg: cfg, awsProfiles: awsProfiles}
	r.explicit = make(map[string]*config.EnvironmentSpec, len(cfg.Environments))
	for i := range cfg.Environments {
		env := &cfg.Environments[i]
		r.explicit[env.Name] = env
	}
	return r
}

// List returns the union of explicit environment names and AWS profile
// names. Order is undefined; callers should sort if presenting in UI.
func (r *Resolver) List() []string {
	seen := make(map[string]struct{}, len(r.explicit)+len(r.awsProfiles))
	out := make([]string, 0, len(r.explicit)+len(r.awsProfiles))
	for name := range r.explicit {
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, name := range r.awsProfiles {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// Resolve returns the EnvironmentSpec for a name. Explicit entries win
// over AWS-profile fallback.
func (r *Resolver) Resolve(name string) (config.EnvironmentSpec, error) {
	if spec, ok := r.explicit[name]; ok {
		return *spec, nil
	}
	for _, p := range r.awsProfiles {
		if p != name {
			continue
		}
		if r.cfg.DefaultAWSBackend == nil {
			return config.EnvironmentSpec{}, fmt.Errorf(
				"profile %q has no environment definition. Add it under 'environments' in ~/.cloudcutter/config.yaml or define a default_aws_backend.",
				name)
		}
		return r.specFromTemplate(name, r.cfg.DefaultAWSBackend), nil
	}
	return config.EnvironmentSpec{}, fmt.Errorf("environment %q not found in config or ~/.aws/credentials", name)
}

func (r *Resolver) specFromTemplate(name string, t *config.EnvironmentTemplate) config.EnvironmentSpec {
	return config.EnvironmentSpec{
		Name:         name,
		Vars:         t.Vars,
		Auth:         t.Auth,
		Transport:    t.Transport,
		IndexPattern: t.IndexPattern,
		TimeFields:   t.TimeFields,
	}
}
```

- [ ] **Step 4: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/environments -v`

Expected: PASS for all environments tests.

- [ ] **Step 5: Commit**

```bash
git add internal/environments/resolver.go internal/environments/resolver_test.go
git commit -m "Add Resolver with environments-shadow-AWS-profiles precedence"
```

---

### Task 9: First-run starter — `internal/config/firstrun.go`

**Files:**
- Create: `internal/config/firstrun.go`
- Create: `internal/config/starter.go` (embeds the starter YAML)
- Create: `internal/config/starter.yaml.tmpl` (the actual starter content, embedded via go:embed)
- Test: `internal/config/firstrun_test.go`

- [ ] **Step 1: Write the starter content `internal/config/starter.yaml.tmpl`**

```yaml
# ~/.cloudcutter/config.yaml — generated by cloudcutter on first run.
# Edit and run cloudcutter again.
#
# Documentation: docs/superpowers/specs/2026-05-09-generic-backend-design.md

# Uncomment and edit if you have AWS profiles in ~/.aws/credentials that
# all hit the same Elasticsearch endpoint pattern.
#
# default_aws_backend:
#   vars:
#     prefix:
#       - { match: "^.*$", value: dev }
#   auth:
#     type: aws_sdk
#   transport:
#     type: sigv4
#     service: es
#     url_template: "https://{prefix}-{region}-es.example.com"
#   index_pattern: "logs-*"
#   time_fields:
#     - { name: "@timestamp", format: date }

environments:
  - name: local
    auth: { type: none }
    transport:
      type: plain
      base_url: http://localhost:9200
    time_fields: []

  # Example JWT-based environment. Uncomment, fill in, and rename.
  #
  # - name: my-platform
  #   auth:
  #     type: jwt
  #     path: ~/.cloudcutter/my-platform.token
  #     login:
  #       url: https://my.platform.example.com/auth/login
  #       body_fields:
  #         - { name: username, kind: text }
  #         - { name: password, kind: password }
  #       token_extract:
  #         from: cookie
  #         name: my-auth-token
  #   transport:
  #     type: kibana_proxy
  #     base_url: https://my.platform.example.com
  #     proxy_path: /kibana/api/console/proxy
  #     token_header:
  #       name: Cookie
  #       format: "my-auth-token={token}"
  #     headers:
  #       kbn-xsrf: cloudcutter
  #     probe:
  #       path: _cluster/health
  #       reject_html: true
  #   index_pattern: "logs-*"
  #   time_fields:
  #     - { name: "@timestamp", format: date }
```

- [ ] **Step 2: Write `internal/config/starter.go` to embed it**

```go
package config

import _ "embed"

//go:embed starter.yaml.tmpl
var starterYAML string

// StarterYAML returns the commented starter config written on first run.
func StarterYAML() string { return starterYAML }
```

- [ ] **Step 3: Write the failing test `internal/config/firstrun_test.go`**

```go
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureExistsWritesStarterWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	wrote, err := EnsureExists(path)
	if err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}
	if !wrote {
		t.Error("expected wrote=true for missing file")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if !strings.Contains(string(got), "name: local") {
		t.Errorf("starter should contain 'name: local', got: %q", string(got))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("starter file mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestEnsureExistsDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	existing := []byte("# user content\n")
	if err := os.WriteFile(path, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	wrote, err := EnsureExists(path)
	if err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}
	if wrote {
		t.Error("expected wrote=false for existing file")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(existing) {
		t.Errorf("file changed; got %q, want %q", got, existing)
	}
}

func TestEnsureExistsCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.yaml")

	wrote, err := EnsureExists(path)
	if err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}
	if !wrote {
		t.Error("expected wrote=true")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not written: %v", err)
	}
}
```

- [ ] **Step 4: Run the tests to confirm they fail**

Run: `go test -mod=vendor ./internal/config -run TestEnsureExists`

Expected: FAIL — `undefined: EnsureExists`.

- [ ] **Step 5: Implement `EnsureExists` and `DefaultConfigPath`**

Write `internal/config/firstrun.go`:

```go
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultConfigPath returns ~/.cloudcutter/config.yaml. Returns the empty
// string if the home directory can't be resolved — callers should fail
// loudly in that case.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cloudcutter", "config.yaml")
}

// EnsureExists writes the starter config to path if and only if path does
// not already exist. Returns wrote=true when it created the file. The
// parent directory is created with mode 0700 and the file with mode 0600.
func EnsureExists(path string) (bool, error) {
	if path == "" {
		return false, fmt.Errorf("config path is empty (could not resolve home directory)")
	}

	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(starterYAML), 0o600); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
```

- [ ] **Step 6: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/config -v`

Expected: all `EnsureExists` tests PASS plus existing tests.

- [ ] **Step 7: Commit**

```bash
git add internal/config/firstrun.go internal/config/starter.go internal/config/starter.yaml.tmpl internal/config/firstrun_test.go
git commit -m "Add first-run starter-config write"
```

---

### Task 10: `cloudcutter init` subcommand — migration tool

**Files:**
- Create: `cmd/cloudcutter/init_cmd.go`
- Modify: `cmd/cloudcutter/main.go` (register the subcommand)
- Test: `cmd/cloudcutter/init_cmd_test.go`

- [ ] **Step 1: Inspect the current main.go to find where rootCmd is set up**

Run: `grep -n "rootCmd\|cobra.Command" cmd/cloudcutter/main.go`

Expected output: a few lines showing `rootCmd = &cobra.Command{...}` and `rootCmd.PersistentFlags()`.

- [ ] **Step 2: Write the failing test `cmd/cloudcutter/init_cmd_test.go`**

```go
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
		t.Errorf("expected starter content with 'name: local', got %q", cfg)
	}
}
```

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test -mod=vendor ./cmd/cloudcutter -run TestRunInit`

Expected: FAIL — `undefined: runInit`.

- [ ] **Step 4: Implement the init subcommand**

Write `cmd/cloudcutter/init_cmd.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tpe11etier/cloudcutter/internal/config"
	"gopkg.in/yaml.v3"
)

var (
	initForce bool
	initCmd   = &cobra.Command{
		Use:   "init",
		Short: "Generate ~/.cloudcutter/config.yaml from existing setup",
		Long: `Reads ~/.cloudcutter/opal.json, ~/.cloudcutter/dragos.json, and
relevant env vars, then writes ~/.cloudcutter/config.yaml. Refuses to
overwrite an existing config.yaml unless --force is given.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("could not resolve home directory: %w", err)
			}
			return runInit(home, initForce)
		},
	}
)

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing config.yaml")
	rootCmd.AddCommand(initCmd)
}

// runInit synthesizes ~/.cloudcutter/config.yaml from legacy configs
// under the given home directory. Exposed for testing.
func runInit(home string, force bool) error {
	cloudcutterDir := filepath.Join(home, ".cloudcutter")
	configPath := filepath.Join(cloudcutterDir, "config.yaml")

	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("%s already exists; pass --force to overwrite", configPath)
		}
	}

	if err := os.MkdirAll(cloudcutterDir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", cloudcutterDir, err)
	}

	cfg := config.Config{}
	if opalPath := filepath.Join(cloudcutterDir, "opal.json"); fileExists(opalPath) {
		if err := mergeLegacyOpal(opalPath, &cfg); err != nil {
			return err
		}
	}

	if dragosPath := filepath.Join(cloudcutterDir, "dragos.json"); fileExists(dragosPath) {
		if err := mergeLegacyDragos(dragosPath, &cfg, cloudcutterDir); err != nil {
			return err
		}
	}

	// If we collected nothing, fall back to the starter.
	if cfg.DefaultAWSBackend == nil && len(cfg.Environments) == 0 {
		return os.WriteFile(configPath, []byte(config.StarterYAML()), 0o600)
	}

	// Always include local; harmless and matches starter's helpfulness.
	cfg.Environments = append(cfg.Environments, config.EnvironmentSpec{
		Name: "local",
		Auth: config.AuthSpec{Type: "none"},
		Transport: config.TransportSpec{
			Type:    "plain",
			BaseURL: "http://localhost:9200",
		},
		TimeFields: []config.TimeField{},
	})

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	header := "# ~/.cloudcutter/config.yaml — generated by `cloudcutter init` from\n" +
		"# legacy ~/.cloudcutter/opal.json, ~/.cloudcutter/dragos.json, and env vars.\n" +
		"# Edit freely; cloudcutter does not regenerate this file unless you re-run\n" +
		"# `cloudcutter init --force`.\n\n"
	return os.WriteFile(configPath, append([]byte(header), out...), 0o600)
}

type legacyOpal struct {
	Environments map[string]struct {
		RoleID      string   `json:"roleId"`
		ProfileTags []string `json:"profileTags"`
	} `json:"environments"`
}

func mergeLegacyOpal(path string, cfg *config.Config) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var legacy legacyOpal
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	if cfg.DefaultAWSBackend == nil {
		cfg.DefaultAWSBackend = &config.EnvironmentTemplate{
			Vars: map[string][]config.VarRule{},
			Auth: config.AuthSpec{
				Type: "aws_sdk",
				PreAuth: &config.PreAuthSpec{
					Command: []string{
						"opal", "iam-roles:start", "--id", "{role_id}", "--profileName", "{profile}",
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
				URLTemplate: "https://{prefix}-{region}-primary-es.darkbytes.io",
			},
			IndexPattern: "main-summary-*",
			TimeFields: []config.TimeField{
				{Name: "unixTime", Format: "unix"},
				{Name: "detectionGeneratedTime", Format: "unix_ms"},
			},
		}
	}

	for envName, env := range legacy.Environments {
		// Pick a deterministic env-var name per env.
		envVarName := "OPAL_" + strings.ToUpper(envName) + "_ROLE_ID"

		for _, tag := range env.ProfileTags {
			matchExpr := "^" + tag + "$"
			cfg.DefaultAWSBackend.Vars["role_id"] = append(cfg.DefaultAWSBackend.Vars["role_id"], config.VarRule{
				Match: matchExpr,
				Env:   envVarName,
			})
			cfg.DefaultAWSBackend.Vars["prefix"] = append(cfg.DefaultAWSBackend.Vars["prefix"], config.VarRule{
				Match: matchExpr,
				Value: envName, // "dev", "prod" — matches the URL prefix
			})
		}
	}
	return nil
}

type legacyDragos struct {
	BaseURL      string `json:"baseUrl"`
	IndexPattern string `json:"indexPattern"`
	KbnVersion   string `json:"kbnVersion"`
	ProviderID   string `json:"providerId"`
	AuthToken    string `json:"authToken"`
}

func mergeLegacyDragos(path string, cfg *config.Config, cloudcutterDir string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var legacy legacyDragos
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	tokenPath := filepath.Join(cloudcutterDir, "dragos.token")
	if legacy.AuthToken != "" {
		if err := os.WriteFile(tokenPath, []byte(legacy.AuthToken), 0o600); err != nil {
			return fmt.Errorf("write token to %s: %w", tokenPath, err)
		}
	}

	indexPattern := legacy.IndexPattern
	if indexPattern == "" {
		indexPattern = "events*"
	}
	kbnVersion := legacy.KbnVersion
	if kbnVersion == "" {
		kbnVersion = "8.19.2"
	}
	providerID := legacy.ProviderID
	if providerID == "" {
		providerID = "00000000-0000-0000-0000-000000000002"
	}

	cfg.Environments = append(cfg.Environments, config.EnvironmentSpec{
		Name: "dragos",
		Auth: config.AuthSpec{
			Type: "jwt",
			Path: tokenPath,
			Env:  "DRAGOS_AUTH_TOKEN",
			Login: &config.LoginSpec{
				URL: legacy.BaseURL + "/auth/api/v1/login/password",
				BodyFields: []config.FormField{
					{Name: "username", Kind: "text"},
					{Name: "password", Kind: "password"},
				},
				Query: map[string]string{"providerId": providerID},
				TokenExtract: config.TokenExtractSpec{
					From: "cookie",
					Name: "dragos-auth-token",
				},
			},
		},
		Transport: config.TransportSpec{
			Type:      "kibana_proxy",
			BaseURL:   legacy.BaseURL,
			ProxyPath: "/kibana/api/console/proxy",
			TokenHeader: &config.TokenHeaderSpec{
				Name:   "Cookie",
				Format: "dragos-auth-token={token}",
			},
			Headers: map[string]string{
				"kbn-xsrf":    "cloudcutter",
				"kbn-version": kbnVersion,
			},
			Probe: &config.ProbeSpec{
				Path:       "_cluster/health",
				RejectHTML: true,
			},
		},
		IndexPattern: indexPattern,
		TimeFields: []config.TimeField{
			{Name: "createdAt", Format: "date"},
		},
	})
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
```

- [ ] **Step 5: Verify rootCmd is exported in main.go (it must be a package var the new file can reference)**

Inspect the current `cmd/cloudcutter/main.go`. The variable `rootCmd` is already defined at package level in the existing code (around line 26), so `init_cmd.go`'s `rootCmd.AddCommand(initCmd)` will compile.

If `rootCmd` isn't a package var, that's a refactor: hoist it to package scope. Otherwise no main.go changes are needed for this task.

- [ ] **Step 6: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./cmd/cloudcutter -v`

Expected: PASS for all 4 init tests.

- [ ] **Step 7: Smoke-test the subcommand from the CLI**

Run:

```bash
go run -mod=vendor ./cmd/cloudcutter init --help
```

Expected: cobra prints the init subcommand help with the `--force` flag listed.

- [ ] **Step 8: Commit**

```bash
git add cmd/cloudcutter/init_cmd.go cmd/cloudcutter/init_cmd_test.go
git commit -m "Add cloudcutter init for migrating legacy configs to YAML"
```

---

### Task 11: Wire first-run check into `cloudcutter` startup

**Files:**
- Modify: `cmd/cloudcutter/main.go` (the `runApplication` function)

- [ ] **Step 1: Inspect runApplication**

Run: `grep -n "func runApplication" cmd/cloudcutter/main.go`

Look at the function body (about 30 lines starting at the matched line). The change inserts a first-run check at the very start of the function.

- [ ] **Step 2: Modify `runApplication`**

In `cmd/cloudcutter/main.go`, find the line starting with `func runApplication() {` and immediately after the opening brace, insert the first-run check. The diff:

Replace:

```go
func runApplication() {
	ctx := context.Background()
	app := ui.NewApp()
```

With:

```go
func runApplication() {
	if path := config.DefaultConfigPath(); path != "" {
		wrote, err := config.EnsureExists(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cloudcutter: %v\n", err)
			os.Exit(1)
		}
		if wrote {
			fmt.Fprintf(os.Stderr, "wrote starter config to %s — edit it and run cloudcutter again\n", path)
			os.Exit(0)
		}
	}

	ctx := context.Background()
	app := ui.NewApp()
```

- [ ] **Step 3: Add the `internal/config` import**

Find the import block in `cmd/cloudcutter/main.go` and add:

```go
	"github.com/tpe11etier/cloudcutter/internal/config"
```

(alphabetically sorted with the other `tpe11etier/cloudcutter/internal/...` entries)

- [ ] **Step 4: Build and confirm**

Run: `go build -mod=vendor ./...`

Expected: succeeds with no output.

- [ ] **Step 5: Manual test — first-run path**

Run:

```bash
mv ~/.cloudcutter ~/.cloudcutter.backup-$(date +%s) 2>/dev/null
go run -mod=vendor ./cmd/cloudcutter
```

Expected:
- stderr prints `wrote starter config to <home>/.cloudcutter/config.yaml — edit it and run cloudcutter again`
- exit code 0
- `~/.cloudcutter/config.yaml` exists with starter content

Restore your real config:

```bash
rm -rf ~/.cloudcutter
mv ~/.cloudcutter.backup-* ~/.cloudcutter 2>/dev/null
```

- [ ] **Step 6: Manual test — config-exists path**

With your real `~/.cloudcutter/config.yaml` (or any non-empty file at that path), run:

```bash
go run -mod=vendor ./cmd/cloudcutter
```

Expected: cloudcutter starts normally (no exit-0 message). The TUI launches.

If your real `~/.cloudcutter/dragos.json` is still around but no `config.yaml`, the first-run message appears. That's expected — phase 1 doesn't yet read legacy configs from runApplication. To migrate, the user runs `cloudcutter init` (phase 1's other deliverable). Phase 2's auth refactor is what actually consumes the YAML.

Quit the TUI (Ctrl-C or :exit).

- [ ] **Step 7: Commit**

```bash
git add cmd/cloudcutter/main.go
git commit -m "Wire first-run starter-config check into cloudcutter startup"
```

---

### Task 12: Phase-1 verification + handoff

**Files:** none modified.

- [ ] **Step 1: Full test sweep**

Run: `go test -mod=vendor ./...`

Expected: every package PASSes. Specifically `internal/config`, `internal/environments`, `cmd/cloudcutter`, plus all unchanged packages.

- [ ] **Step 2: `go vet`**

Run: `go vet -mod=vendor ./...`

Expected: no output.

- [ ] **Step 3: gofmt sanity check (only files this phase touched)**

Run: `gofmt -l internal/config internal/environments cmd/cloudcutter`

Expected: no output. If it lists any files, run `gofmt -w` on them and amend the most recent commit.

- [ ] **Step 4: Confirm new packages have no callers in the running app**

Run: `grep -rn '"github.com/tpe11etier/cloudcutter/internal/config"' --include='*.go' .`

Expected lines:
- `cmd/cloudcutter/main.go` (first-run check — added in Task 11)
- `cmd/cloudcutter/init_cmd.go` (the init subcommand — Task 10)
- `cmd/cloudcutter/init_cmd_test.go` (tests — Task 10)
- Files inside `internal/config/` itself

There should be NO references from `internal/auth/`, `internal/services/`, or `internal/ui/`. Phase 2 introduces those.

Run: `grep -rn '"github.com/tpe11etier/cloudcutter/internal/environments"' --include='*.go' .`

Expected: only files inside `internal/environments/` itself. (No callers yet.)

If either grep shows unexpected references, investigate before moving on — phase 2 is supposed to be the first consumer.

- [ ] **Step 5: Tag the phase**

```bash
git tag phase-1-complete
```

- [ ] **Step 6: Note the handoff in the spec**

Edit `docs/superpowers/specs/2026-05-09-generic-backend-design.md` and update the `**Status**:` line at the top:

Change from:

```
**Status**: Approved through brainstorming; awaiting implementation plan.
```

To:

```
**Status**: Phase 1 implemented (commit phase-1-complete). Phase 2 (auth refactor) plan to be drafted next.
```

```bash
git add docs/superpowers/specs/2026-05-09-generic-backend-design.md
git commit -m "Note phase 1 complete in spec status"
```

---

## What's NOT in phase 1 (deferred to later plans)

These are intentionally absent:

- **Auth refactor** — `internal/auth/auth.go`'s `SwitchProfile` switching on `Environment.Auth.Type`. Phase 2 plan.
- **Probe consolidation** — `internal/probe.Run`. Phase 2 plan.
- **Transport refactor** — `dragosTransport` becoming a generic `kibanaProxyTransport`. Phase 3 plan.
- **Manager / View collapse** — `Manager.switchToEnvironment(name)`. Phase 4 plan.
- **Legacy deletion** — removing `DragosConfig`, `OpalConfig`, etc. Phase 5 plan.

After phase 1 ships, draft phase 2 against the spec's "Phase 2" row and what we learned shipping phase 1.

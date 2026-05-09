# Generic Backend — Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor authentication to dispatch on `Environment.Auth.Type` instead of profile name. Consolidate the two existing probes (`auth.ProbeDragos` + `elastic.probeDragosProxy`) into a single `internal/probe.Run`. Behavior is **unchanged from the user's POV**.

**Architecture:** Add `auth.Authenticator.SwitchEnvironment(ctx, env)` as the new dispatch surface. Keep the existing `SwitchProfile(ctx, profile, region)` as a thin wrapper that translates profile-name + legacy configs (`opal.json`, `dragos.json`, env vars) into an `environments.Environment`, then calls `SwitchEnvironment`. The manager and views do not change in this phase — that's phase 4. The translator is a temporary bridge that gets deleted in phase 4 when the manager consumes the Resolver directly.

**Tech Stack:** Go, `net/http`, `httptest`, existing AWS SDK and Kibana-proxy transport.

**Spec:** `docs/superpowers/specs/2026-05-09-generic-backend-design.md` — particularly the "Phase 2" row of the Phasing table, the `Authenticator` and `Session` types in "Go types & key interfaces", and the `internal/probe.Run` reference in "What collapses".

**Status as of plan-write time:** phase 1 is complete (tag `phase-1-complete`). Phase 2 builds on the `internal/config` and `internal/environments` packages added in phase 1 but adds no new YAML-runtime-consumption — that's phase 4.

---

## Out of scope (deferred to phase 4)

- The manager's `switchTo*Profile` funcs are not touched. They keep calling `Authenticator.SwitchProfile(ctx, profile, region)` exactly as today.
- `cmd/cloudcutter/main.go` does not gain a YAML loader call. The first-run hook stays. No `config.Load()` at startup.
- The picker's auto-discovery path (`profile.Selector.discoverProfiles`) is not touched.
- `Manager.switchToEnvironment(name)` is **not** introduced in phase 2. That's phase 4.

The end-of-phase-2 binary is byte-for-byte equivalent to today's binary in the running app's behavior, only with a refactored auth layer underneath.

---

## File structure

**New packages**

| File | Responsibility |
|---|---|
| `internal/probe/probe.go` | `Run(ctx, client, req, rejectHTML) error` — execute one probe HTTP request, fail on 401/403, fail on HTML response when rejectHTML is true, fail on non-200, succeed otherwise. |
| `internal/probe/probe_test.go` | httptest-driven coverage of HTML detection, 401/403, 200 success, non-200, network error. |

**New files inside `internal/auth`**

| File | Responsibility |
|---|---|
| `internal/auth/translator.go` | `(*Authenticator).legacyToEnvironment(profile, region) (environments.Environment, error)` — pure, deterministic mapping from a legacy profile name (plus the Authenticator's `opalConfig` / env-var view) to an `environments.Environment`. Same shape `cloudcutter init` produces. |
| `internal/auth/translator_test.go` | Table-driven tests covering the four legacy shapes (dragos, opal, local, standard). |

**Modified files**

| File | Change |
|---|---|
| `internal/auth/auth.go` | Add `Session.Environment environments.Environment` field (additive — does NOT remove `Session.Dragos`). Add `Authenticator.SwitchEnvironment(ctx, env) (*Session, error)`. Refactor `SwitchProfile` to call `legacyToEnvironment` then `SwitchEnvironment`. The four `authenticate*` helpers shrink or get inlined into `SwitchEnvironment`'s branches. |
| `internal/auth/dragos_login.go` | Delete `ProbeDragos`. Its callers (only `authenticateDragos` today) move to using `probe.Run` via `SwitchEnvironment`'s jwt branch. `LoginWithPassword` is unchanged. |
| `internal/services/elastic/dragos.go` | Replace `probeDragosProxy`'s body with a call to `probe.Run`. Keep the function (it stays a thin wrapper) so the two callsites in `NewDragosService`/`ReinitializeDragos` don't change. |

**No changes** to: `cmd/cloudcutter/*`, `internal/services/services.go`, `internal/services/elastic/elastic.go`, `internal/ui/*`, `internal/config/*`, `internal/environments/*`.

---

## Tasks

### Task 1: `internal/probe/probe.go` — generic HTTP probe

**Files:**
- Create: `internal/probe/probe.go`
- Test: `internal/probe/probe_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/probe/probe_test.go`:

```go
package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunSuccessOnJSON200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"status":"green"}`))
	}))
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL, nil)
	if err := Run(context.Background(), srv.Client(), req, true); err != nil {
		t.Errorf("expected success, got %v", err)
	}
}

func TestRunFailsOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL, nil)
	err := Run(context.Background(), srv.Client(), req, true)
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention 401, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("error should mention 'unauthorized', got %q", err.Error())
	}
}

func TestRunFailsOn403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL, nil)
	err := Run(context.Background(), srv.Client(), req, true)
	if err == nil {
		t.Fatal("expected error on 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention 403, got %q", err.Error())
	}
}

func TestRunFailsOnHTMLByContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`<html><body>login</body></html>`))
	}))
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL, nil)
	err := Run(context.Background(), srv.Client(), req, true)
	if err == nil {
		t.Fatal("expected HTML rejection")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "html") {
		t.Errorf("error should mention HTML, got %q", err.Error())
	}
}

func TestRunFailsOnHTMLByBodySniff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server lies about content-type; body-sniff catches it.
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>login</body></html>`))
	}))
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL, nil)
	err := Run(context.Background(), srv.Client(), req, true)
	if err == nil {
		t.Fatal("expected HTML rejection by body sniff")
	}
}

func TestRunHTMLAllowedWhenRejectHTMLFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`<html></html>`))
	}))
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL, nil)
	if err := Run(context.Background(), srv.Client(), req, false); err != nil {
		t.Errorf("expected HTML to pass when rejectHTML=false, got %v", err)
	}
}

func TestRunFailsOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL, nil)
	err := Run(context.Background(), srv.Client(), req, true)
	if err == nil {
		t.Fatal("expected error on 503")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention 503, got %q", err.Error())
	}
}

func TestRunPropagatesNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // immediately closed; subsequent connections fail

	req, _ := http.NewRequest("POST", srv.URL, nil)
	err := Run(context.Background(), srv.Client(), req, true)
	if err == nil {
		t.Fatal("expected network error")
	}
}

func TestRunNilClientUsesDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL, nil)
	if err := Run(context.Background(), nil, req, false); err != nil {
		t.Errorf("expected nil client to fall back to http.DefaultClient, got %v", err)
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `go test -mod=vendor ./internal/probe`

Expected: FAIL — `package github.com/tpe11etier/cloudcutter/internal/probe is not in std`.

- [ ] **Step 3: Implement `probe.Run`**

Write `internal/probe/probe.go`:

```go
// Package probe runs a single HTTP request to verify that auth and
// connectivity to a backend are working. It does not know what the
// backend's URL or headers should look like; the caller builds the
// request and probe inspects the response.
//
// probe is consumed by both internal/auth (validate-on-switch) and
// internal/services/elastic (validate-on-construct) so the two
// previously-separate probe functions are now one.
package probe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Run executes req via client and returns nil on success.
//
// Failure modes:
//   - 401 / 403 → "probe: NNN unauthorized — token is missing/expired/invalid"
//   - rejectHTML=true and response is HTML (by Content-Type OR body sniff
//     of the first 16 bytes for "<!doctype html"/"<html") →
//     "probe: gateway returned HTML — token rejected or stale"
//   - other non-200 status → "probe: unexpected status NNN"
//   - transport error → "probe: <wrapped>"
//
// rejectHTML exists because some gateways serve their SPA login page at
// HTTP 200 when auth fails, defeating naive status-code checks. The
// Dragos platform behaves this way.
//
// If client is nil, http.DefaultClient is used.
func Run(ctx context.Context, client *http.Client, req *http.Request, rejectHTML bool) error {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("probe: %d unauthorized — token is missing/expired/invalid", resp.StatusCode)
	}

	if rejectHTML && isHTML(resp.Header.Get("Content-Type"), body) {
		return fmt.Errorf("probe: gateway returned HTML — token rejected or stale")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("probe: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func isHTML(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) > 16 {
		trimmed = trimmed[:16]
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html")
}
```

- [ ] **Step 4: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/probe -v`

Expected: PASS for all 9 sub-tests.

- [ ] **Step 5: Commit**

```bash
git add internal/probe/
git commit -m "Add internal/probe.Run for shared HTTP probe checks"
```

---

### Task 2: Replace `auth.ProbeDragos` with `probe.Run`

**Files:**
- Modify: `internal/auth/dragos_login.go` (delete `ProbeDragos`)
- Modify: `internal/auth/auth.go` (`authenticateDragos` calls `probe.Run` directly)

- [ ] **Step 1: Confirm callers**

Run: `grep -rn 'ProbeDragos' --include='*.go' .`

Expected output: `internal/auth/dragos_login.go:NN: func ProbeDragos(...)` and one caller in `internal/auth/auth.go:authenticateDragos`. If there are other callers, escalate as NEEDS_CONTEXT.

- [ ] **Step 2: Update `authenticateDragos` to call `probe.Run` directly**

In `internal/auth/auth.go`, find the `authenticateDragos` function. It currently looks like:

```go
func (a *Authenticator) authenticateDragos(ctx context.Context, region string) (aws.Config, *DragosSession, error) {
	cfg, err := LoadDragosConfig()
	if err != nil {
		return aws.Config{}, nil, err
	}
	a.sendStatus(fmt.Sprintf("Verifying Dragos token at %s", cfg.BaseURL))
	if err := ProbeDragos(ctx, cfg.BaseURL, cfg.AuthToken, cfg.KbnVersion); err != nil {
		return aws.Config{}, nil, err
	}
	return aws.Config{Region: region}, &DragosSession{
		BaseURL:      cfg.BaseURL,
		AuthToken:    cfg.AuthToken,
		IndexPattern: cfg.IndexPattern,
		KbnVersion:   cfg.KbnVersion,
	}, nil
}
```

Replace the `ProbeDragos` call with a request-build + `probe.Run`:

```go
func (a *Authenticator) authenticateDragos(ctx context.Context, region string) (aws.Config, *DragosSession, error) {
	cfg, err := LoadDragosConfig()
	if err != nil {
		return aws.Config{}, nil, err
	}
	a.sendStatus(fmt.Sprintf("Verifying Dragos token at %s", cfg.BaseURL))

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	probeURL := strings.TrimRight(cfg.BaseURL, "/") + "/kibana/api/console/proxy?path=_cluster/health&method=GET"
	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, probeURL, nil)
	if err != nil {
		return aws.Config{}, nil, fmt.Errorf("dragos probe: build request: %w", err)
	}
	req.Header.Set("Cookie", "dragos-auth-token="+cfg.AuthToken)
	req.Header.Set("kbn-xsrf", "cloudcutter")
	req.Header.Set("Content-Type", "application/json")
	if cfg.KbnVersion != "" {
		req.Header.Set("kbn-version", cfg.KbnVersion)
	}

	if err := probe.Run(probeCtx, &http.Client{Timeout: 15 * time.Second}, req, true); err != nil {
		return aws.Config{}, nil, err
	}

	return aws.Config{Region: region}, &DragosSession{
		BaseURL:      cfg.BaseURL,
		AuthToken:    cfg.AuthToken,
		IndexPattern: cfg.IndexPattern,
		KbnVersion:   cfg.KbnVersion,
	}, nil
}
```

- [ ] **Step 3: Add the new imports to `internal/auth/auth.go`**

Add to the import block (alphabetized with the other tpe11etier imports):

```go
	"net/http"
	"strings"
	"time"

	"github.com/tpe11etier/cloudcutter/internal/probe"
```

If `net/http`, `strings`, or `time` are already imported, don't duplicate.

- [ ] **Step 4: Delete `ProbeDragos` from `internal/auth/dragos_login.go`**

Open `internal/auth/dragos_login.go`. Locate the `ProbeDragos` function and its associated documentation comment. Delete the entire function (and the comment). Inspect the imports — if `io`, `net/http`, `strings`, `time`, or `context` are no longer used elsewhere in the file (the file only contains `LoginWithPassword`), prune them. `LoginWithPassword` likely keeps most of these imports.

- [ ] **Step 5: Build and run the auth tests**

Run: `go build -mod=vendor ./...`

Expected: succeeds with no output.

Run: `go test -mod=vendor ./internal/auth ./internal/probe`

Expected: PASS for all auth tests (existing) and probe tests (Task 1). The auth tests include any tests that exercise `authenticateDragos` indirectly via `SwitchProfile`.

- [ ] **Step 6: Confirm no orphaned `ProbeDragos` references**

Run: `grep -rn 'ProbeDragos' --include='*.go' .`

Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add internal/auth/auth.go internal/auth/dragos_login.go
git commit -m "Replace auth.ProbeDragos with probe.Run"
```

---

### Task 3: Replace `elastic.probeDragosProxy` with `probe.Run`

**Files:**
- Modify: `internal/services/elastic/dragos.go`

- [ ] **Step 1: Inspect the current implementation**

Run: `grep -n 'probeDragosProxy' internal/services/elastic/dragos.go`

Expected: function definition + 2 callers (`NewDragosService` and `ReinitializeDragos`).

- [ ] **Step 2: Rewrite `probeDragosProxy` to use `probe.Run`**

In `internal/services/elastic/dragos.go`, replace the body of `probeDragosProxy`:

```go
// probeDragosProxy sends one POST to console/proxy?path=_cluster/health to
// confirm we can reach Kibana with the given cookie. Delegates to
// internal/probe.Run for the response-validation logic, including the
// HTML-on-401 detection that the Dragos edge gateway exhibits.
func probeDragosProxy(ctx context.Context, t *dragosTransport, l *logger.Logger) error {
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	probeURL := t.baseURL + "/kibana/api/console/proxy?path=_cluster/health&method=GET"
	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, probeURL, nil)
	if err != nil {
		return fmt.Errorf("dragos probe: build request: %w", err)
	}
	t.applyHeaders(req)

	if err := probe.Run(probeCtx, t.client, req, true); err != nil {
		return fmt.Errorf("dragos probe: %w", err)
	}
	l.Debug("Dragos console/proxy probe ok")
	return nil
}

// isHTMLResponse is no longer used — probe.Run handles HTML detection.
// Delete the function from this file.
```

Delete the existing `isHTMLResponse` function from `dragos.go` — it's now dead.

- [ ] **Step 3: Update imports**

Add to `dragos.go`'s import block:

```go
	"github.com/tpe11etier/cloudcutter/internal/probe"
```

The existing `io` import may now be unused. Run `gofmt -w` on the file in a later step; the `goimports`-style check is to read the file and confirm `io` is referenced elsewhere. If it isn't, remove the import line.

(After this rewrite the file no longer reads response bodies — `applyHeaders` is the last user of `io`-related types in the probe path.)

- [ ] **Step 4: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds with no output. Compiler will complain about any orphaned imports — fix them by removing the offending lines.

- [ ] **Step 5: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS across all packages. The elastic package has no direct probe tests, but the build is the hard check here.

- [ ] **Step 6: gofmt sanity**

Run: `gofmt -l internal/services/elastic/dragos.go`

Expected: no output. If output, run `gofmt -w` and re-verify.

- [ ] **Step 7: Confirm `isHTMLResponse` is gone**

Run: `grep -rn 'isHTMLResponse' --include='*.go' .`

Expected: no output.

- [ ] **Step 8: Commit**

```bash
git add internal/services/elastic/dragos.go
git commit -m "Replace elastic.probeDragosProxy body with probe.Run"
```

---

### Task 4: Add `Session.Environment` field

**Files:**
- Modify: `internal/auth/auth.go` (just the `Session` struct)

- [ ] **Step 1: Read the current Session struct**

In `internal/auth/auth.go`, find the `Session` type definition (currently around line 15):

```go
type Session struct {
	Config  aws.Config
	Profile string
	Region  string
	// Dragos is populated only when Profile == DragosProfile.
	Dragos *DragosSession
}
```

- [ ] **Step 2: Add the Environment field**

Replace the struct with:

```go
type Session struct {
	Config  aws.Config
	Profile string
	Region  string
	// Dragos is populated only when Profile == DragosProfile.
	// Phase 2: kept for backwards compatibility with view.go's
	// session.Dragos != nil checks. Phase 4 deletes this field once
	// callers consult Environment.Transport.Type instead.
	Dragos *DragosSession
	// Environment is the resolved description of the active backend.
	// Populated alongside the legacy fields by SwitchEnvironment, which
	// SwitchProfile now delegates to. Phase 2 readers may ignore this
	// field; phase 4 makes it the source of truth.
	Environment environments.Environment
}
```

- [ ] **Step 3: Add the import**

Add to `internal/auth/auth.go`'s import block (alphabetized):

```go
	"github.com/tpe11etier/cloudcutter/internal/environments"
```

- [ ] **Step 4: Build to confirm no callers broke**

Run: `go build -mod=vendor ./...`

Expected: succeeds. The new field is additive; all existing call sites that use `Session.Config`, `Session.Profile`, etc. continue to compile.

- [ ] **Step 5: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/auth.go
git commit -m "Add Session.Environment alongside legacy fields"
```

---

### Task 5: Legacy → Environment translator

**Files:**
- Create: `internal/auth/translator.go`
- Test: `internal/auth/translator_test.go`

- [ ] **Step 1: Write the failing test `internal/auth/translator_test.go`**

```go
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
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `go test -mod=vendor ./internal/auth -run TestTranslate`

Expected: FAIL — `undefined: legacyToEnvironment`.

- [ ] **Step 3: Implement the translator**

Write `internal/auth/translator.go`:

```go
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
```

- [ ] **Step 4: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/auth -run TestTranslate -v`

Expected: PASS for `TestTranslateLocal`, `TestTranslateOpal`, `TestTranslateOpalProd`, `TestTranslateDragosWithConfig`, `TestTranslateStandard`, and `TestTranslateUnusedConfigImport`. `TestTranslateDragosNoConfig` may PASS, FAIL, or SKIP depending on implementation choice — the test allows either behavior.

- [ ] **Step 5: Run all auth tests**

Run: `go test -mod=vendor ./internal/auth -v`

Expected: existing auth tests still PASS plus the new translator tests.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/translator.go internal/auth/translator_test.go
git commit -m "Add legacy-profile to Environment translator"
```

---

### Task 6: `Authenticator.SwitchEnvironment`

**Files:**
- Modify: `internal/auth/auth.go` (add `SwitchEnvironment`)
- Test: `internal/auth/auth_test.go` (or `internal/auth/switch_environment_test.go` — choose based on existing layout; if `auth_test.go` is small, add tests there; otherwise create the new file)

- [ ] **Step 1: Inspect existing test layout**

Run: `ls internal/auth/*_test.go`

Note which files already exist. If a focused test file would be cleanest, create `internal/auth/switch_environment_test.go`. Otherwise add tests to the existing `auth_test.go`.

- [ ] **Step 2: Write the failing test**

Create `internal/auth/switch_environment_test.go`:

```go
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
```

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test -mod=vendor ./internal/auth -run TestSwitchEnvironment`

Expected: FAIL — `undefined: SwitchEnvironment`.

- [ ] **Step 4: Implement `SwitchEnvironment`**

In `internal/auth/auth.go`, add the new method. Place it near `SwitchProfile`:

```go
// SwitchEnvironment authenticates using the given Environment and returns
// the resulting Session. Unlike SwitchProfile (which keys off a profile
// name), SwitchEnvironment dispatches on env.Auth.Type.
//
// SwitchEnvironment is the new dispatch surface introduced in phase 2.
// Phase 4 is when the manager calls it directly; until then, SwitchProfile
// translates legacy profile names into Environment values and delegates.
func (a *Authenticator) SwitchEnvironment(ctx context.Context, env environments.Environment) (*Session, error) {
	a.mu.Lock()
	if a.isAuthenticating {
		a.mu.Unlock()
		return nil, fmt.Errorf("authentication already in progress")
	}
	a.isAuthenticating = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.isAuthenticating = false
		a.mu.Unlock()
	}()

	a.sendStatus(fmt.Sprintf("Switching to %s in %s", env.Name, env.Region))

	session := &Session{
		Profile:     env.Name,
		Region:      env.Region,
		Environment: env,
	}

	switch env.Auth.Type {
	case "none":
		// No-op. Session.Config stays zero-value.

	case "aws_sdk":
		if env.Auth.PreAuth != nil {
			if err := a.runPreAuthCommand(ctx, env.Auth.PreAuth, env.Name); err != nil {
				return nil, fmt.Errorf("pre-auth: %w", err)
			}
		}
		cfg, err := a.authenticateStandard(ctx, env.Name, env.Region)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
		session.Config = cfg

	case "jwt":
		token, err := loadJWT(env.Auth)
		if err != nil {
			return nil, err
		}
		session.Token = token
		// Run the probe if the transport defines one — Dragos always does.
		if env.Transport.Probe != nil {
			if err := runJWTProbe(ctx, env, token); err != nil {
				return nil, err
			}
		}
		// Phase 2 backwards-compat: still populate the legacy DragosSession
		// so view.go's session.Dragos != nil checks work. Phase 4 deletes
		// this branch.
		if env.Name == DragosProfile {
			session.Dragos = &DragosSession{
				BaseURL:      env.Transport.BaseURL,
				AuthToken:    token,
				IndexPattern: env.IndexPattern,
				KbnVersion:   env.Transport.Headers["kbn-version"],
			}
			session.Config = aws.Config{Region: env.Region}
		}

	default:
		return nil, fmt.Errorf("unknown auth.type %q (want none|aws_sdk|jwt)", env.Auth.Type)
	}

	a.mu.Lock()
	a.currentSession = session
	a.mu.Unlock()

	return session, nil
}

// runPreAuthCommand executes Auth.PreAuth.Command, substituting any
// {profile} placeholder with the active profile name. {role_id} is
// already substituted by the translator (or the materializer in
// phase 4); we only handle {profile} here as a runtime-only convenience.
func (a *Authenticator) runPreAuthCommand(ctx context.Context, spec *config.PreAuthSpec, profile string) error {
	if len(spec.Command) == 0 {
		return nil
	}
	cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	a.sendStatus(fmt.Sprintf("Running pre-auth: %s", strings.Join(spec.Command, " ")))
	if err := cmd.Run(); err != nil {
		output := stdout.String() + stderr.String()
		for _, marker := range spec.DetectSessionExpired {
			if strings.Contains(output, marker) {
				return fmt.Errorf("session expired: please re-authenticate the underlying tool (%s)", spec.Command[0])
			}
		}
		return fmt.Errorf("%s failed: %v\nOutput: %s", spec.Command[0], err, output)
	}
	a.sendStatus("Pre-auth completed")
	return nil
}

// loadJWT resolves the token from env > path. Returns an error when none
// is available; callers may then trigger a login modal flow (the manager
// is responsible for that — auth itself doesn't pop UIs).
func loadJWT(spec config.AuthSpec) (string, error) {
	if spec.Env != "" {
		if v := os.Getenv(spec.Env); v != "" {
			return v, nil
		}
	}
	if spec.Path != "" {
		raw, err := os.ReadFile(spec.Path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("jwt token not available: file %s does not exist (set $%s or run the login flow)", spec.Path, spec.Env)
			}
			return "", fmt.Errorf("read jwt %s: %w", spec.Path, err)
		}
		t := strings.TrimSpace(string(raw))
		if t == "" {
			return "", fmt.Errorf("jwt token at %s is empty", spec.Path)
		}
		return t, nil
	}
	return "", fmt.Errorf("jwt token unavailable: no env or path configured")
}

// runJWTProbe builds a probe request from the Environment and runs it.
func runJWTProbe(ctx context.Context, env environments.Environment, token string) error {
	probeURL := strings.TrimRight(env.Transport.BaseURL, "/") +
		env.Transport.ProxyPath +
		"?path=" + env.Transport.Probe.Path + "&method=GET"

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, probeURL, nil)
	if err != nil {
		return fmt.Errorf("probe: build request: %w", err)
	}
	if env.Transport.TokenHeader != nil {
		header := strings.ReplaceAll(env.Transport.TokenHeader.Format, "{token}", token)
		req.Header.Set(env.Transport.TokenHeader.Name, header)
	}
	for k, v := range env.Transport.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if err := probe.Run(probeCtx, &http.Client{Timeout: 15 * time.Second}, req, env.Transport.Probe.RejectHTML); err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 5: Add the new imports**

`internal/auth/auth.go` imports already include `bytes`, `context`, `os/exec`, `strings`, `sync`. Add what's missing:

```go
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/tpe11etier/cloudcutter/internal/config"
	"github.com/tpe11etier/cloudcutter/internal/probe"
```

`environments` is already imported (Task 4). Verify — if missing, add `"github.com/tpe11etier/cloudcutter/internal/environments"`.

- [ ] **Step 6: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/auth -v`

Expected: PASS for all `TestSwitchEnvironment*` tests plus all existing auth tests.

- [ ] **Step 7: Commit**

```bash
git add internal/auth/auth.go internal/auth/switch_environment_test.go
git commit -m "Add Authenticator.SwitchEnvironment dispatching on Auth.Type"
```

---

### Task 7: Refactor `SwitchProfile` to delegate

**Files:**
- Modify: `internal/auth/auth.go` (only `SwitchProfile`'s body)

- [ ] **Step 1: Read the current SwitchProfile**

In `internal/auth/auth.go`, find `func (a *Authenticator) SwitchProfile`. The current implementation is roughly:

```go
func (a *Authenticator) SwitchProfile(ctx context.Context, profile, region string) (*Session, error) {
	a.mu.Lock()
	if a.isAuthenticating {
		a.mu.Unlock()
		return nil, fmt.Errorf("authentication already in progress")
	}
	a.isAuthenticating = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.isAuthenticating = false
		a.mu.Unlock()
	}()

	if a.currentSession != nil &&
		a.currentSession.Profile == profile &&
		a.currentSession.Region == region {
		return a.currentSession, nil
	}

	a.sendStatus(fmt.Sprintf("Switching to profile %s in %s", profile, region))

	session := &Session{
		Profile: profile,
		Region:  region,
	}

	switch {
	case profile == DragosProfile:
		// ... etc
	}
	// ... etc

	a.mu.Lock()
	a.currentSession = session
	a.mu.Unlock()

	return session, nil
}
```

- [ ] **Step 2: Replace SwitchProfile with the delegating wrapper**

Replace the function body:

```go
// SwitchProfile is the legacy entry point: callers pass a profile name and
// region, and SwitchProfile builds the corresponding Environment from
// legacy config sources (~/.cloudcutter/dragos.json, the opal profile map,
// env vars) and delegates to SwitchEnvironment.
//
// Phase 4 deletes this method when callers switch to SwitchEnvironment
// directly.
func (a *Authenticator) SwitchProfile(ctx context.Context, profile, region string) (*Session, error) {
	a.mu.RLock()
	cached := a.currentSession
	a.mu.RUnlock()
	if cached != nil && cached.Profile == profile && cached.Region == region {
		return cached, nil
	}

	env, err := a.legacyToEnvironment(profile, region)
	if err != nil {
		return nil, err
	}
	return a.SwitchEnvironment(ctx, env)
}
```

The cache-hit short-circuit stays (it predates phase 2 and is still useful) but is now read-locked. The lock+isAuthenticating logic moves into `SwitchEnvironment` (already present from Task 6). Note: the original code over-locked — it took the write lock and set `isAuthenticating=true` even for cache hits. The new shape lets concurrent cache-hit callers proceed without contention, which is strictly an improvement. Behavior on cache miss is preserved exactly.

- [ ] **Step 3: Delete the old `authenticate*` helpers that have been superseded**

The four `authenticate*` helpers (`authenticateStandard`, `authenticateOpal`, `authenticateDragos`, `authenticateLocal`) used to be the per-branch implementations. After Task 6 + Task 7:

- `authenticateStandard` is still called from `SwitchEnvironment`'s `aws_sdk` branch (via `a.authenticateStandard(ctx, env.Name, env.Region)`). **Keep it.**
- `authenticateOpal` was wrapping `runOpalCommand` + `authenticateStandard`. The pre-auth logic is now in `runPreAuthCommand` (generic). **Delete `authenticateOpal`.**
- `authenticateDragos` had the probe + DragosSession-building. Both are now in `SwitchEnvironment`'s `jwt` branch. **Delete `authenticateDragos`.**
- `authenticateLocal` was wrapping `LoadDefaultConfig` with region "local". The local environment's Auth.Type is "none" so there's no AWS load. **Delete `authenticateLocal`.**
- `runOpalCommand` is replaced by `runPreAuthCommand`. **Delete `runOpalCommand`.**

After deleting those four-plus helpers, the only branching that remains in auth.go is in `SwitchEnvironment`. `SwitchProfile` is the thin delegator.

- [ ] **Step 4: Build and run all tests**

Run: `go build -mod=vendor ./...`

Expected: succeeds.

Run: `go test -mod=vendor ./...`

Expected: PASS for all packages, including `internal/auth` (existing tests) and the new SwitchEnvironment / translator tests.

- [ ] **Step 5: Smoke-test the running app**

Run:

```bash
go run -mod=vendor ./cmd/cloudcutter
```

Expected: cloudcutter starts. The picker shows the same profiles as before. Selecting `dragos` triggers the same login modal flow as today (probe runs, login modal pops if token is bad). Selecting `local` works. Selecting an opal profile runs `opal iam-roles:start` as today.

If you don't want to do interactive smoke testing, skip this step and rely on the test suite. Note in your report that interactive smoke was skipped.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/auth.go
git commit -m "Refactor SwitchProfile to delegate to SwitchEnvironment via translator"
```

---

### Task 8: Phase 2 verification

**Files:** none modified.

- [ ] **Step 1: Full test sweep**

Run: `go test -mod=vendor ./...`

Expected: every package PASSes.

- [ ] **Step 2: `go vet`**

Run: `go vet -mod=vendor ./...`

Expected: no output.

- [ ] **Step 3: gofmt sanity**

Run: `gofmt -l internal/auth internal/probe internal/services/elastic`

Expected: no output. If output, run `gofmt -w` on the listed files and amend the most recent commit (not the whole phase — just the gofmt fix).

- [ ] **Step 4: Confirm `auth.ProbeDragos` is gone**

Run: `grep -rn 'ProbeDragos\b' --include='*.go' .`

Expected: no output.

- [ ] **Step 5: Confirm the manager and views are unchanged**

Run: `git diff phase-1-complete..HEAD -- 'internal/ui/' 'cmd/cloudcutter/' 'internal/services/services.go'`

Expected: no output. Phase 2 is supposed to leave the UI layer untouched. If there's a diff, investigate — was it intentional?

- [ ] **Step 6: Confirm no callers in the running app reference SwitchEnvironment yet**

Run: `grep -rn 'SwitchEnvironment' --include='*.go' . | grep -v internal/auth/`

Expected: no output. Only `internal/auth` should reference SwitchEnvironment in phase 2 (its own tests and the SwitchProfile body).

- [ ] **Step 7: Tag the phase**

```bash
git tag phase-2-complete
```

- [ ] **Step 8: Update the spec status**

Edit `docs/superpowers/specs/2026-05-09-generic-backend-design.md` and change the `**Status**:` line:

From:

```
**Status**: Phase 1 implemented (commit `phase-1-complete`). Phase 2 (auth refactor) plan to be drafted next.
```

To:

```
**Status**: Phase 2 implemented (tag `phase-2-complete`). Phase 3 (transport refactor) plan to be drafted next.
```

```bash
git add docs/superpowers/specs/2026-05-09-generic-backend-design.md
git commit -m "Note phase 2 complete in spec status"
```

---

## What's NOT in phase 2 (deferred)

- **Manager-resolver wiring** — the manager's `switchTo*Profile` funcs continue to call `Authenticator.SwitchProfile(ctx, name, region)` exactly as today. Phase 4 reworks them.
- **Cloudcutter-startup YAML loading** — `cmd/cloudcutter/main.go` still doesn't read `config.yaml` at runtime. Phase 4.
- **Picker rework** — phase 4.
- **Transport refactor** — `dragosTransport` and `awsTransport` are unchanged in phase 2. Phase 3 makes them parameterized by Environment.
- **`Session.Dragos` removal** — kept in phase 2 for backwards compat with `view.go`'s `session.Dragos != nil` checks. Phase 4 deletes it.
- **Legacy config deletion** — `LoadDragosConfig`, `LoadOpalConfig` stay in place; the translator reads them. Phase 5.

# Generic Backend — Phase 5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete every piece of vendor-named legacy code now that phase 4 has cut the running app over to the YAML resolver. The only substantive change is generalizing the dragos-named login modal (`ShowDragosLoginModal` + `auth.LoginWithPassword` + `auth.SaveDragosToken`) into `ShowJWTLoginModal` parameterized by `env.Auth.Login.BodyFields`. Everything else is mechanical deletion.

**Architecture:** Phase 5 deletes in dependency order — caller before callee — so the build stays green between commits. T1 generalizes the login modal because it's the last caller of `auth.LoadDragosConfig` / `LoginWithPassword` / `SaveDragosToken`. T2-T8 then delete: the auth translator, the legacy elastic constructors, the legacy services initializers, the legacy profile picker code, `Session.Dragos`, the dragos+opal config helpers, and any orphaned manager methods. Final tag = `phase-5-complete` = the running app has zero references to the strings "dragos", "darkbytes", "Sophos", "opal" outside user-facing config files / token paths / diagnostic CLIs.

**Tech Stack:** Go. No new dependencies; phase 5 reduces the dependency surface (`gopkg.in/ini.v1` and `github.com/browserutils/kooky` may become removable from `internal/auth`'s consumers).

**Spec:** `docs/superpowers/specs/2026-05-09-generic-backend-design.md` — particularly the "Deletions" list under "Go types & key interfaces" and the "Phase 5" row of the Phasing table.

**Status as of plan-write time:** phases 1-4 complete (tags `phase-1-complete` through `phase-4-complete`). Phase 5 is the final phase.

---

## Out of scope (true follow-ups, not phase 5)

- **Renaming user-facing artifacts**: `~/.cloudcutter/dragos.token` filename, `cmd/dragos-cookie-probe/` diagnostic CLI, `dragos.json` migration logic in `cloudcutter init`. These are user-config / one-shot tooling — phase 5 leaves them alone.
- **The `gopkg.in/ini.v1` vendor entry**: still used by `cmd/dragos-cookie-probe` (and possibly other transitive deps). Phase 5 removes the auth-package usage; leave the vendor module in place.
- **The `github.com/browserutils/kooky` vendor entry**: same — still used by `cmd/dragos-cookie-probe`. Phase 5 removes the import from `internal/auth/dragos_config.go`; the binary keeps the dep.
- **Login modal styling / UX polish**: the generalization in T1 preserves the existing form-rendering style. Re-skinning is a separate UX project.

---

## File structure

**New files**

| File | Responsibility |
|---|---|
| `internal/auth/login.go` | `LoginJWT(ctx, spec, formValues) (string, error)` — generalized version of `LoginWithPassword`. Takes `config.LoginSpec` + a map of submitted form values; builds the request from the spec; extracts the token per spec.TokenExtract. Replaces `dragos_login.go`'s `LoginWithPassword`. |
| `internal/auth/login_test.go` | httptest-driven tests for `LoginJWT`. Covers JSON body, form body, cookie extraction, header extraction, json_path-not-implemented error, 401-credentials, transport error. |
| `internal/auth/path.go` | Promotes `expandHome(path)` from `auth.go` into an exported helper, plus a small write-token-to-path helper used by the modal. Two public helpers, ten lines each. |

**Files DELETED (entirely)**

| File | Reason |
|---|---|
| `internal/auth/translator.go` | All 4 env-builder helpers + `legacyToEnvironment` + `writeDragosTokenFile`. |
| `internal/auth/translator_test.go` | Tests for above. |
| `internal/auth/dragos_login.go` | `LoginWithPassword` + `ProbeDragos` (already deleted in P2; the file may be empty). The new `LoginJWT` lives in `login.go`. |
| `internal/auth/dragos_config.go` | `LoadDragosConfig`, `SaveDragosToken`, `DragosConfigPath`, `DefaultDragos*` constants, `loadDragosCookieFromBrowser`. |
| `internal/auth/config.go` | `OpalConfig`, `LoadOpalConfig`, `DefaultOpalConfig`, `getEnvOrDefault`, `isValidConfig`. |

**Modified files**

| File | Change |
|---|---|
| `internal/auth/auth.go` | Delete `Authenticator.SwitchProfile`. Delete `Authenticator.opalConfig` + `opalProfiles` fields. Update `New(statusFn)` to not call `LoadOpalConfig`. Delete `Session.Dragos` field. Delete the `if env.Name == DragosProfile` back-compat branch in `SwitchEnvironment.jwt`. Delete `auth.DragosProfile` constant. Delete `auth.DragosSession` type. Move `expandHome` out into `path.go` (or just leave it — see T6). |
| `internal/services/elastic/elastic.go` | Delete `NewService(cfg)`, `Reinitialize(cfg, profile)`, `legacySophosEnvFromAWSConfig`. The `Service` struct + utility methods (`PreloadIndexStats`, `ListIndices`, `Close`, etc.) stay. |
| `internal/services/elastic/dragos.go` | Delete the file entirely if it's only legacy wrappers + `legacyDragosEnvFromSession`. Otherwise delete those specific symbols. |
| `internal/services/services.go` | Delete `InitializeElastic(cfg)` + `InitializeElasticDragos(d)`. The `Services` struct field + `New` constructor stay (used by main.go's lazy view registration to hold `services.Elastic`). |
| `internal/ui/components/profile/handler.go` | Delete `Handler.SwitchProfile`. The `Handler.SwitchEnvironment` (added in P4-T1) is the only switch path now. |
| `internal/ui/components/profile/profile.go` | Delete `Selector.discoverProfiles` + `Selector.switchProfile`. Remove `auth` and `gopkg.in/ini.v1` imports. The Resolver-driven path stays (added in P4-T4). |
| `internal/ui/manager/manager.go` | Rename `ShowDragosLoginModal` → `ShowJWTLoginModal`. Parameterize from `env.Auth.Login` instead of calling `LoadDragosConfig`. Form fields rendered from `env.Auth.Login.BodyFields`. Token persisted via `os.WriteFile(env.Auth.Path, ...)`. Update the `switchToEnvironment` callsite to pass `env`. Delete `Manager.CurrentProfile()` if no callers remain. |

---

## Tasks

### Task 1: Generalize the JWT login modal

**Files:**
- Create: `internal/auth/login.go` (new `LoginJWT` function)
- Create: `internal/auth/login_test.go` (TDD)
- Create: `internal/auth/path.go` (`ExpandHome` exported)
- Modify: `internal/ui/manager/manager.go` (`ShowDragosLoginModal` → `ShowJWTLoginModal`, parameterized)

This task is the only substantive (non-deletion) work in phase 5. It detangles the modal from its dragos-specific dependencies so the subsequent deletion tasks (T7-T8) can drop `LoadDragosConfig`, `LoginWithPassword`, `SaveDragosToken`, `DragosConfigPath`, `DefaultDragos*` constants safely.

- [ ] **Step 1: Write failing tests for `LoginJWT`**

Write `internal/auth/login_test.go`:

```go
package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tpe11etier/cloudcutter/internal/config"
)

func TestLoginJWTPostsJSONAndExtractsCookie(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotProviderID string
	var gotBody map[string]string
	var gotContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotProviderID = r.URL.Query().Get("providerId")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		http.SetCookie(w, &http.Cookie{Name: "auth-tok", Value: "the-jwt"})
		w.WriteHeader(200)
	}))
	defer srv.Close()

	spec := config.LoginSpec{
		URL:        srv.URL + "/auth/api/v1/login/password",
		BodyFormat: "json",
		BodyFields: []config.FormField{
			{Name: "username", Kind: "text"},
			{Name: "password", Kind: "password"},
		},
		Query: map[string]string{"providerId": "abc-123"},
		TokenExtract: config.TokenExtractSpec{
			From: "cookie",
			Name: "auth-tok",
		},
	}
	values := map[string]string{
		"username": "alice",
		"password": "s3cret",
	}

	tok, err := LoginJWT(context.Background(), spec, values)
	if err != nil {
		t.Fatalf("LoginJWT: %v", err)
	}
	if tok != "the-jwt" {
		t.Errorf("token = %q, want the-jwt", tok)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/auth/api/v1/login/password") {
		t.Errorf("path = %q", gotPath)
	}
	if gotProviderID != "abc-123" {
		t.Errorf("providerId = %q", gotProviderID)
	}
	if !strings.Contains(gotContentType, "application/json") {
		t.Errorf("content-type = %q", gotContentType)
	}
	if gotBody["username"] != "alice" || gotBody["password"] != "s3cret" {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestLoginJWTFormBodyFormat(t *testing.T) {
	var gotBody string
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("X-Auth", "headertok")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	spec := config.LoginSpec{
		URL:        srv.URL + "/login",
		BodyFormat: "form",
		BodyFields: []config.FormField{
			{Name: "u", Kind: "text"},
			{Name: "p", Kind: "password"},
		},
		TokenExtract: config.TokenExtractSpec{From: "header", Name: "X-Auth"},
	}
	values := map[string]string{"u": "bob", "p": "pw"}

	tok, err := LoginJWT(context.Background(), spec, values)
	if err != nil {
		t.Fatalf("LoginJWT: %v", err)
	}
	if tok != "headertok" {
		t.Errorf("token = %q, want headertok", tok)
	}
	if !strings.Contains(gotContentType, "x-www-form-urlencoded") {
		t.Errorf("content-type = %q", gotContentType)
	}
	parsed, _ := url.ParseQuery(gotBody)
	if parsed.Get("u") != "bob" || parsed.Get("p") != "pw" {
		t.Errorf("body = %q", gotBody)
	}
}

func TestLoginJWTRejects401As4InvalidCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	spec := config.LoginSpec{
		URL:        srv.URL + "/login",
		BodyFormat: "json",
		BodyFields: []config.FormField{{Name: "u", Kind: "text"}},
		TokenExtract: config.TokenExtractSpec{From: "cookie", Name: "x"},
	}
	_, err := LoginJWT(context.Background(), spec, map[string]string{"u": "x"})
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error should mention invalid credentials, got %q", err.Error())
	}
}

func TestLoginJWTRejectsJSONPathNotImplemented(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	spec := config.LoginSpec{
		URL:          srv.URL + "/login",
		BodyFormat:   "json",
		BodyFields:   []config.FormField{{Name: "u", Kind: "text"}},
		TokenExtract: config.TokenExtractSpec{From: "json_path", Name: "$.token"},
	}
	_, err := LoginJWT(context.Background(), spec, map[string]string{"u": "x"})
	if err == nil {
		t.Fatal("expected error for json_path")
	}
	if !strings.Contains(err.Error(), "json_path") {
		t.Errorf("error should mention json_path, got %q", err.Error())
	}
}

func TestLoginJWTMissingCookieErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200) // 200 but no cookie
	}))
	defer srv.Close()

	spec := config.LoginSpec{
		URL:          srv.URL + "/login",
		BodyFormat:   "json",
		BodyFields:   []config.FormField{{Name: "u", Kind: "text"}},
		TokenExtract: config.TokenExtractSpec{From: "cookie", Name: "auth-tok"},
	}
	_, err := LoginJWT(context.Background(), spec, map[string]string{"u": "x"})
	if err == nil {
		t.Fatal("expected error when cookie is absent")
	}
	if !strings.Contains(err.Error(), "auth-tok") {
		t.Errorf("error should name expected cookie, got %q", err.Error())
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `go test -mod=vendor ./internal/auth -run TestLoginJWT`

Expected: FAIL — `undefined: LoginJWT`.

- [ ] **Step 3: Implement `LoginJWT`**

Write `internal/auth/login.go`:

```go
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tpe11etier/cloudcutter/internal/config"
)

// LoginJWT exchanges form values for a JWT according to the LoginSpec.
// It POSTs to spec.URL with body in spec.BodyFormat (json | form),
// applies any spec.Query params, and extracts the token per
// spec.TokenExtract.
//
// Replaces the dragos-specific LoginWithPassword. The new function is
// vendor-neutral: a future env can declare its own URL, body format,
// query, and token extraction strategy without touching Go.
func LoginJWT(ctx context.Context, spec config.LoginSpec, formValues map[string]string) (string, error) {
	if spec.URL == "" {
		return "", fmt.Errorf("login: spec.URL is empty")
	}

	loginURL, err := url.Parse(spec.URL)
	if err != nil {
		return "", fmt.Errorf("login: invalid URL %q: %w", spec.URL, err)
	}
	if len(spec.Query) > 0 {
		q := loginURL.Query()
		for k, v := range spec.Query {
			q.Set(k, v)
		}
		loginURL.RawQuery = q.Encode()
	}

	body, contentType, err := encodeLoginBody(spec.BodyFormat, formValues)
	if err != nil {
		return "", err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, loginURL.String(), body)
	if err != nil {
		return "", fmt.Errorf("login: build request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cloudcutter")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("invalid credentials")
	}
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("login failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	return extractToken(resp, spec.TokenExtract)
}

func encodeLoginBody(format string, values map[string]string) (io.Reader, string, error) {
	switch format {
	case "", "json":
		raw, err := json.Marshal(values)
		if err != nil {
			return nil, "", fmt.Errorf("marshal credentials: %w", err)
		}
		return bytes.NewReader(raw), "application/json", nil
	case "form":
		formData := url.Values{}
		for k, v := range values {
			formData.Set(k, v)
		}
		return strings.NewReader(formData.Encode()), "application/x-www-form-urlencoded", nil
	default:
		return nil, "", fmt.Errorf("login: unknown body_format %q (want json|form)", format)
	}
}

func extractToken(resp *http.Response, spec config.TokenExtractSpec) (string, error) {
	switch spec.From {
	case "cookie":
		for _, c := range resp.Cookies() {
			if c.Name == spec.Name && c.Value != "" {
				return c.Value, nil
			}
		}
		return "", fmt.Errorf("login OK but no %q cookie in response", spec.Name)
	case "header":
		if v := resp.Header.Get(spec.Name); v != "" {
			return v, nil
		}
		return "", fmt.Errorf("login OK but no %q header in response", spec.Name)
	case "json_path":
		return "", fmt.Errorf("token_extract from=json_path not yet implemented")
	default:
		return "", fmt.Errorf("token_extract: unknown from %q", spec.From)
	}
}
```

- [ ] **Step 4: Promote `expandHome` out of auth.go**

Run: `grep -n 'func expandHome' internal/auth/auth.go`

Move the existing `expandHome` function from `internal/auth/auth.go` to a new file `internal/auth/path.go`, exporting it as `ExpandHome`:

Write `internal/auth/path.go`:

```go
package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandHome resolves a leading "~" to the user's home directory.
// Anything else passes through unchanged.
//
// The YAML config schema documents tilde-prefixed paths
// (e.g. auth.path: ~/.cloudcutter/dragos.token) and users naturally
// write them, but Go's os.ReadFile/os.WriteFile do no shell-style
// expansion. ExpandHome bridges that gap.
func ExpandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand %q: %w", path, err)
		}
		if path == "~" {
			return home, nil
		}
		return home + path[1:], nil
	}
	return path, nil
}

// WriteTokenFile writes a token to path with mode 0600, creating the
// parent directory with mode 0700 if needed. ~/path is expanded.
//
// Used by the JWT login modal to persist the token at the env's
// auth.path location.
func WriteTokenFile(path, token string) error {
	expanded, err := ExpandHome(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(expanded), 0o700); err != nil {
		return fmt.Errorf("create parent dir of %s: %w", expanded, err)
	}
	if err := os.WriteFile(expanded, []byte(strings.TrimSpace(token)), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", expanded, err)
	}
	return nil
}
```

In `internal/auth/auth.go`, find the existing `expandHome` definition (lowercase) and DELETE it. Update its caller in `loadJWT` to call `ExpandHome` instead:

Find:

```go
		path, err := expandHome(spec.Path)
```

Replace with:

```go
		path, err := ExpandHome(spec.Path)
```

If the build now reports unused imports (`path/filepath` was used by `expandHome` and is also used elsewhere — check), leave them; the compiler will tell you.

- [ ] **Step 5: Verify the new code compiles + LoginJWT tests pass**

Run: `go build -mod=vendor ./...`

Expected: succeeds. The old `LoginWithPassword` still exists; both coexist temporarily.

Run: `go test -mod=vendor ./internal/auth -run TestLoginJWT -v`

Expected: PASS for all 5 sub-tests.

- [ ] **Step 6: Update `Manager.ShowDragosLoginModal` → `ShowJWTLoginModal` parameterized from env**

In `internal/ui/manager/manager.go`, find `ShowDragosLoginModal` (a long function — ~120 lines, near the bottom of the file).

Replace the entire function with the version below. Note the new signature takes `env environments.Environment`:

```go
// ShowJWTLoginModal opens a form derived from env.Auth.Login.BodyFields,
// POSTs the submitted values via auth.LoginJWT, persists the resulting
// token to env.Auth.Path, and calls onSuccess. Esc / Cancel calls
// onCancel.
//
// Replaces the dragos-named ShowDragosLoginModal. All vendor-specific
// knobs (URL, providerId, body fields, token extraction) come from
// env.Auth.Login.
func (vm *Manager) ShowJWTLoginModal(env environments.Environment, onSuccess func(), onCancel func()) {
	const pageName = "jwtLogin"

	if env.Auth.Login == nil {
		vm.statusBar.SetText(fmt.Sprintf("Environment %q has no login spec", env.Name))
		if onCancel != nil {
			onCancel()
		}
		return
	}
	loginSpec := *env.Auth.Login

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" %s Login — %s (Tab to move, Enter to submit, Esc to cancel) ", env.Name, loginSpec.URL))
	form.SetTitleAlign(tview.AlignLeft)
	form.SetTitleColor(style.GruvboxMaterial.Yellow)
	form.SetBorderColor(tcell.ColorMediumTurquoise)
	form.SetFieldBackgroundColor(tcell.ColorBlack)
	form.SetFieldTextColor(tcell.ColorBeige)
	form.SetButtonBackgroundColor(tcell.ColorDarkCyan)
	form.SetButtonTextColor(tcell.ColorBeige)

	for _, f := range loginSpec.BodyFields {
		label := strings.Title(f.Name)
		if f.Kind == "password" {
			form.AddPasswordField(label, "", 40, '*', nil)
		} else {
			form.AddInputField(label, "", 40, nil, nil)
		}
	}

	closeModal := func() { vm.pages.RemovePage(pageName) }

	submit := func() {
		values := make(map[string]string, len(loginSpec.BodyFields))
		missing := []string{}
		for _, f := range loginSpec.BodyFields {
			label := strings.Title(f.Name)
			input, ok := form.GetFormItemByLabel(label).(*tview.InputField)
			if !ok {
				continue
			}
			val := strings.TrimSpace(input.GetText())
			if f.Kind == "password" {
				val = input.GetText() // don't trim password
			}
			if val == "" {
				missing = append(missing, label)
			}
			values[f.Name] = val
		}
		if len(missing) > 0 {
			vm.statusBar.SetText(fmt.Sprintf("Required: %s", strings.Join(missing, ", ")))
			return
		}

		vm.statusBar.SetText(fmt.Sprintf("Authenticating with %s...", env.Name))
		go func() {
			token, err := auth.LoginJWT(vm.ctx, loginSpec, values)
			vm.app.QueueUpdateDraw(func() {
				if err != nil {
					vm.statusBar.SetText(fmt.Sprintf("Login failed: %v", err))
					return
				}
				if err := auth.WriteTokenFile(env.Auth.Path, token); err != nil {
					vm.statusBar.SetText(fmt.Sprintf("Login OK but couldn't save token: %v", err))
					return
				}
				closeModal()
				vm.statusBar.SetText(fmt.Sprintf("%s login successful", env.Name))
				if onSuccess != nil {
					onSuccess()
				}
			})
		}()
	}

	form.AddButton("Login", submit)
	form.AddButton("Cancel", func() {
		closeModal()
		if onCancel != nil {
			onCancel()
		}
	})
	form.SetCancelFunc(func() {
		closeModal()
		if onCancel != nil {
			onCancel()
		}
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() != tcell.KeyEnter {
			return event
		}
		_, btnIdx := form.GetFocusedItemIndex()
		if btnIdx >= 0 {
			return event
		}
		submit()
		return nil
	})

	height := 5 + 2*len(loginSpec.BodyFields)
	width := 60
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(form, width, 0, true).
			AddItem(nil, 0, 1, false),
			height, 0, true).
		AddItem(nil, 0, 1, false)

	vm.pages.AddPage(pageName, layout, true, true)
	vm.app.SetFocus(form)
}
```

The `strings.Title` is deprecated in modern Go but still works. If your Go version is 1.18+ and rejects it, replace with a small inline helper:

```go
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
```

…and use `titleCase(f.Name)` in place of `strings.Title(f.Name)`. Add the helper at the bottom of `manager.go`.

DELETE the old `ShowDragosLoginModal` function entirely.

- [ ] **Step 7: Update the two callsites in `switchToEnvironment`**

In `manager.go`'s `switchToEnvironment`, find the two `vm.ShowDragosLoginModal(...)` invocations. Replace each with `vm.ShowJWTLoginModal(env, ...)`:

```go
				vm.ShowJWTLoginModal(env,
					func() { vm.switchToEnvironment(name) },
					func() { vm.StatusChan <- "Auth canceled" },
				)
```

(Two occurrences — one in the err branch, one in the reinit-fail branch.)

- [ ] **Step 8: Build + run all tests**

Run: `go build -mod=vendor ./...`

Expected: succeeds. The old `LoginWithPassword` and `SaveDragosToken` may still exist but no longer have callers from `manager.go`; T7 deletes them.

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/auth/login.go internal/auth/login_test.go internal/auth/path.go internal/auth/auth.go internal/ui/manager/manager.go
git commit -m "Generalize JWT login modal: ShowJWTLoginModal + LoginJWT"
```

---

### Task 2: Delete the auth translator + `Authenticator.SwitchProfile`

**Files:**
- Delete: `internal/auth/translator.go`
- Delete: `internal/auth/translator_test.go`
- Modify: `internal/auth/auth.go` (delete `SwitchProfile` method)

- [ ] **Step 1: Confirm no callers remain**

Run: `grep -rn 'legacyToEnvironment\|SwitchProfile\b' --include='*.go' . | grep -v internal/auth/`

Expected: only the dead `Handler.SwitchProfile` in `internal/ui/components/profile/handler.go` (deleted in T5). If any other caller exists, escalate.

Run: `grep -rn 'legacyToEnvironment\|writeDragosTokenFile\|dragosEnvironment\|opalEnvironment\|localEnvironment\|standardAWSEnvironment' --include='*.go' .`

Expected: only references inside `internal/auth/translator.go` and `translator_test.go`.

- [ ] **Step 2: Delete the translator files**

Run:

```bash
rm internal/auth/translator.go internal/auth/translator_test.go
```

- [ ] **Step 3: Delete `Authenticator.SwitchProfile`**

In `internal/auth/auth.go`, find `func (a *Authenticator) SwitchProfile` and delete the entire function. It's the wrapper that called `a.legacyToEnvironment` (now gone).

- [ ] **Step 4: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds. `Handler.SwitchProfile` (in profile package) still references this method via `ph.auth.SwitchProfile` — fix it now by also deleting Handler.SwitchProfile.

If the build error names `internal/ui/components/profile/handler.go`'s call to `ph.auth.SwitchProfile`, that's expected. Open `handler.go`, find `func (ph *Handler) SwitchProfile`, and delete the entire function. Then re-build.

- [ ] **Step 5: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/auth.go internal/auth/translator.go internal/auth/translator_test.go internal/ui/components/profile/handler.go
git commit -m "Delete auth.SwitchProfile and the legacy translator"
```

---

### Task 3: Delete legacy elastic entry points

**Files:**
- Modify: `internal/services/elastic/elastic.go` (delete `NewService`, `Reinitialize`, `legacySophosEnvFromAWSConfig`)
- Modify or delete: `internal/services/elastic/dragos.go` (delete `NewDragosService`, `ReinitializeDragos`, `legacyDragosEnvFromSession`)

- [ ] **Step 1: Inspect `dragos.go` to see what's left**

Run: `grep -n 'func \|type ' internal/services/elastic/dragos.go`

Expected: just `NewDragosService`, `ReinitializeDragos`, `legacyDragosEnvFromSession` (after phase 3's cleanup). If that's all, delete the file entirely.

- [ ] **Step 2: Delete `dragos.go` (entirely if it only has the legacy wrappers)**

If the file only contains the three functions above:

```bash
rm internal/services/elastic/dragos.go
```

If the file contains additional symbols beyond those three, open it and delete only the three legacy functions. Keep everything else.

- [ ] **Step 3: Delete legacy functions from `elastic.go`**

In `internal/services/elastic/elastic.go`, find and delete:

- `func NewService(cfg aws.Config) (*Service, error) { return NewServiceFromEnv(...) }`
- `func (s *Service) Reinitialize(cfg aws.Config, profile string) error { return s.ReinitializeFromEnv(...) }`
- `func legacySophosEnvFromAWSConfig(...) environments.Environment { ... }`

The `Service` struct + `PreloadIndexStats` + `ListIndices` + `Close` + `IndexStats` + `parseSize` + `formatSize` + utility functions stay.

- [ ] **Step 4: Inspect imports**

After deletion, `elastic.go` may have orphaned imports. Likely:

- `github.com/tpe11etier/cloudcutter/internal/config` — was used by `legacySophosEnvFromAWSConfig`. Now unused. Remove.
- `github.com/tpe11etier/cloudcutter/internal/environments` — same. Remove.
- `fmt` — still used by error messages elsewhere. Keep.

The compiler will tell you which imports are unused. Iterate until clean.

- [ ] **Step 5: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds. Any caller of `NewService(cfg)` / `Reinitialize` / `NewDragosService(d)` / `ReinitializeDragos(d)` will fail to compile — but per phase 4, those callers no longer exist. If the compiler reports any, they're unexpected; investigate.

- [ ] **Step 6: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/services/elastic/elastic.go internal/services/elastic/dragos.go
git commit -m "Delete legacy elastic.NewService / NewDragosService / Reinitialize wrappers"
```

If `dragos.go` was deleted, git records the deletion automatically.

---

### Task 4: Delete legacy services initializers

**Files:**
- Modify: `internal/services/services.go` (delete `InitializeElastic`, `InitializeElasticDragos`)

- [ ] **Step 1: Inspect the file**

Run: `cat internal/services/services.go`

The `Services` struct + `New(cfg, region)` + `InitializeDynamoDB(cfg)` stay. Delete only `InitializeElastic(cfg)` and `InitializeElasticDragos(d)`.

- [ ] **Step 2: Delete the two functions**

In `internal/services/services.go`, find and delete:

- `func (s *Services) InitializeElastic(cfg aws.Config) error { ... }`
- `func (s *Services) InitializeElasticDragos(d *auth.DragosSession) error { ... }`

The second function's parameter type `*auth.DragosSession` will go away in T6, but for now (until T6) it still exists. After this task, the only reference to `*auth.DragosSession` in services.go is gone.

- [ ] **Step 3: Inspect imports**

After deletion, `services.go` may have unused imports:

- `github.com/tpe11etier/cloudcutter/internal/auth` — was used for `*auth.DragosSession`. If nothing else in `services.go` uses `auth.*`, remove.

Check: `grep -n 'auth\.' internal/services/services.go`. If no matches, remove the import.

- [ ] **Step 4: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds.

- [ ] **Step 5: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/services/services.go
git commit -m "Delete services.InitializeElastic and InitializeElasticDragos"
```

---

### Task 5: Delete legacy profile picker code

**Files:**
- Modify: `internal/ui/components/profile/profile.go` (delete `discoverProfiles`, `switchProfile`, prune imports)

- [ ] **Step 1: Inspect what's left in profile.go**

Run: `grep -n 'func ' internal/ui/components/profile/profile.go`

Expected at this point: `NewSelector`, `(*Selector).switchProfile` (dead), `(*Selector).ShowSelector`, `(*Selector).discoverProfiles` (dead).

- [ ] **Step 2: Delete `switchProfile` and `discoverProfiles`**

Open `internal/ui/components/profile/profile.go` and delete:

- `func (ps *Selector) switchProfile(profile string) { ... }` — was the legacy dispatch with the dragos-special-case bypass.
- `func (ps *Selector) discoverProfiles() []string { ... }` — was the legacy `~/.aws/credentials` reader.

Keep `NewSelector` and `ShowSelector`.

- [ ] **Step 3: Prune imports**

The deleted functions used `context`, `os`, `path/filepath`, `strings`, `github.com/aws/aws-sdk-go-v2/aws`, `github.com/tpe11etier/cloudcutter/internal/auth`, `gopkg.in/ini.v1`.

After deletion, check which are still used by the surviving `NewSelector`/`ShowSelector`:

Run: `grep -n 'context\.\|os\.\|filepath\.\|strings\.\|aws\.\|auth\.\|ini\.' internal/ui/components/profile/profile.go`

For each that isn't referenced, remove from the import block. Likely orphans (after the deletions): `context`, `os`, `path/filepath`, `aws`, `auth`, `ini`. `strings` is probably still used by `sort.Strings(...)`.

The build will tell you exactly which are unused — iterate.

- [ ] **Step 4: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds.

- [ ] **Step 5: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/components/profile/profile.go
git commit -m "Delete legacy profile.Selector.discoverProfiles and switchProfile"
```

---

### Task 6: Delete `Session.Dragos` + `auth.DragosSession` + back-compat population

**Files:**
- Modify: `internal/auth/auth.go` (delete `Session.Dragos` field, `auth.DragosSession` type, back-compat population branch in `SwitchEnvironment`)

- [ ] **Step 1: Find the back-compat population branch**

In `internal/auth/auth.go`, find the `SwitchEnvironment` function. Inside the `case "jwt":` branch, locate this block:

```go
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
```

Delete the entire block. The `session.Config = aws.Config{Region: env.Region}` line is also dragos-specific (it set the AWS region for a session that doesn't actually use AWS). After deletion, JWT sessions have a zero-value `aws.Config`, which is correct for non-AWS auth.

- [ ] **Step 2: Delete `Session.Dragos` field**

In `internal/auth/auth.go`, find the `Session` struct:

```go
type Session struct {
	Config  aws.Config
	Profile string
	Region  string
	// Dragos is populated only when Profile == DragosProfile.
	// ...long comment...
	Dragos *DragosSession
	// Environment is the resolved description of the active backend.
	// ...
	Environment environments.Environment
	// Token is the JWT for jwt-typed environments.
	// ...
	Token string
}
```

Delete the `Dragos *DragosSession` field and its preceding comment block. The new struct is:

```go
type Session struct {
	Config      aws.Config
	Profile     string
	Region      string
	Environment environments.Environment
	Token       string
}
```

- [ ] **Step 3: Delete the `DragosSession` type**

Find `type DragosSession struct { ... }` and delete the entire definition (and the preceding comment).

- [ ] **Step 4: Delete the `DragosProfile` constant**

Find:

```go
// DragosProfile is the profile name selected from the picker to use Dragos.
const DragosProfile = "dragos"
```

And delete it. Confirm no callers:

Run: `grep -rn 'auth\.DragosProfile\|DragosProfile\b' --include='*.go' .`

Expected: no remaining references.

- [ ] **Step 5: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds. If anything still references `DragosSession` or `DragosProfile`, the compiler will name it; investigate.

- [ ] **Step 6: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/auth/auth.go
git commit -m "Delete Session.Dragos, auth.DragosSession type, DragosProfile constant"
```

---

### Task 7: Delete `dragos_login.go`, `dragos_config.go`, opal config

**Files:**
- Delete: `internal/auth/dragos_login.go`
- Delete: `internal/auth/dragos_config.go`
- Delete: `internal/auth/config.go`
- Modify: `internal/auth/auth.go` (drop `opalConfig` + `opalProfiles` fields; `New(statusFn)` doesn't call `LoadOpalConfig`)

- [ ] **Step 1: Confirm callers**

Run: `grep -rn 'LoginWithPassword\|LoadDragosConfig\|SaveDragosToken\|DragosConfigPath\|DefaultDragos\|loadDragosCookieFromBrowser\|LoadOpalConfig\|OpalConfig\|getEnvOrDefault' --include='*.go' .`

Expected: no callers outside `internal/auth/` itself (T1 already replaced the modal's calls with `LoginJWT` / `WriteTokenFile`). If the grep finds a caller, escalate.

`Authenticator.opalProfiles` is referenced inside `auth.go` itself by the deleted `legacyToEnvironment` (already gone in T2) — no more readers.

- [ ] **Step 2: Delete the three files**

```bash
rm internal/auth/dragos_login.go internal/auth/dragos_config.go internal/auth/config.go
```

- [ ] **Step 3: Update `Authenticator` struct + `New`**

In `internal/auth/auth.go`, find the `Authenticator` struct:

```go
type Authenticator struct {
	mu               sync.RWMutex
	currentSession   *Session
	isAuthenticating bool
	onStatus         func(string)
	opalConfig       OpalConfig
	opalProfiles     map[string]string // maps profile names to role IDs
}
```

Delete the `opalConfig` and `opalProfiles` fields:

```go
type Authenticator struct {
	mu               sync.RWMutex
	currentSession   *Session
	isAuthenticating bool
	onStatus         func(string)
}
```

Find `func New(statusFn func(string)) (*Authenticator, error)`:

```go
func New(statusFn func(string)) (*Authenticator, error) {
	opalConfig := LoadOpalConfig()

	opalProfiles := make(map[string]string)
	for _, env := range opalConfig.Environments {
		for _, profileTag := range env.ProfileTags {
			opalProfiles[profileTag] = env.RoleID
		}
	}

	return &Authenticator{
		onStatus:     statusFn,
		opalConfig:   opalConfig,
		opalProfiles: opalProfiles,
	}, nil
}
```

Replace with:

```go
func New(statusFn func(string)) (*Authenticator, error) {
	return &Authenticator{
		onStatus: statusFn,
	}, nil
}
```

- [ ] **Step 4: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds.

- [ ] **Step 5: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS. The phase-1 `auth_test.go` (or whatever test verifies New) may need updating — read the failures, update in place.

- [ ] **Step 6: gofmt sanity**

Run: `gofmt -l internal/auth`

Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add internal/auth/
git commit -m "Delete dragos_login.go, dragos_config.go, config.go (opal)"
```

(The `git add internal/auth/` covers both the deletions and the auth.go modification.)

---

### Task 8: Delete `Manager.CurrentProfile` if dead + phase 5 verification + tag

**Files:**
- Modify: `internal/ui/manager/manager.go` (possibly delete `CurrentProfile()`)

- [ ] **Step 1: Check whether `Manager.CurrentProfile()` has callers**

Run: `grep -rn 'CurrentProfile()\|GetCurrentProfile()' --include='*.go' . | grep -v _test.go`

If any callers exist outside `internal/ui/manager/manager.go` itself or `internal/ui/components/profile/handler.go`, keep the method. If no callers, delete it.

- [ ] **Step 2: Delete `Manager.CurrentProfile()` if dead**

If Step 1 showed no callers, find in `internal/ui/manager/manager.go`:

```go
func (vm *Manager) CurrentProfile() string {
	return vm.profileHandler.GetCurrentProfile()
}
```

Delete it. If `profile.Handler.GetCurrentProfile` is also unreferenced after this, delete that too.

- [ ] **Step 3: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds.

- [ ] **Step 4: Final verification**

Run: `go test -mod=vendor ./...`

Expected: every package PASSes.

Run: `go vet -mod=vendor ./...`

Expected: no output.

Run: `gofmt -l internal/auth internal/ui internal/services cmd`

Expected: no output. If output, run `gofmt -w` on the listed files.

- [ ] **Step 5: Confirm vendor names are gone from production code**

Run these greps. Each should return zero matches in production Go files (excluding `cmd/dragos-cookie-probe/` diagnostic CLI, `~/.cloudcutter/dragos.token` token file, comments referencing the migration):

```bash
grep -rn 'DragosSession\|DragosProfile\|DragosConfig\b\|LoadDragosConfig\|SaveDragosToken\|DefaultDragos\|loadDragosCookieFromBrowser\|legacyToEnvironment\|legacySophosEnvFromAWSConfig\|legacyDragosEnvFromSession\|LoginWithPassword\|OpalConfig\|LoadOpalConfig\|ShowDragosLoginModal' --include='*.go' . | grep -v cmd/dragos-cookie-probe
```

Expected: no output (or only matches in doc comments / migration-related code in `cmd/cloudcutter/init_cmd.go` which describes legacy file shapes for the migrator).

If matches appear in production files outside expected places, investigate and clean up.

- [ ] **Step 6: Smoke test**

```bash
go build -mod=vendor -o /tmp/cloudcutter-phase5 ./cmd/cloudcutter
/tmp/cloudcutter-phase5
```

Expected: cloudcutter starts, picker shows your YAML environments, switching works, login modal pops on stale tokens with the form rendered from `env.Auth.Login.BodyFields`. Same UX as phase 4.

If the smoke test fails, capture the error and fix before tagging.

- [ ] **Step 7: Tag**

```bash
git tag phase-5-complete
```

- [ ] **Step 8: Update spec status**

Edit `docs/superpowers/specs/2026-05-09-generic-backend-design.md`:

From:

```
**Status**: Phase 4 implemented (tag `phase-4-complete`). Phase 5 (delete legacy) plan to be drafted next.
```

To:

```
**Status**: All phases complete (tag `phase-5-complete`). Generic-backend refactor done.
```

```bash
git add docs/superpowers/specs/2026-05-09-generic-backend-design.md
git commit -m "Note phase 5 complete in spec status — refactor done"
```

- [ ] **Step 9: Optional commit message body for the final commit (or PR description)**

Five-phase summary worth quoting in any final review of the branch:

```
Generic-backend refactor complete (phases 1-5).

The cloudcutter binary that started this branch had Sophos and
Dragos vendor names baked into ~30 places across auth, services,
and UI. The branch ends with zero vendor names in production Go
code (the user's ~/.cloudcutter/config.yaml carries them in user-
supplied entries instead).

Phase 1: ship config.yaml schema, Resolver, Materialize, first-run
starter, cloudcutter init migrator. No behavior change.

Phase 2: refactor auth dispatch onto Environment.Auth.Type;
internal/probe.Run consolidates the two existing probes.

Phase 3: refactor transports onto config.TransportSpec; one
NewServiceFromEnv constructor dispatches transport choice.

Phase 4: cut the running app over to YAML — cmd/cloudcutter loads
config.yaml at startup and threads the Resolver to the manager;
the picker reads from Resolver.List(); switchToEnvironment
collapses the five vendor switch funcs into one.

Phase 5: delete every dead legacy entry point now that nothing
references them. ShowDragosLoginModal generalizes into
ShowJWTLoginModal parameterized by env.Auth.Login.BodyFields.
```

---

## What stays (intentionally) after phase 5

- **`~/.cloudcutter/dragos.token`** as a user-config file path. The user's YAML names this file; renaming it requires updating the YAML and the migrated dragos.json migrator. Out of scope.
- **`cmd/dragos-cookie-probe/`** diagnostic CLI. Useful debugging tool, not in the running-app path.
- **`cmd/cloudcutter/init_cmd.go`** — `cloudcutter init` migrator that reads legacy `dragos.json` / `opal.json` and synthesizes a YAML config. The legacy-file shapes remain in this code as known-input formats for migration. New users can still run it once.
- **The `~/.cloudcutter/dragos.json.bak`** file in the user's home directory — that's their file, not the project's.

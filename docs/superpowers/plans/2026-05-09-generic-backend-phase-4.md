# Generic Backend — Phase 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Switch the running app from legacy profile-name dispatch to YAML-driven environment dispatch. The picker reads from `~/.cloudcutter/config.yaml` (∪ AWS profiles) via the Resolver. Selecting an environment routes through one `switchToEnvironment(name)` that resolves+materializes via `environments`, calls `auth.SwitchEnvironment(env)`, and constructs the elastic service via `elastic.NewServiceFromEnv`. The view reads `session.Environment.Transport.Type` instead of `session.Dragos != nil`.

**Architecture:** `cmd/cloudcutter/main.go` loads `config.yaml`, validates, builds an `environments.Resolver`, and threads it to the Manager constructor. Manager gains `switchToEnvironment(name)` and a new `profile.Handler.SwitchEnvironment(ctx, env, callback)` method. The picker's source becomes `Resolver.List()`. The five vendor-named `switchTo*Profile` methods on Manager are deleted; the picker invokes `onSelect(name)` and the manager dispatches via `switchToEnvironment`. `view.Reinitialize` reads the active session and calls `service.ReinitializeFromEnv(env, awsCfg, token)`. The lazy elastic view in `main.go` switches to `elastic.NewServiceFromEnv`.

**Tech Stack:** Go, existing `internal/config` + `internal/environments` + `internal/auth` + `internal/services/elastic` packages. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-05-09-generic-backend-design.md` — particularly the "Phase 4" row, the "What collapses" table, and the `Resolver` interface description.

**Status as of plan-write time:** phases 1, 2, 3 are complete (tags `phase-1-complete`, `phase-2-complete`, `phase-3-complete`). Phase 4 ties them together at the call sites.

---

## Out of scope (deferred to phase 5)

- **Deleting legacy entry points**: `elastic.NewService(cfg)`, `elastic.NewDragosService(d)`, `elastic.Reinitialize(cfg, profile)`, `elastic.ReinitializeDragos(d)`, `elastic.legacySophosEnvFromAWSConfig`, `elastic.legacyDragosEnvFromSession`, `services.InitializeElastic`, `services.InitializeElasticDragos` all become unreachable after phase 4 but stay as dead code. Phase 5 deletes them.
- **`auth.SwitchProfile`** and `auth.legacyToEnvironment` lose their UI callers in phase 4 but stay as dead code. Phase 5 deletes them.
- **`auth.DragosSession`** field on `Session` and the legacy DragosSession-population branch in `SwitchEnvironment` survive phase 4. Phase 5 deletes them once nothing reads `session.Dragos` anymore.
- **`auth.LoadDragosConfig`, `auth.SaveDragosToken`, `auth.DragosConfigPath`, `OpalConfig`** stay until phase 5.
- **`profile.Selector.discoverProfiles`** the legacy AWS-credentials reader stays as dead code in phase 4. Phase 5 deletes it.
- **`profile.Handler.SwitchProfile`** has no callers after phase 4 but stays. Phase 5 deletes it.

The cleanup phase (5) is mechanical deletion-only, easy to plan after phase 4 ships.

---

## File structure

**New files**

| File | Responsibility |
|---|---|
| `internal/ui/components/profile/handler_env.go` | `(*Handler).SwitchEnvironment(ctx, env, callback)` — async wrapper around `auth.SwitchEnvironment`. Companion to existing `SwitchProfile`. Splitting into a separate file keeps the diff isolated and the legacy `SwitchProfile` easy to delete in phase 5. |
| `internal/ui/components/profile/handler_env_test.go` | Unit test for `SwitchEnvironment` using a fake authenticator-like surface. |

**Modified files**

| File | Change |
|---|---|
| `cmd/cloudcutter/main.go` | At startup, after the first-run check, call `config.Load(...)` + `config.Validate(...)` + `environments.DiscoverAWSProfiles(...)` + `environments.NewResolver(...)`. Pass the resolver into `manager.NewViewManager`. The lazy `ViewElastic` registration switches from `services.InitializeElastic` / `services.InitializeElasticDragos` to `elastic.NewServiceFromEnv(session.Environment, session.Config, session.Token)`. |
| `internal/ui/manager/manager.go` | `Manager` gains a `*environments.Resolver` field. `NewViewManager` takes it. Add `switchToEnvironment(name)` that resolves+materializes+invokes `profileHandler.SwitchEnvironment`. The five vendor switch funcs (`switchToDragosProfile`, `switchToDevProfile`, `switchToProdProfile`, `switchToLocalProfile`, `switchToStandardProfile`) are deleted. `ShowProfileSelector`'s callback dispatches via `switchToEnvironment(name)`. The dragos-login modal trigger logic (popping the modal on jwt-auth-failure) moves into the new `switchToEnvironment`. |
| `internal/ui/components/profile/profile.go` | The picker reads from `manager.Resolver().List()` (sorted) instead of `discoverProfiles()`. The selection callback always invokes `onSelect(name)` — there is no special-casing for "dragos" anymore. The legacy `discoverProfiles` and `switchProfile` (which called `ph.SwitchProfile`) become dead code; phase 5 deletes them. |
| `internal/ui/views/elastic/view.go` | `Reinitialize` reads `session := manager.CurrentSession()` and calls `service.ReinitializeFromEnv(session.Environment, session.Config, session.Token)`. The `session.Dragos != nil` branch is removed. The local-region timeframe-clearing special case checks `session.Environment.Auth.Type == "none"` instead of `cfg.Region == "local"` (semantically equivalent for the local env, but doesn't reach into AWS-config terms). The Dragos index-pattern reset logic also moves to read from `session.Environment.IndexPattern`. |

**Files NOT modified** (despite being touched by previous phases): `internal/auth/*` (legacy code stays), `internal/services/elastic/*` (legacy entry points stay), `internal/services/services.go` (legacy initializers stay).

---

## Tasks

### Task 1: `profile.Handler.SwitchEnvironment` — async Environment-based switch

**Files:**
- Create: `internal/ui/components/profile/handler_env.go`
- Create: `internal/ui/components/profile/handler_env_test.go`

- [ ] **Step 1: Read the existing Handler structure**

Run: `cat internal/ui/components/profile/handler.go`

Note: `Handler` has `auth *auth.Authenticator`, `statusChan chan<- string`, `mu sync.RWMutex`, `region string`, `onLoadStart`, `onLoadEnd`. The existing `SwitchProfile` method calls `ph.auth.SwitchProfile` in a goroutine.

The new method calls `ph.auth.SwitchEnvironment` (added in phase 2) instead.

- [ ] **Step 2: Write the failing test**

Write `internal/ui/components/profile/handler_env_test.go`:

```go
package profile

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/tpe11etier/cloudcutter/internal/auth"
	"github.com/tpe11etier/cloudcutter/internal/config"
	"github.com/tpe11etier/cloudcutter/internal/environments"
)

// TestSwitchEnvironmentCallsBackWithSession exercises the happy path:
// SwitchEnvironment dispatches to auth.SwitchEnvironment in a goroutine,
// then invokes the callback on completion with (*auth.Session, nil).
func TestSwitchEnvironmentCallsBackWithSession(t *testing.T) {
	ph, err := NewProfileHandler(make(chan string, 4), nil, nil)
	if err != nil {
		t.Fatalf("NewProfileHandler: %v", err)
	}

	env := environments.Environment{
		Name: "local",
		Auth: config.AuthSpec{Type: "none"},
		Transport: config.TransportSpec{
			Type:    "plain",
			BaseURL: "http://localhost:9200",
		},
	}

	var gotSess *auth.Session
	var gotErr error
	done := make(chan struct{})
	ph.SwitchEnvironment(context.Background(), env, func(sess *auth.Session, err error) {
		gotSess = sess
		gotErr = err
		close(done)
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("callback never fired")
	}

	if gotErr != nil {
		t.Errorf("expected nil error, got %v", gotErr)
	}
	if gotSess == nil {
		t.Fatal("expected non-nil session")
	}
	if gotSess.Environment.Name != "local" {
		t.Errorf("Session.Environment.Name = %q, want local", gotSess.Environment.Name)
	}
}

// TestSwitchEnvironmentForwardsErrors confirms that auth errors propagate
// through the callback as (nil, err).
func TestSwitchEnvironmentForwardsErrors(t *testing.T) {
	ph, err := NewProfileHandler(make(chan string, 4), nil, nil)
	if err != nil {
		t.Fatalf("NewProfileHandler: %v", err)
	}

	env := environments.Environment{
		Name: "x",
		Auth: config.AuthSpec{Type: "exotic"}, // unknown type → SwitchEnvironment errors
		Transport: config.TransportSpec{
			Type:    "plain",
			BaseURL: "http://example.com",
		},
	}

	var gotErr error
	done := make(chan struct{})
	ph.SwitchEnvironment(context.Background(), env, func(sess *auth.Session, err error) {
		gotErr = err
		close(done)
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("callback never fired")
	}
	if gotErr == nil {
		t.Fatal("expected error for unknown auth.type")
	}
	if !errors.Is(gotErr, gotErr) { // sanity: gotErr is non-nil and comparable
		t.Errorf("got %v", gotErr)
	}
}

// TestSwitchEnvironmentInvokesLifecycleHooks verifies onLoadStart/onLoadEnd
// fire even on error paths.
func TestSwitchEnvironmentInvokesLifecycleHooks(t *testing.T) {
	var startMsg string
	var endCalled bool
	var mu sync.Mutex

	ph, err := NewProfileHandler(
		make(chan string, 4),
		func(msg string) { mu.Lock(); startMsg = msg; mu.Unlock() },
		func() { mu.Lock(); endCalled = true; mu.Unlock() },
	)
	if err != nil {
		t.Fatalf("NewProfileHandler: %v", err)
	}

	env := environments.Environment{
		Name: "local",
		Auth: config.AuthSpec{Type: "none"},
		Transport: config.TransportSpec{
			Type:    "plain",
			BaseURL: "http://localhost:9200",
		},
	}

	done := make(chan struct{})
	ph.SwitchEnvironment(context.Background(), env, func(sess *auth.Session, err error) {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("callback never fired")
	}

	mu.Lock()
	defer mu.Unlock()
	if startMsg == "" {
		t.Error("onLoadStart never invoked")
	}
	if !endCalled {
		t.Error("onLoadEnd never invoked")
	}
}
```

- [ ] **Step 3: Run the tests to confirm they fail**

Run: `go test -mod=vendor ./internal/ui/components/profile -run TestSwitchEnvironment`

Expected: FAIL — `undefined: SwitchEnvironment` (method on Handler).

- [ ] **Step 4: Implement `SwitchEnvironment`**

Write `internal/ui/components/profile/handler_env.go`:

```go
package profile

import (
	"context"
	"fmt"

	"github.com/tpe11etier/cloudcutter/internal/auth"
	"github.com/tpe11etier/cloudcutter/internal/environments"
)

// SwitchEnvironment is the Environment-aware sibling of SwitchProfile.
// It runs auth.SwitchEnvironment in a goroutine and invokes the callback
// with the resulting Session (or an error) on the goroutine — callers
// are responsible for marshaling UI updates onto the tview main loop via
// QueueUpdateDraw.
//
// SwitchProfile is preserved for phases 1-3 callers; phase 5 deletes it
// once nothing references the legacy path.
func (ph *Handler) SwitchEnvironment(ctx context.Context, env environments.Environment, callback func(*auth.Session, error)) {
	if ph.onLoadStart != nil {
		ph.onLoadStart(fmt.Sprintf("Authenticating environment: %s", env.Name))
	}

	go func() {
		defer func() {
			if ph.onLoadEnd != nil {
				ph.onLoadEnd()
			}
		}()

		session, err := ph.auth.SwitchEnvironment(ctx, env)
		if err != nil {
			ph.sendStatus(fmt.Sprintf("Authentication failed: %v", err))
			callback(nil, err)
			return
		}

		ph.sendStatus(fmt.Sprintf("Successfully authenticated: %s", env.Name))
		callback(session, nil)
	}()
}
```

- [ ] **Step 5: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/ui/components/profile -run TestSwitchEnvironment -v`

Expected: PASS for all three sub-tests.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/components/profile/handler_env.go internal/ui/components/profile/handler_env_test.go
git commit -m "Add profile.Handler.SwitchEnvironment async wrapper"
```

---

### Task 2: Manager constructor accepts `*environments.Resolver`

**Files:**
- Modify: `internal/ui/manager/manager.go`
- Modify: `cmd/cloudcutter/main.go`

- [ ] **Step 1: Inspect the current constructor**

Run: `grep -n 'func NewViewManager' internal/ui/manager/manager.go`

The signature is `NewViewManager(ctx context.Context, app *ui.App, awsConfig aws.Config, log *logger.Logger) *Manager`.

- [ ] **Step 2: Add the resolver field and update the constructor signature**

In `internal/ui/manager/manager.go`, locate the `Manager` struct definition (around line 35). Add a new field at the bottom of the struct:

```go
	resolver       *environments.Resolver
```

(alphabetically the new field can go between `prompt` and the next field, but placing it near `profileHandler` is more readable; either is fine — match the existing style.)

Add to the import block (alphabetized with other internal imports):

```go
	"github.com/tpe11etier/cloudcutter/internal/environments"
```

Update the constructor:

```go
func NewViewManager(ctx context.Context, app *ui.App, awsConfig aws.Config, log *logger.Logger, resolver *environments.Resolver) *Manager {
	ctx, cancel := context.WithCancel(ctx)
	vm := &Manager{
		ctx:            ctx,
		cancelFunc:     cancel,
		app:            app,
		views:          make(map[string]views.View),
		pages:          tview.NewPages(),
		header:         header.NewHeader(),
		statusBar:      statusbar.NewStatusBar(),
		prompt:         components.NewPrompt(),
		filterPrompt:   components.NewPrompt(),
		awsConfig:      awsConfig,
		primitivesByID: make(map[string]tview.Primitive),
		StatusChan:     make(chan string, 10),
		help:           help.NewHelp(),
		logger:         log,
		resolver:       resolver,
	}
	// ... rest unchanged
```

Add a public accessor (the picker needs it in Task 4):

```go
// Resolver returns the environments.Resolver. May be nil if the binary was
// constructed without one (only happens in tests today).
func (vm *Manager) Resolver() *environments.Resolver {
	return vm.resolver
}
```

- [ ] **Step 3: Update the existing `manager_test.go` to use the new signature**

Run: `grep -n 'NewViewManager' internal/ui/manager/manager_test.go`

If the test calls `NewViewManager(...)` with the old 4-arg signature, update each callsite to pass `nil` as the resolver argument:

```go
vm := NewViewManager(ctx, app, awsConfig, log, nil)
```

Repeat for every test that constructs a Manager.

- [ ] **Step 4: Update `cmd/cloudcutter/main.go` to construct + pass the resolver**

In `cmd/cloudcutter/main.go`, locate `func runApplication()`. After the existing first-run hook (around the `wrote, err := config.EnsureExists(path)` block) and before `viewManager := manager.NewViewManager(...)`, insert:

```go
	configPath := config.DefaultConfigPath()
	rawCfg, err := config.Load(configPath)
	if err != nil {
		logInstance.Error("Failed to load config", "path", configPath, "error", err)
		fmt.Fprintf(os.Stderr, "cloudcutter: failed to load %s: %v\n", configPath, err)
		os.Exit(1)
	}
	if err := config.Validate(rawCfg); err != nil {
		logInstance.Error("Config validation failed", "error", err)
		fmt.Fprintf(os.Stderr, "cloudcutter: config validation: %v\n", err)
		os.Exit(1)
	}
	homeDir, _ := os.UserHomeDir()
	awsProfiles, _ := environments.DiscoverAWSProfiles(homeDir) // missing-creds = empty list
	resolver := environments.NewResolver(rawCfg, awsProfiles)
```

The `_ = err` discard on `os.UserHomeDir()` is intentional: if home can't be resolved, `DefaultConfigPath()` already returned `""` and the EnsureExists check would have failed earlier.

The `_, _ = environments.DiscoverAWSProfiles(...)` ignores the error path because phase-1 already established that a missing `~/.aws/credentials` is not fatal (returns nil, nil).

Add the new imports at the top:

```go
	"github.com/tpe11etier/cloudcutter/internal/environments"
```

Update the `NewViewManager` call:

```go
	viewManager := manager.NewViewManager(ctx, app, defaultConfig, logInstance, resolver)
```

- [ ] **Step 5: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds.

If the compiler complains about a duplicate import (e.g. `environments` already imported in main.go), de-dupe.

- [ ] **Step 6: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/manager/manager.go internal/ui/manager/manager_test.go cmd/cloudcutter/main.go
git commit -m "Thread environments.Resolver through Manager constructor"
```

---

### Task 3: `Manager.switchToEnvironment(name)` — unified dispatch

**Files:**
- Modify: `internal/ui/manager/manager.go` (add new method)

- [ ] **Step 1: Add `switchToEnvironment` method**

In `internal/ui/manager/manager.go`, add this method (place it near the existing `switchToDragosProfile` for now; Task 4 deletes the legacy ones):

```go
// switchToEnvironment is the unified profile-switch entry point introduced
// in phase 4. Resolves the named environment via the YAML resolver,
// materializes it with the current region, and dispatches via
// profileHandler.SwitchEnvironment. The callback updates the header,
// reinitializes the active view, and (for jwt environments) pops the
// login modal on auth failure.
func (vm *Manager) switchToEnvironment(name string) {
	if vm.resolver == nil {
		vm.StatusChan <- "Configuration error: no environment resolver"
		return
	}
	if vm.profileHandler.IsAuthenticating() {
		vm.StatusChan <- "Authentication already in progress"
		return
	}

	spec, err := vm.resolver.Resolve(name)
	if err != nil {
		vm.StatusChan <- fmt.Sprintf("Resolve %q: %v", name, err)
		return
	}

	region := vm.profileHandler.GetRegion()
	env, err := environments.Materialize(spec, region)
	if err != nil {
		vm.StatusChan <- fmt.Sprintf("Materialize %q: %v", name, err)
		return
	}

	// Dragos-style jwt environments need the login modal popped on auth
	// failure if the user has no token cached. Detect this before invoking
	// SwitchEnvironment so the modal appears with no detour through a
	// status-bar error.
	needsLoginModal := env.Auth.Type == "jwt" && env.Auth.Login != nil

	vm.profileHandler.SwitchEnvironment(vm.ctx, env, func(sess *auth.Session, err error) {
		if err != nil {
			vm.app.QueueUpdateDraw(func() {
				vm.statusBar.SetText(fmt.Sprintf("Auth failed: %v", err))
				if needsLoginModal {
					vm.ShowDragosLoginModal(
						func() { vm.switchToEnvironment(name) },
						func() { vm.StatusChan <- "Auth canceled" },
					)
				}
			})
			return
		}

		vm.app.QueueUpdateDraw(func() {
			vm.awsConfig = sess.Config
			vm.header.UpdateEnvVar("Profile", sess.Environment.Name)
			if sess.Environment.Auth.Type == "aws_sdk" {
				vm.header.UpdateEnvVar("Region", sess.Environment.Region)
			} else {
				vm.header.UpdateEnvVar("Region", "—")
			}
		})

		if err := vm.reinitializeActiveView(); err != nil {
			vm.app.QueueUpdateDraw(func() {
				vm.statusBar.SetText(fmt.Sprintf("Reinit failed: %v", err))
				if needsLoginModal {
					vm.ShowDragosLoginModal(
						func() { vm.switchToEnvironment(name) },
						func() { vm.StatusChan <- "Auth canceled" },
					)
				}
			})
			return
		}

		if err := vm.SwitchToView(ViewElastic); err != nil {
			vm.Logger().Error("Failed to switch to Elastic", "error", err)
		}

		vm.StatusChan <- fmt.Sprintf("Switched to %s", sess.Environment.Name)
	})
}
```

The method needs `auth.Session` from the existing import. Confirm `internal/auth` is already imported in manager.go.

- [ ] **Step 2: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds. The new method is added but has no callers yet (Task 4 wires it in); it's dead code at this point.

- [ ] **Step 3: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/manager/manager.go
git commit -m "Add Manager.switchToEnvironment for unified profile dispatch"
```

---

### Task 4: Picker uses `Resolver.List()`; delete the 5 vendor switch funcs

**Files:**
- Modify: `internal/ui/components/profile/profile.go`
- Modify: `internal/ui/manager/manager.go` (delete 5 funcs + update `ShowProfileSelector`)

- [ ] **Step 1: Update the `Manager` interface in profile.go to expose Resolver**

In `internal/ui/components/profile/profile.go`, find the `Manager` interface (around line 19):

```go
type Manager interface {
	Pages() *tview.Pages
}
```

Replace with:

```go
type Manager interface {
	Pages() *tview.Pages
	Resolver() *environments.Resolver
}
```

Add to the import block:

```go
	"github.com/tpe11etier/cloudcutter/internal/environments"
```

- [ ] **Step 2: Replace `discoverProfiles` with Resolver-driven discovery**

In `profile.go`, find `NewSelector` and `discoverProfiles`. Replace the body of `NewSelector` so it uses the resolver:

```go
func NewSelector(ph *Handler, onSelect func(profile string), onCancel func(), statusBar *statusbar.StatusBar, manager Manager) *Selector {
	selector := &Selector{
		List:      tview.NewList().ShowSecondaryText(false),
		onSelect:  onSelect,
		onCancel:  onCancel,
		ph:        ph,
		statusBar: statusBar,
		manager:   manager,
	}

	selector.
		SetMainTextColor(style.GruvboxMaterial.Foreground).
		SetSelectedStyle(tcell.StyleDefault.
			Foreground(tcell.ColorLightYellow).
			Background(tcell.ColorDarkCyan)).
		SetBorder(true).
		SetTitle(" Select Environment ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(style.GruvboxMaterial.Foreground).
		SetBorderColor(tcell.ColorMediumTurquoise)

	if r := manager.Resolver(); r != nil {
		selector.profiles = r.List()
		sort.Strings(selector.profiles)
	}

	for _, profile := range selector.profiles {
		selector.AddItem(profile, "", 0, nil)
	}

	selector.SetSelectedFunc(func(index int, name string, secondName string, shortcut rune) {
		// All dispatch goes through the manager's onSelect callback.
		// Special-casing per profile name is gone in phase 4.
		selector.onSelect(name)
	})

	selector.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			if selector.onCancel != nil {
				selector.onCancel()
			}
			return nil
		}
		return event
	})

	selector.SetCurrentItem(0)
	return selector
}
```

Note: the `auth` import in profile.go is no longer needed (the `auth.DragosProfile` usage is gone). If unused, remove it.

`switchProfile` (the method that dispatched to `ph.SwitchProfile` for non-dragos) and `discoverProfiles` are now unreachable but kept as dead code for phase 5 to delete. Don't touch them.

- [ ] **Step 3: Update `Manager.ShowProfileSelector` to dispatch via `switchToEnvironment`**

In `internal/ui/manager/manager.go`, find `ShowProfileSelector` (around line 681). Replace its body:

```go
func (vm *Manager) ShowProfileSelector() (tview.Primitive, error) {
	profileSelector := profile.NewSelector(
		vm.profileHandler,
		func(name string) {
			if vm.activeView != nil {
				vm.focusActiveView()
			}
			vm.statusBar.SetText(fmt.Sprintf("Switching to %s...", name))
			vm.switchToEnvironment(name)
		},
		func() {
			vm.pages.RemovePage("profileSelector")
			if vm.activeView != nil {
				vm.focusActiveView()
			}
		},
		vm.statusBar,
		vm,
	)

	return profileSelector.ShowSelector()
}
```

- [ ] **Step 4: Delete the 5 vendor `switchTo*Profile` funcs**

In `internal/ui/manager/manager.go`, find and DELETE entirely:

- `func (vm *Manager) switchToDevProfile() error { ... }`
- `func (vm *Manager) switchToLocalProfile() error { ... }`
- `func (vm *Manager) switchToProdProfile() error { ... }`
- `func (vm *Manager) switchToStandardProfile(profile string) { ... }`
- `func (vm *Manager) switchToDragosProfile() { ... }`

Confirm none of them are referenced anywhere else in the package:

Run: `grep -n 'switchToDevProfile\|switchToLocalProfile\|switchToProdProfile\|switchToStandardProfile\|switchToDragosProfile' internal/ui/manager/manager.go`

Expected: no matches after deletion.

- [ ] **Step 5: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds. If anything else in the codebase referenced one of the deleted funcs, the compiler will say so — those callers should also be deleted (or ported to switchToEnvironment).

- [ ] **Step 6: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/components/profile/profile.go internal/ui/manager/manager.go
git commit -m "Picker dispatches via Resolver+switchToEnvironment; delete vendor switch funcs"
```

---

### Task 5: `view.Reinitialize` reads from `session.Environment`

**Files:**
- Modify: `internal/ui/views/elastic/view.go`

- [ ] **Step 1: Inspect the current `Reinitialize`**

Run: `grep -n 'func.*Reinitialize' internal/ui/views/elastic/view.go`

Read the function body — it currently checks `session.Dragos != nil` to choose between `service.ReinitializeDragos(session.Dragos)` and `service.Reinitialize(cfg, manager.CurrentProfile())`.

- [ ] **Step 2: Replace `Reinitialize`**

Replace the function with:

```go
func (v *View) Reinitialize(cfg aws.Config) error {
	session := v.manager.CurrentSession()
	if session == nil {
		return fmt.Errorf("no active session")
	}

	if err := v.service.ReinitializeFromEnv(session.Environment, session.Config, session.Token); err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error reinitializing ES service: %v", err))
		return err
	}

	v.state.mu.Lock()
	if pattern := session.Environment.IndexPattern; pattern != "" {
		v.state.search.currentIndex = pattern
	}
	if session.Environment.Auth.Type == "none" {
		v.state.search.timeframe = ""
	}
	v.state.data.fieldCache = NewFieldCache()
	v.state.data.fieldState = NewFieldState(v.state.data.fieldCache)
	v.state.mu.Unlock()

	v.manager.App().QueueUpdateDraw(func() {
		if session.Environment.Auth.Type == "none" {
			v.components.timeframeInput.SetText("")
		}
		if pattern := session.Environment.IndexPattern; pattern != "" && v.components.indexInput != nil {
			v.components.indexInput.SetText(pattern)
		}
	})

	if err := v.loadFields(); err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error loading fields: %v", err))
		return err
	}

	v.manager.App().QueueUpdateDraw(func() {
		v.rebuildFieldList()
	})

	v.refreshResults()
	return nil
}
```

The change versus today:

- The if/else on `session.Dragos != nil` is gone. One unified path through `ReinitializeFromEnv`.
- The local-region detection moves from `cfg.Region == "local"` to `session.Environment.Auth.Type == "none"`. Equivalent for the local environment, more semantic.
- The Dragos-specific index-pattern reset (`session.Dragos.IndexPattern`) becomes a generic check on `session.Environment.IndexPattern` — applies to any environment that defines one.

- [ ] **Step 3: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds.

- [ ] **Step 4: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/views/elastic/view.go
git commit -m "view.Reinitialize reads session.Environment instead of session.Dragos"
```

---

### Task 6: Lazy elastic view uses `NewServiceFromEnv`

**Files:**
- Modify: `cmd/cloudcutter/main.go`

- [ ] **Step 1: Inspect the current lazy registration**

Run: `grep -n 'RegisterLazyView' cmd/cloudcutter/main.go`

Note: there are two registrations — `ViewDynamoDB` (unchanged in phase 4) and `ViewElastic` (which currently branches on `session.Dragos != nil`).

- [ ] **Step 2: Rewrite the `ViewElastic` lazy registration**

In `cmd/cloudcutter/main.go`, replace the `viewManager.RegisterLazyView(manager.ViewElastic, ...)` block:

```go
	viewManager.RegisterLazyView(manager.ViewElastic, func() (views.View, error) {
		session := viewManager.CurrentSession()
		if session == nil {
			return nil, fmt.Errorf("no active session for elastic view")
		}

		svc, err := elastic.NewServiceFromEnv(session.Environment, session.Config, session.Token)
		if err != nil {
			return nil, fmt.Errorf("failed to create elastic service: %w", err)
		}
		services.Elastic = svc

		defaultIndex := session.Environment.IndexPattern
		if defaultIndex == "" {
			defaultIndex = "main-summary-*"
		}

		elasticViewInstance, err := elasticView.NewView(viewManager, services.Elastic, defaultIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to create elastic view: %w", err)
		}
		return elasticViewInstance, nil
	})
```

The key changes versus today:

- No `session.Dragos != nil` branch.
- Uses `elastic.NewServiceFromEnv` directly instead of `services.InitializeElastic` / `services.InitializeElasticDragos`.
- `defaultIndex` comes from `session.Environment.IndexPattern` (falling back to the legacy "main-summary-*" if the env didn't define one — this preserves today's default for AWS profiles whose YAML entry omits `index_pattern`).
- `services.Elastic` is set directly so the existing `services.Services` struct still has the field populated for any other code that reads it.

- [ ] **Step 3: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds. If the compiler reports unused imports (`viewManager.GetCurrentConfig` was the only caller of `aws.Config` at this layer), prune them.

- [ ] **Step 4: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 5: Manual smoke test — first switch end-to-end**

This is the moment of truth — phase 4 is supposed to make the running app YAML-driven. Verify on your machine:

```bash
go build -mod=vendor -o /tmp/cloudcutter-phase4 ./cmd/cloudcutter
/tmp/cloudcutter-phase4
```

Expected:
- The picker shows entries from your `~/.cloudcutter/config.yaml` plus AWS profiles from `~/.aws/credentials`. For your machine, that's `dragos`, `local`, plus any AWS profiles.
- Selecting `dragos` triggers the same login-modal-on-failure / probe / reinitialize flow as today, but routed through `switchToEnvironment` instead of `switchToDragosProfile`.
- Selecting `local` works.
- Header shows `Profile: dragos`, `Region: —` for non-AWS environments.
- The elastic view loads and queries work.
- DynamoDB view continues to work for AWS profiles.

If anything is broken, capture the error from the status bar and the relevant log file under `./logs/`. Report it as a bug to fix before proceeding.

If everything works:

- [ ] **Step 6: Commit**

```bash
git add cmd/cloudcutter/main.go
git commit -m "Lazy elastic view uses NewServiceFromEnv from session.Environment"
```

---

### Task 7: Phase 4 verification + tag

**Files:** none modified.

- [ ] **Step 1: Confirm the 5 vendor switch funcs are gone**

Run: `grep -rn 'switchToDevProfile\|switchToLocalProfile\|switchToProdProfile\|switchToStandardProfile\|switchToDragosProfile' --include='*.go' .`

Expected: no output.

- [ ] **Step 2: Confirm phase-4 doesn't reach into legacy auth/services entry points anymore**

Run: `grep -rn 'auth.SwitchProfile\b' --include='*.go' . | grep -v internal/auth/`

Expected: no output. `auth.SwitchProfile` is dead code outside `internal/auth/` itself (where it remains until phase 5).

Run: `grep -rn 'services.InitializeElastic\|services.InitializeElasticDragos' --include='*.go' .`

Expected: only definitions in `internal/services/services.go`. No callers outside that file.

Run: `grep -rn 'session.Dragos' --include='*.go' .`

Expected: only references inside `internal/auth/auth.go` (the legacy population branch in `SwitchEnvironment`'s jwt case) and `internal/services/elastic/dragos.go`'s legacy `legacyDragosEnvFromSession` (also dead). No callers in views/manager/cmd.

- [ ] **Step 3: Confirm the new entry points have callers**

Run: `grep -rn 'NewServiceFromEnv\b' --include='*.go' . | grep -v _test.go`

Expected: at least `cmd/cloudcutter/main.go` and the legacy wrappers in `internal/services/elastic/elastic.go` + `internal/services/elastic/dragos.go`.

Run: `grep -rn 'switchToEnvironment\|SwitchEnvironment\b' --include='*.go' . | grep -v _test.go`

Expected: at least `internal/ui/manager/manager.go`, `internal/ui/components/profile/handler_env.go`, `internal/auth/auth.go`.

- [ ] **Step 4: Full test sweep**

Run: `go test -mod=vendor ./...`

Expected: every package PASSes.

- [ ] **Step 5: `go vet` + gofmt**

Run: `go vet -mod=vendor ./...`

Expected: no output.

Run: `gofmt -l internal/ui/manager internal/ui/components/profile internal/ui/views/elastic cmd/cloudcutter`

Expected: no output.

- [ ] **Step 6: Tag**

```bash
git tag phase-4-complete
```

- [ ] **Step 7: Update spec status**

Edit `docs/superpowers/specs/2026-05-09-generic-backend-design.md`:

From:

```
**Status**: Phase 3 implemented (tag `phase-3-complete`). Phase 4 (manager + view cleanup) plan to be drafted next.
```

To:

```
**Status**: Phase 4 implemented (tag `phase-4-complete`). Phase 5 (delete legacy) plan to be drafted next.
```

```bash
git add docs/superpowers/specs/2026-05-09-generic-backend-design.md
git commit -m "Note phase 4 complete in spec status"
```

---

## What's NOT in phase 4 (deferred to phase 5)

- **Delete legacy auth surface**: `auth.SwitchProfile`, `auth.legacyToEnvironment`, `auth.LoadDragosConfig`, `auth.SaveDragosToken`, `auth.DragosConfigPath`, `auth.OpalConfig`, `auth.LoadOpalConfig`, `auth.DragosSession` (and the `Session.Dragos` field), the kooky import + `loadDragosCookieFromBrowser`, `auth.DefaultDragos*` constants.
- **Delete legacy elastic surface**: `elastic.NewService(cfg)`, `elastic.NewDragosService(d)`, `elastic.Reinitialize(cfg, profile)`, `elastic.ReinitializeDragos(d)`, `elastic.legacySophosEnvFromAWSConfig`, `elastic.legacyDragosEnvFromSession`.
- **Delete legacy services surface**: `services.InitializeElastic`, `services.InitializeElasticDragos`.
- **Delete legacy profile picker surface**: `profile.Handler.SwitchProfile`, `profile.Selector.discoverProfiles`, `profile.Selector.switchProfile`, the `auth` import in `profile.go`.

After phase 5: zero references to vendor names ("dragos", "darkbytes", "Sophos") in Go source (the `~/.cloudcutter/dragos.token` filename and the `cmd/dragos-cookie-probe/` diagnostic CLI are user-facing artifacts — kept).

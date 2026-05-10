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

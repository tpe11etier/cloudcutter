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

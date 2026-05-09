package elastic

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/tpe11etier/cloudcutter/internal/config"
)

func TestKibanaProxyTransportRewritesPathAndMethod(t *testing.T) {
	var gotPath string
	var gotMethod string
	var gotPathQuery string
	var gotMethodQuery string
	var gotCookie string
	var gotXSRF string
	var gotKbnVersion string
	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		q, _ := url.ParseQuery(r.URL.RawQuery)
		gotPathQuery = q.Get("path")
		gotMethodQuery = q.Get("method")
		gotCookie = r.Header.Get("Cookie")
		gotXSRF = r.Header.Get("kbn-xsrf")
		gotKbnVersion = r.Header.Get("kbn-version")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{}}`))
	}))
	defer srv.Close()

	spec := config.TransportSpec{
		Type:      "kibana_proxy",
		BaseURL:   srv.URL,
		ProxyPath: "/kibana/api/console/proxy",
		TokenHeader: &config.TokenHeaderSpec{
			Name:   "Cookie",
			Format: "auth-token={token}",
		},
		Headers: map[string]string{
			"kbn-xsrf":    "cloudcutter",
			"kbn-version": "8.19.2",
		},
	}
	tr := newKibanaProxyTransport(spec, "the-jwt", nil)

	// Caller does GET /events*/_search?pretty=true with a JSON body.
	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		srv.URL+"/events%2A/_search?pretty=true", strings.NewReader(`{"q":"x"}`))
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if gotPath != "/kibana/api/console/proxy" {
		t.Errorf("server got path %q, want /kibana/api/console/proxy", gotPath)
	}
	if gotMethod != "POST" {
		t.Errorf("server got method %q, want POST", gotMethod)
	}
	// path query carries the original ES path + query
	if !strings.Contains(gotPathQuery, "_search") {
		t.Errorf("path= query should contain _search, got %q", gotPathQuery)
	}
	if !strings.Contains(gotPathQuery, "pretty=true") {
		t.Errorf("path= query should preserve original querystring, got %q", gotPathQuery)
	}
	if gotMethodQuery != "GET" {
		t.Errorf("method= query = %q, want GET (original method)", gotMethodQuery)
	}
	if gotCookie != "auth-token=the-jwt" {
		t.Errorf("Cookie header = %q, want auth-token=the-jwt", gotCookie)
	}
	if gotXSRF != "cloudcutter" {
		t.Errorf("kbn-xsrf = %q", gotXSRF)
	}
	if gotKbnVersion != "8.19.2" {
		t.Errorf("kbn-version = %q", gotKbnVersion)
	}
	if gotBody != `{"q":"x"}` {
		t.Errorf("body = %q, want %q", gotBody, `{"q":"x"}`)
	}
}

func TestKibanaProxyTransportRetriesOn401WithFreshToken(t *testing.T) {
	var calls int
	var lastCookie string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		lastCookie = r.Header.Get("Cookie")
		mu.Unlock()
		if calls == 1 {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	refreshCalls := 0
	refresh := func() (string, error) {
		refreshCalls++
		return "fresh-jwt", nil
	}

	spec := config.TransportSpec{
		Type:      "kibana_proxy",
		BaseURL:   srv.URL,
		ProxyPath: "/p",
		TokenHeader: &config.TokenHeaderSpec{Name: "Cookie", Format: "x={token}"},
	}
	tr := newKibanaProxyTransport(spec, "stale-jwt", refresh)

	req, _ := http.NewRequest("GET", srv.URL+"/_search", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("final status = %d, want 200", resp.StatusCode)
	}
	if calls != 2 {
		t.Errorf("server saw %d calls, want 2 (initial + retry)", calls)
	}
	if refreshCalls != 1 {
		t.Errorf("refresh called %d times, want 1", refreshCalls)
	}
	if lastCookie != "x=fresh-jwt" {
		t.Errorf("retry cookie = %q, want x=fresh-jwt", lastCookie)
	}
}

func TestKibanaProxyTransportNoRetryWhenRefreshReturnsSameToken(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(401)
	}))
	defer srv.Close()

	refresh := func() (string, error) {
		return "stale-jwt", nil // same as initial
	}

	spec := config.TransportSpec{
		Type:      "kibana_proxy",
		BaseURL:   srv.URL,
		ProxyPath: "/p",
		TokenHeader: &config.TokenHeaderSpec{Name: "Cookie", Format: "x={token}"},
	}
	tr := newKibanaProxyTransport(spec, "stale-jwt", refresh)

	req, _ := http.NewRequest("GET", srv.URL+"/_search", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if calls != 1 {
		t.Errorf("server saw %d calls, want 1 (no retry when refresh is no-op)", calls)
	}
}

func TestKibanaProxyTransportNoRefreshConfigured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	spec := config.TransportSpec{
		Type:      "kibana_proxy",
		BaseURL:   srv.URL,
		ProxyPath: "/p",
		TokenHeader: &config.TokenHeaderSpec{Name: "Cookie", Format: "x={token}"},
	}
	tr := newKibanaProxyTransport(spec, "stale-jwt", nil)

	req, _ := http.NewRequest("GET", srv.URL+"/_search", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Errorf("expected 401 to be returned to caller when no refresh, got %d", resp.StatusCode)
	}
}

func TestKibanaProxyTransportContentTypeDefaultsToJSON(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	spec := config.TransportSpec{
		Type:      "kibana_proxy",
		BaseURL:   srv.URL,
		ProxyPath: "/p",
		TokenHeader: &config.TokenHeaderSpec{Name: "Cookie", Format: "x={token}"},
		// No "Content-Type" key in Headers — transport should default.
	}
	tr := newKibanaProxyTransport(spec, "tok", nil)
	req, _ := http.NewRequest("GET", srv.URL+"/_search", nil)
	resp, _ := tr.RoundTrip(req)
	defer resp.Body.Close()

	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
}

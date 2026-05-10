# Generic Backend — Phase 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the vendor-named `awsTransport` (Sophos darkbytes SigV4) and `dragosTransport` (Kibana proxy + cookie) with two generic transports parameterized by `config.TransportSpec`. Add a new unified `elastic.Service` constructor that takes `environments.Environment` instead of `aws.Config` + `DragosSession`. Behavior is **unchanged from the user's POV** — the manager and views still call the legacy entry points; those entry points become thin wrappers around the new generic path.

**Architecture:** Two new generic transport types — `sigv4Transport` (parameterized by `aws.Config` + `service` from spec) and `kibanaProxyTransport` (parameterized by `BaseURL`, `ProxyPath`, `TokenHeader`, `Headers`, `Probe`, plus a `tokenRefresh` callback for the 401-retry). The new `NewServiceFromEnv(env, awsCfg, token)` constructor dispatches transport choice from `env.Transport.Type`. The four existing legacy entry points (`NewService`, `Reinitialize`, `NewDragosService`, `ReinitializeDragos`) become thin wrappers that build a legacy Environment internally and delegate. Phase 4 deletes those wrappers when the manager calls `NewServiceFromEnv` directly.

**Tech Stack:** Go, `net/http`, `httptest`, AWS SDK v4 signer, `github.com/elastic/go-elasticsearch/v6`, existing `internal/probe` and `internal/environments` packages.

**Spec:** `docs/superpowers/specs/2026-05-09-generic-backend-design.md` — particularly the "Phase 3" row of the Phasing table, the `internal/services/elastic` lines in "What collapses" and "Deletions", and the `NewService(env, awsCfg, token)` signature in "Go types & key interfaces".

**Status as of plan-write time:** phase 1 (config + environments + init) and phase 2 (auth refactor + probe consolidation) are complete (tags `phase-1-complete`, `phase-2-complete`). Phase 3 builds on `internal/probe.Run`, `environments.Environment`, and `config.TransportSpec`.

---

## Out of scope (deferred to phase 4)

- The manager's `switchTo*Profile` funcs and views are not touched. They keep calling `services.InitializeElastic(cfg)`, `services.InitializeElasticDragos(d)`, `view.Reinitialize(cfg)`, etc.
- The four legacy entry points (`NewService`, `Reinitialize`, `NewDragosService`, `ReinitializeDragos`) survive phase 3 as thin wrappers around `NewServiceFromEnv` / `ReinitializeFromEnv`.
- `services.InitializeElasticDragos` survives phase 3 (it now wraps `NewServiceFromEnv`). Phase 4 deletes it.
- The hardcoded Sophos URL template (`https://{prefix}-{region}-primary-es.darkbytes.io`) survives in the **legacy elastic.go wrapper** that builds a default Environment for AWS-profile callers. The vendor-named string moves out of the transport implementation but stays in the legacy bridge until phase 4.

The end-of-phase-3 binary is byte-for-byte equivalent in user-visible behavior, only the transport internals are now generic.

---

## File structure

**New files**

| File | Responsibility |
|---|---|
| `internal/services/elastic/sigv4_transport.go` | Generic SigV4-signing `http.RoundTripper`. Replaces inlined `awsTransport.RoundTrip`. Parameterized by `aws.Config` (provides region + creds) and `service` (e.g., "es"). |
| `internal/services/elastic/sigv4_transport_test.go` | httptest-driven tests verifying the transport signs requests and forwards body bytes correctly. |
| `internal/services/elastic/kibana_proxy_transport.go` | Generic Kibana-proxy `http.RoundTripper`. Replaces `dragosTransport`. Parameterized by `BaseURL`, `ProxyPath`, `TokenHeader`, static `Headers`, optional `tokenRefresh` callback for the 401-retry. |
| `internal/services/elastic/kibana_proxy_transport_test.go` | httptest-driven tests verifying path/method tunneling, token header attachment, 401-refresh-and-retry, and HTML-on-401 detection. |
| `internal/services/elastic/from_env.go` | `NewServiceFromEnv(env, awsCfg, token) (*Service, error)` and `(*Service).ReinitializeFromEnv(env, awsCfg, token) error`. Dispatches transport selection from `env.Transport.Type`. |
| `internal/services/elastic/from_env_test.go` | Coverage of the dispatch: each transport type produces the expected RoundTripper; bad type errors. |

**Modified files**

| File | Change |
|---|---|
| `internal/services/elastic/elastic.go` | `awsTransport` and its method definitions DELETED (replaced by `sigv4Transport`). Legacy `NewService(cfg)` and `Reinitialize(cfg, profile)` rewritten as thin wrappers that build a "default Sophos Environment" with the darkbytes URL template and call `NewServiceFromEnv` / `ReinitializeFromEnv`. |
| `internal/services/elastic/dragos.go` | `dragosTransport` and its method definitions DELETED (replaced by `kibanaProxyTransport`). Legacy `NewDragosService(d)` and `ReinitializeDragos(d)` rewritten as thin wrappers that build an Environment from the DragosSession and call `NewServiceFromEnv` / `ReinitializeFromEnv`. `probeDragosProxy` DELETED (the body was already moved to `probe.Run` in phase 2; the wrapper is now unused once `NewServiceFromEnv` calls `probe.Run` directly). |
| `internal/services/services.go` | `InitializeElastic(cfg)` and `InitializeElasticDragos(d)` continue to exist with unchanged signatures; their bodies still call the legacy `elastic.NewService` / `elastic.NewDragosService` (which themselves delegate). No surface-level change here. |

**No changes** to: `cmd/cloudcutter/*`, `internal/auth/*`, `internal/ui/*`, `internal/config/*`, `internal/environments/*`, `internal/probe/*`. Phase 3 is service-internals only.

---

## Tasks

### Task 1: `sigv4_transport.go` — generic SigV4 RoundTripper

**Files:**
- Create: `internal/services/elastic/sigv4_transport.go`
- Test: `internal/services/elastic/sigv4_transport_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/services/elastic/sigv4_transport_test.go`:

```go
package elastic

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func staticAWSConfig(region string) aws.Config {
	return aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider("AKIATEST", "secret-test", ""),
	}
}

func TestSigV4TransportSetsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	var gotMethod string
	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tr := newSigV4Transport(staticAWSConfig("us-west-2"), "es")
	req, _ := http.NewRequestWithContext(context.Background(), "POST", srv.URL+"/_search", strings.NewReader(`{"q":"x"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 ") {
		t.Errorf("Authorization header = %q, want SigV4 prefix", gotAuth)
	}
	if !strings.Contains(gotAuth, "Credential=AKIATEST/") {
		t.Errorf("Authorization should embed AKIATEST credential, got %q", gotAuth)
	}
	if !strings.Contains(gotAuth, "/us-west-2/es/") {
		t.Errorf("Authorization should embed region/service, got %q", gotAuth)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
	if gotBody != `{"q":"x"}` {
		t.Errorf("body = %q, want %q", gotBody, `{"q":"x"}`)
	}
}

func TestSigV4TransportPropagatesNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	tr := newSigV4Transport(staticAWSConfig("us-west-2"), "es")
	req, _ := http.NewRequest("GET", srv.URL, nil)

	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatal("expected network error")
	}
}

func TestSigV4TransportSetsXAmzContentSha256(t *testing.T) {
	var gotSha string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSha = r.Header.Get("X-Amz-Content-Sha256")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tr := newSigV4Transport(staticAWSConfig("us-west-2"), "es")
	// Empty body → known sha256 of empty.
	req, _ := http.NewRequest("GET", srv.URL+"/_cluster/health", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	emptySha := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if gotSha != emptySha {
		t.Errorf("X-Amz-Content-Sha256 = %q, want %q", gotSha, emptySha)
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `go test -mod=vendor ./internal/services/elastic -run TestSigV4Transport`

Expected: FAIL — `undefined: newSigV4Transport`.

- [ ] **Step 3: Implement `sigv4Transport`**

Write `internal/services/elastic/sigv4_transport.go`:

```go
package elastic

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// sigv4Transport signs every outgoing request with AWS SigV4 using the
// region from the supplied aws.Config and the AWS service name from the
// caller-supplied spec ("es" for Elasticsearch on AWS, but parameterized
// for forward-compat). Replaces the vendor-named awsTransport whose
// service was hardcoded to "es" and whose region was duplicated as a
// struct field rather than read from cfg.Region.
type sigv4Transport struct {
	client  *http.Client
	cfg     aws.Config
	service string
}

func newSigV4Transport(cfg aws.Config, service string) *sigv4Transport {
	return &sigv4Transport{
		client:  &http.Client{},
		cfg:     cfg,
		service: service,
	}
}

func (t *sigv4Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	credentials, err := t.cfg.Credentials.Retrieve(req.Context())
	if err != nil {
		return nil, err
	}

	req.Header.Set("Host", req.Host)
	payloadHash := hashPayload(body)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	signer := v4.NewSigner()
	if err := signer.SignHTTP(req.Context(), credentials, req, payloadHash, t.service, t.cfg.Region, time.Now()); err != nil {
		return nil, err
	}

	if body != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	return t.client.Do(req)
}

func hashPayload(b []byte) string {
	if b == nil {
		return "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}
	h := sha256.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
```

Note: `hashPayload` already exists in `elastic.go` (it was used by `awsTransport`). Step 6 of Task 4 will delete the old definition.

- [ ] **Step 4: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/services/elastic -run TestSigV4Transport -v`

Expected: PASS for all three sub-tests.

If you see a duplicate `hashPayload` redeclaration error, that's expected — the old `awsTransport`-companion function still exists in `elastic.go`. Skip to Task 4 step 6, delete the old `hashPayload`, then come back and re-run.

Actually — to avoid that compile error temporarily, **rename** the function in this task to a unique name. Update the new file:

```go
func sha256Hex(b []byte) string {
	if b == nil {
		return "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}
	h := sha256.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
```

…and update the `RoundTrip` body to call `sha256Hex` instead of `hashPayload`. Task 4 will then delete the old `hashPayload` from `elastic.go`. The renamed function survives.

- [ ] **Step 5: Commit**

```bash
git add internal/services/elastic/sigv4_transport.go internal/services/elastic/sigv4_transport_test.go
git commit -m "Add generic sigv4Transport parameterized by aws.Config + service"
```

---

### Task 2: `kibana_proxy_transport.go` — generic Kibana-proxy RoundTripper

**Files:**
- Create: `internal/services/elastic/kibana_proxy_transport.go`
- Test: `internal/services/elastic/kibana_proxy_transport_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/services/elastic/kibana_proxy_transport_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `go test -mod=vendor ./internal/services/elastic -run TestKibanaProxyTransport`

Expected: FAIL — `undefined: newKibanaProxyTransport`.

- [ ] **Step 3: Implement `kibanaProxyTransport`**

Write `internal/services/elastic/kibana_proxy_transport.go`:

```go
package elastic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tpe11etier/cloudcutter/internal/config"
)

// kibanaProxyTransport rewrites every request the elasticsearch SDK makes
// so it goes through Kibana's `console/proxy` endpoint instead of hitting
// ES directly. The proxy returns plain ES JSON, so the rest of the SDK
// keeps working unchanged.
//
// Wire shape (with proxyPath=/kibana/api/console/proxy):
//
//	SDK builds:    POST {base}/_search?pretty=true     body=<query>
//	we rewrite to: POST {base}/kibana/api/console/proxy?path=_search?pretty=true&method=POST
//	                    body=<query>
//
// On a 401 response, the transport calls tokenRefresh (if configured) and
// retries the request once with the fresh token. The "stale" check (new
// token equals current token) prevents infinite loops when the source of
// truth hasn't actually changed.
//
// All vendor-specific knobs (proxy path, cookie name + format, kbn-version
// header, kbn-xsrf header, base URL) come from the supplied
// config.TransportSpec — none are baked in.
type kibanaProxyTransport struct {
	client       *http.Client
	baseURL      string
	proxyPath    string
	tokenHeader  *config.TokenHeaderSpec
	headers      map[string]string
	tokenRefresh func() (string, error)

	mu    sync.Mutex
	token string
}

// newKibanaProxyTransport constructs a transport from the spec, an initial
// token, and an optional tokenRefresh callback (pass nil to disable the
// 401-retry).
func newKibanaProxyTransport(spec config.TransportSpec, token string, tokenRefresh func() (string, error)) *kibanaProxyTransport {
	return &kibanaProxyTransport{
		client:       &http.Client{Timeout: 30 * time.Second},
		baseURL:      strings.TrimRight(spec.BaseURL, "/"),
		proxyPath:    spec.ProxyPath,
		tokenHeader:  spec.TokenHeader,
		headers:      spec.Headers,
		tokenRefresh: tokenRefresh,
		token:        token,
	}
}

func (t *kibanaProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	originalPath := strings.TrimPrefix(req.URL.Path, "/")
	originalMethod := req.Method
	originalQuery := req.URL.RawQuery

	pathParam := originalPath
	if originalQuery != "" {
		pathParam = originalPath + "?" + originalQuery
	}

	proxyURL, err := url.Parse(t.baseURL + t.proxyPath)
	if err != nil {
		return nil, fmt.Errorf("invalid kibana proxy base URL: %w", err)
	}
	q := url.Values{}
	q.Set("path", pathParam)
	q.Set("method", originalMethod)
	proxyURL.RawQuery = q.Encode()

	// Buffer the body once so we can replay it on retry after a 401.
	var bodyBytes []byte
	if req.Body != nil {
		buf, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
		bodyBytes = buf
	}

	resp, err := t.do(req.Context(), proxyURL.String(), bodyBytes)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized || t.tokenRefresh == nil {
		return resp, nil
	}

	newToken, refreshErr := t.tokenRefresh()
	if refreshErr != nil || newToken == "" {
		return resp, nil
	}
	t.mu.Lock()
	stale := t.token == newToken
	if !stale {
		t.token = newToken
	}
	t.mu.Unlock()
	if stale {
		return resp, nil
	}

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return t.do(req.Context(), proxyURL.String(), bodyBytes)
}

func (t *kibanaProxyTransport) do(ctx context.Context, proxyURL string, bodyBytes []byte) (*http.Response, error) {
	var body io.Reader
	if len(bodyBytes) > 0 {
		body = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, proxyURL, body)
	if err != nil {
		return nil, err
	}
	t.applyHeaders(req)
	return t.client.Do(req)
}

func (t *kibanaProxyTransport) applyHeaders(req *http.Request) {
	t.mu.Lock()
	token := t.token
	t.mu.Unlock()
	if t.tokenHeader != nil {
		val := strings.ReplaceAll(t.tokenHeader.Format, "{token}", token)
		req.Header.Set(t.tokenHeader.Name, val)
	}
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
}
```

- [ ] **Step 4: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/services/elastic -run TestKibanaProxyTransport -v`

Expected: PASS for all 5 sub-tests.

- [ ] **Step 5: Commit**

```bash
git add internal/services/elastic/kibana_proxy_transport.go internal/services/elastic/kibana_proxy_transport_test.go
git commit -m "Add generic kibanaProxyTransport parameterized by config.TransportSpec"
```

---

### Task 3: `from_env.go` — unified Service constructor

**Files:**
- Create: `internal/services/elastic/from_env.go`
- Test: `internal/services/elastic/from_env_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/services/elastic/from_env_test.go`:

```go
package elastic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpe11etier/cloudcutter/internal/config"
	"github.com/tpe11etier/cloudcutter/internal/environments"
)

func TestNewServiceFromEnvPlain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	env := environments.Environment{
		Name: "local",
		Auth: config.AuthSpec{Type: "none"},
		Transport: config.TransportSpec{
			Type:    "plain",
			BaseURL: srv.URL,
		},
	}
	svc, err := NewServiceFromEnv(env, aws.Config{}, "")
	if err != nil {
		t.Fatalf("NewServiceFromEnv: %v", err)
	}
	if svc.Client == nil {
		t.Error("Service.Client is nil")
	}
}

func TestNewServiceFromEnvSigV4(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	env := environments.Environment{
		Name:   "opal_dev",
		Region: "us-west-2",
		Auth:   config.AuthSpec{Type: "aws_sdk"},
		Transport: config.TransportSpec{
			Type:        "sigv4",
			Service:     "es",
			URLTemplate: srv.URL, // already substituted
		},
	}
	svc, err := NewServiceFromEnv(env, staticAWSConfig("us-west-2"), "")
	if err != nil {
		t.Fatalf("NewServiceFromEnv: %v", err)
	}
	// Touch the client to force a request through the sigv4 transport.
	res, err := svc.Client.Cat.Indices(svc.Client.Cat.Indices.WithContext(context.Background()))
	if err != nil {
		t.Fatalf("Cat.Indices: %v", err)
	}
	defer res.Body.Close()
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 ") {
		t.Errorf("Authorization = %q, want SigV4 prefix", gotAuth)
	}
}

func TestNewServiceFromEnvKibanaProxy(t *testing.T) {
	var gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	env := environments.Environment{
		Name: "dragos",
		Auth: config.AuthSpec{Type: "jwt"},
		Transport: config.TransportSpec{
			Type:      "kibana_proxy",
			BaseURL:   srv.URL,
			ProxyPath: "/kibana/api/console/proxy",
			TokenHeader: &config.TokenHeaderSpec{
				Name:   "Cookie",
				Format: "auth-token={token}",
			},
		},
	}
	svc, err := NewServiceFromEnv(env, aws.Config{}, "the-jwt")
	if err != nil {
		t.Fatalf("NewServiceFromEnv: %v", err)
	}
	res, err := svc.Client.Cat.Indices(svc.Client.Cat.Indices.WithContext(context.Background()))
	if err != nil {
		t.Fatalf("Cat.Indices: %v", err)
	}
	defer res.Body.Close()
	if gotCookie != "auth-token=the-jwt" {
		t.Errorf("Cookie = %q, want auth-token=the-jwt", gotCookie)
	}
}

func TestNewServiceFromEnvUnknownTransportType(t *testing.T) {
	env := environments.Environment{
		Name: "x",
		Transport: config.TransportSpec{
			Type:    "exotic",
			BaseURL: "https://example.com",
		},
	}
	_, err := NewServiceFromEnv(env, aws.Config{}, "")
	if err == nil {
		t.Fatal("expected error for unknown transport type")
	}
	if !strings.Contains(err.Error(), "exotic") {
		t.Errorf("error should mention bad type, got %q", err.Error())
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `go test -mod=vendor ./internal/services/elastic -run TestNewServiceFromEnv`

Expected: FAIL — `undefined: NewServiceFromEnv`.

- [ ] **Step 3: Implement `NewServiceFromEnv` and `ReinitializeFromEnv`**

Write `internal/services/elastic/from_env.go`:

```go
package elastic

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/elastic/go-elasticsearch/v6"
	"github.com/spf13/viper"
	"github.com/tpe11etier/cloudcutter/internal/environments"
	"github.com/tpe11etier/cloudcutter/internal/logger"
)

// NewServiceFromEnv constructs an elastic Service whose transport is
// chosen from env.Transport.Type. The caller supplies the AWS config (for
// sigv4) and JWT token (for kibana_proxy); both are ignored when not
// applicable to the chosen transport.
//
// This is the unified entry point introduced in phase 3. Phase 4 makes
// the manager call this directly; until then, the legacy elastic.NewService
// and elastic.NewDragosService delegate here.
func NewServiceFromEnv(env environments.Environment, awsCfg aws.Config, token string) (*Service, error) {
	l, err := newServiceLogger("es_svc")
	if err != nil {
		return nil, err
	}

	transport, addr, err := buildTransport(env, awsCfg, token, l)
	if err != nil {
		return nil, err
	}

	esCfg := elasticsearch.Config{
		Addresses:     []string{addr},
		EnableMetrics: true,
	}
	if transport != nil {
		esCfg.Transport = transport
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	s := &Service{
		Client: client,
		log:    l,
		cache:  make(map[string]*IndexStats),
		mu:     sync.RWMutex{},
	}

	if client != nil {
		if err := s.PreloadIndexStats(context.Background()); err != nil {
			l.Warn("Initial cache preload failed: %v", err)
		}
	}

	return s, nil
}

// ReinitializeFromEnv rebuilds the underlying Client+Transport from the
// new Environment. Used when the user changes profile or region in-app.
func (s *Service) ReinitializeFromEnv(env environments.Environment, awsCfg aws.Config, token string) error {
	transport, addr, err := buildTransport(env, awsCfg, token, s.log)
	if err != nil {
		return err
	}

	esCfg := elasticsearch.Config{
		Addresses:     []string{addr},
		EnableMetrics: true,
	}
	if transport != nil {
		esCfg.Transport = transport
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return fmt.Errorf("failed to reinitialize elasticsearch client: %w", err)
	}

	s.mu.Lock()
	s.Client = client
	s.cache = make(map[string]*IndexStats)
	s.mu.Unlock()
	return nil
}

// buildTransport dispatches transport construction from env.Transport.Type
// and returns the transport plus the URL the SDK should target. A nil
// transport means "use http.DefaultTransport" (i.e., plain HTTP).
func buildTransport(env environments.Environment, awsCfg aws.Config, token string, l *logger.Logger) (http.RoundTripper, string, error) {
	tspec := env.Transport
	switch tspec.Type {
	case "plain":
		if tspec.BaseURL == "" {
			return nil, "", fmt.Errorf("transport=plain requires base_url")
		}
		return nil, tspec.BaseURL, nil

	case "sigv4":
		if tspec.URLTemplate == "" {
			return nil, "", fmt.Errorf("transport=sigv4 requires url_template")
		}
		service := tspec.Service
		if service == "" {
			service = "es"
		}
		return newSigV4Transport(awsCfg, service), tspec.URLTemplate, nil

	case "kibana_proxy":
		if tspec.BaseURL == "" {
			return nil, "", fmt.Errorf("transport=kibana_proxy requires base_url")
		}
		if tspec.ProxyPath == "" {
			return nil, "", fmt.Errorf("transport=kibana_proxy requires proxy_path")
		}
		return newKibanaProxyTransport(tspec, token, nil), strings.TrimRight(tspec.BaseURL, "/"), nil

	default:
		return nil, "", fmt.Errorf("unknown transport type %q (want plain|sigv4|kibana_proxy)", tspec.Type)
	}
}

// newServiceLogger builds the per-Service logger using viper config. The
// prefix is part of the log filename so the Sophos and Dragos services
// each get distinct log streams when both are around.
func newServiceLogger(prefix string) (*logger.Logger, error) {
	logDir := viper.GetString("log_dir")
	if logDir == "" {
		logDir = "./logs"
	}
	level, err := logger.ParseLevel(strings.ToLower(viper.GetString("logging")))
	if err != nil {
		level = logger.INFO
	}
	l, err := logger.New(logger.Config{
		LogDir: logDir,
		Prefix: prefix,
		Level:  level,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	return l, nil
}
```

- [ ] **Step 4: Run the tests to confirm they pass**

Run: `go test -mod=vendor ./internal/services/elastic -run TestNewServiceFromEnv -v`

Expected: PASS for all 4 sub-tests.

- [ ] **Step 5: Commit**

```bash
git add internal/services/elastic/from_env.go internal/services/elastic/from_env_test.go
git commit -m "Add NewServiceFromEnv with transport dispatch on env.Transport.Type"
```

---

### Task 4: Refactor legacy `elastic.go` entry points to delegate

**Files:**
- Modify: `internal/services/elastic/elastic.go`

- [ ] **Step 1: Read the current state**

Run: `wc -l internal/services/elastic/elastic.go && grep -n 'func ' internal/services/elastic/elastic.go`

Note: there's `NewService(cfg)`, `Reinitialize(cfg, profile)`, `awsTransport`, `awsTransport.RoundTrip`, `hashPayload`. Plus utility functions (`parseSize`, `formatSize`, `Close`, `PreloadIndexStats`, `ListIndices`).

- [ ] **Step 2: Replace `NewService` and `Reinitialize` bodies**

In `internal/services/elastic/elastic.go`, replace the existing `NewService` and `Reinitialize` functions with:

```go
// NewService is the legacy entry point: callers pass an aws.Config
// (region pulled from there) and the Service is built against the Sophos
// darkbytes URL template. Phase 3 keeps this working by translating the
// AWS config into an Environment internally and delegating to
// NewServiceFromEnv. Phase 4 deletes this once the manager constructs
// Environment values from the Resolver and calls NewServiceFromEnv
// directly.
func NewService(cfg aws.Config) (*Service, error) {
	return NewServiceFromEnv(legacySophosEnvFromAWSConfig(cfg, "default"), cfg, "")
}

// Reinitialize is the legacy reinit entry point. profile is used only to
// pick the URL prefix ("dev" vs "prod") in the legacy darkbytes template.
func (s *Service) Reinitialize(cfg aws.Config, profile string) error {
	return s.ReinitializeFromEnv(legacySophosEnvFromAWSConfig(cfg, profile), cfg, "")
}

// legacySophosEnvFromAWSConfig builds a phase-3 bridge Environment that
// reproduces the URL the legacy awsTransport hit: localhost:9200 for
// region=local, otherwise https://{prefix}-{region}-primary-es.darkbytes.io
// where prefix is "prod" for opal_prod and "dev" otherwise.
//
// The vendor-named darkbytes URL is the only company-specific knob still
// present in the running code path after phase 3; phase 4 removes it
// when the manager supplies Environment values from the YAML resolver.
func legacySophosEnvFromAWSConfig(cfg aws.Config, profile string) environments.Environment {
	if cfg.Region == "local" {
		return environments.Environment{
			Name: "local",
			Auth: config.AuthSpec{Type: "none"},
			Transport: config.TransportSpec{
				Type:    "plain",
				BaseURL: "http://localhost:9200",
			},
		}
	}
	prefix := "dev"
	if profile == "opal_prod" {
		prefix = "prod"
	}
	return environments.Environment{
		Name:   profile,
		Region: cfg.Region,
		Auth:   config.AuthSpec{Type: "aws_sdk"},
		Transport: config.TransportSpec{
			Type:        "sigv4",
			Service:     "es",
			URLTemplate: fmt.Sprintf("https://%s-%s-primary-es.darkbytes.io", prefix, cfg.Region),
		},
	}
}
```

- [ ] **Step 3: Delete the now-orphaned `awsTransport` and its methods**

Find and delete from `elastic.go`:

- The `type awsTransport struct {...}` definition.
- `func (t *awsTransport) RoundTrip(req *http.Request) (*http.Response, error) {...}`.
- `func hashPayload(b []byte) string {...}`.

The renamed `sha256Hex` in `sigv4_transport.go` (Task 1) takes over `hashPayload`'s role.

- [ ] **Step 4: Update imports**

Add to `elastic.go`'s import block:

```go
	"github.com/tpe11etier/cloudcutter/internal/config"
	"github.com/tpe11etier/cloudcutter/internal/environments"
```

Remove now-unused imports. After the deletions in Step 3:

- `bytes`, `crypto/sha256`, `encoding/hex`, `io`, `time` — these were used by `awsTransport.RoundTrip` and `hashPayload`. They are still used by `PreloadIndexStats` (which uses `time` for the context timeout) and `ListIndices` / `PreloadIndexStats` (which use `json.NewDecoder`). Run the build; the compiler will tell you which can be pruned.
- `v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"` — was used only by `awsTransport.RoundTrip`. Remove it.

- [ ] **Step 5: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds. If the compiler complains about unused imports, prune them and retry.

- [ ] **Step 6: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS across all packages.

- [ ] **Step 7: Commit**

```bash
git add internal/services/elastic/elastic.go
git commit -m "Refactor legacy NewService/Reinitialize to delegate via Environment"
```

---

### Task 5: Refactor legacy `dragos.go` entry points to delegate

**Files:**
- Modify: `internal/services/elastic/dragos.go`

- [ ] **Step 1: Read the current state**

Run: `grep -n 'func ' internal/services/elastic/dragos.go`

Should show: `(*dragosTransport).RoundTrip`, `(*dragosTransport).do`, `(*dragosTransport).applyHeaders`, `NewDragosService`, `(*Service).ReinitializeDragos`, `probeDragosProxy`.

- [ ] **Step 2: Replace `NewDragosService` and `ReinitializeDragos` bodies**

Replace both functions with delegating wrappers. In `internal/services/elastic/dragos.go`:

```go
// NewDragosService is the legacy entry point used by services.go's
// InitializeElasticDragos. Phase 3 keeps it working by translating the
// DragosSession into an Environment and delegating to NewServiceFromEnv.
// Phase 4 deletes this once the manager constructs Environment values
// directly.
func NewDragosService(d *auth.DragosSession) (*Service, error) {
	if d == nil {
		return nil, fmt.Errorf("nil DragosSession")
	}
	if d.AuthToken == "" {
		return nil, fmt.Errorf("dragos auth token is empty")
	}
	if d.BaseURL == "" {
		return nil, fmt.Errorf("dragos base URL is empty")
	}

	env := legacyDragosEnvFromSession(d)
	return NewServiceFromEnv(env, aws.Config{}, d.AuthToken)
}

// ReinitializeDragos is the legacy reinit entry point.
func (s *Service) ReinitializeDragos(d *auth.DragosSession) error {
	if d == nil {
		return fmt.Errorf("nil DragosSession")
	}
	env := legacyDragosEnvFromSession(d)
	return s.ReinitializeFromEnv(env, aws.Config{}, d.AuthToken)
}

// legacyDragosEnvFromSession reproduces the phase-2 translator's
// dragosEnvironment shape from a DragosSession (which is what the
// services.InitializeElasticDragos caller has). Kept inline rather than
// imported from internal/auth to avoid the dependency cycle services →
// auth → environments.
func legacyDragosEnvFromSession(d *auth.DragosSession) environments.Environment {
	return environments.Environment{
		Name: auth.DragosProfile,
		Auth: config.AuthSpec{Type: "jwt"},
		Transport: config.TransportSpec{
			Type:      "kibana_proxy",
			BaseURL:   d.BaseURL,
			ProxyPath: "/kibana/api/console/proxy",
			TokenHeader: &config.TokenHeaderSpec{
				Name:   "Cookie",
				Format: "dragos-auth-token={token}",
			},
			Headers: map[string]string{
				"kbn-xsrf":    "cloudcutter",
				"kbn-version": d.KbnVersion,
			},
			Probe: &config.ProbeSpec{
				Path:       "_cluster/health",
				RejectHTML: true,
			},
		},
		IndexPattern: d.IndexPattern,
	}
}
```

- [ ] **Step 3: Delete `dragosTransport`, its methods, and `probeDragosProxy`**

In `internal/services/elastic/dragos.go`, find and delete:

- `type dragosTransport struct {...}` and its long doc comment.
- `func (t *dragosTransport) RoundTrip(...) {...}`.
- `func (t *dragosTransport) do(...) {...}`.
- `func (t *dragosTransport) applyHeaders(...) {...}`.
- `func probeDragosProxy(...) {...}` (replaced by inline `probe.Run` in `NewServiceFromEnv` — wait, NewServiceFromEnv doesn't probe today). Actually, `probeDragosProxy` was used by the OLD `NewDragosService` to validate auth. The new `NewServiceFromEnv` doesn't probe. Phase 2 already moved the auth-time probe into `auth.SwitchEnvironment.runJWTProbe`. So we don't need `probeDragosProxy` here at all — delete it.

- [ ] **Step 4: Update imports**

`internal/services/elastic/dragos.go` imports were sized for the old `dragosTransport`. After deletion, prune. Likely orphans:

- `bytes` — was used by `do()`. Now unused. Remove.
- `context` — was used by `do()` and `probeDragosProxy()`. Check whether anything else in this file still uses context. Most likely unused now. Remove if so.
- `io` — was used by `RoundTrip()` for body buffering and by `probeDragosProxy()` for body sniffing. Now unused. Remove.
- `net/http` — was used everywhere. Now unused in this file. Remove.
- `net/url` — was used by `RoundTrip()`. Now unused. Remove.
- `strings` — was used by `RoundTrip()` and `probeDragosProxy()`. Now unused. Remove.
- `sync` — was used in the struct. Now unused. Remove.
- `time` — was used by `do()` client timeout and `probeDragosProxy()` context. Now unused. Remove.
- `github.com/elastic/go-elasticsearch/v6` — was used by `NewDragosService` for elasticsearch.Config. Now unused (the new wrappers delegate). Remove.
- `github.com/spf13/viper` — was used by `NewDragosService` for the logger. Now unused. Remove.
- `github.com/tpe11etier/cloudcutter/internal/auth` — STILL used (for `*auth.DragosSession` and `auth.DragosProfile`). Keep.
- `github.com/tpe11etier/cloudcutter/internal/logger` — was used by `probeDragosProxy`. Now unused. Remove.
- `github.com/tpe11etier/cloudcutter/internal/probe` — was used by the phase-2 `probeDragosProxy` body. Now unused (probe runs in auth and the proxy retry logic moved into `kibanaProxyTransport`). Remove.

Add what the new wrappers need:

- `github.com/aws/aws-sdk-go-v2/aws` — for `aws.Config{}`.
- `github.com/tpe11etier/cloudcutter/internal/config` — for the spec types.
- `github.com/tpe11etier/cloudcutter/internal/environments` — for the Environment type.
- `fmt` — for error wrapping. (Already there if `NewDragosService` had any `fmt.Errorf`.)

The build will tell you exactly which imports are missing/extra; iterate until clean.

- [ ] **Step 5: Build**

Run: `go build -mod=vendor ./...`

Expected: succeeds.

- [ ] **Step 6: Run all tests**

Run: `go test -mod=vendor ./...`

Expected: PASS.

- [ ] **Step 7: gofmt sanity**

Run: `gofmt -l internal/services/elastic`

Expected: no output. If output, run `gofmt -w` on the listed files.

- [ ] **Step 8: Commit**

```bash
git add internal/services/elastic/dragos.go
git commit -m "Refactor legacy dragos entry points to delegate via Environment"
```

---

### Task 6: Phase 3 verification + tag

**Files:** none modified.

- [ ] **Step 1: Confirm old transport types are gone**

Run: `grep -rn 'type awsTransport\|type dragosTransport' --include='*.go' .`

Expected: no output.

Run: `grep -rn 'probeDragosProxy\|isHTMLResponse' --include='*.go' .`

Expected: no output.

- [ ] **Step 2: Confirm UI / cmd / auth are unchanged since phase-2-complete**

Run: `git diff phase-2-complete..HEAD -- 'internal/auth/' 'internal/ui/' 'cmd/cloudcutter/' 'internal/probe/' 'internal/config/' 'internal/environments/'`

Expected: no output. Phase 3 is supposed to leave those layers untouched.

- [ ] **Step 3: Confirm new types exist**

Run: `grep -n 'type sigv4Transport\|type kibanaProxyTransport\|func NewServiceFromEnv\|func.*ReinitializeFromEnv' internal/services/elastic/*.go`

Expected: each name appears at least once.

- [ ] **Step 4: Full test sweep**

Run: `go test -mod=vendor ./...`

Expected: every package PASSes.

- [ ] **Step 5: `go vet` + gofmt**

Run: `go vet -mod=vendor ./...`

Expected: no output.

Run: `gofmt -l internal/services/elastic`

Expected: no output.

- [ ] **Step 6: Confirm no callers in the running app touch the new entry points yet**

Run: `grep -rn 'NewServiceFromEnv\|ReinitializeFromEnv' --include='*.go' . | grep -v internal/services/elastic/`

Expected: no output. Only the elastic package itself should reference these names in phase 3 (its own tests + the legacy wrappers in elastic.go and dragos.go that delegate). Phase 4 will introduce the first external caller.

- [ ] **Step 7: Tag the phase**

```bash
git tag phase-3-complete
```

- [ ] **Step 8: Update the spec status**

Edit `docs/superpowers/specs/2026-05-09-generic-backend-design.md` and change the `**Status**:` line:

From:

```
**Status**: Phase 2 implemented (tag `phase-2-complete`). Phase 3 (transport refactor) plan to be drafted next.
```

To:

```
**Status**: Phase 3 implemented (tag `phase-3-complete`). Phase 4 (manager + view cleanup) plan to be drafted next.
```

```bash
git add docs/superpowers/specs/2026-05-09-generic-backend-design.md
git commit -m "Note phase 3 complete in spec status"
```

---

## What's NOT in phase 3 (deferred)

- **Manager-resolver wiring** — the manager's `switchTo*Profile` funcs continue to call `services.InitializeElastic(cfg)` / `services.InitializeElasticDragos(d)` exactly as today. Phase 4.
- **View `Reinitialize` rework** — view.go still checks `session.Dragos != nil` and calls `service.Reinitialize(cfg, profile)` / `service.ReinitializeDragos(session.Dragos)`. Phase 4.
- **Legacy entry-point deletion** — `NewService(cfg)`, `Reinitialize(cfg, profile)`, `NewDragosService(d)`, `ReinitializeDragos(d)`, `services.InitializeElastic(cfg)`, `services.InitializeElasticDragos(d)` all survive phase 3. Phase 4 deletes the manager-side calls; phase 5 deletes the wrappers themselves.
- **`auth.DragosSession`** — still produced by `auth.SwitchEnvironment`'s legacy-DragosSession-population branch. Phase 4 drops it from the Session struct; phase 5 deletes the type.
- **The hardcoded darkbytes URL** — moved out of the transport but lives in `legacySophosEnvFromAWSConfig`. Phase 4 removes it when YAML supplies the URL.

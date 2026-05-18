package elastic

import (
	"bytes"
	"context"
	"crypto/tls"
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
	client := &http.Client{Timeout: 30 * time.Second}
	if spec.TLSSkipVerify {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec
	}
	return &kibanaProxyTransport{
		client:       client,
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

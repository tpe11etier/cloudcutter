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

	"github.com/elastic/go-elasticsearch/v6"
	"github.com/spf13/viper"
	"github.com/tpe11etier/cloudcutter/internal/auth"
	"github.com/tpe11etier/cloudcutter/internal/logger"
)

// dragosTransport rewrites every request the elasticsearch SDK makes so it
// goes through Kibana's `api/console/proxy` endpoint instead of hitting ES
// directly. The endpoint is what Kibana's Dev Tools console uses, and it
// returns plain ES JSON, so the rest of the SDK keeps working unchanged.
//
// Wire shape:
//
//	SDK builds:    POST {base}/_search?pretty=true     body=<query>
//	we rewrite to: POST {base}/kibana/api/console/proxy?path=_search?pretty=true&method=POST
//	                    body=<query>
//
// For GET requests the SDK sends with no body, we still POST to the proxy
// with the original method tunneled via ?method=GET (proxy requires POST).
//
// On a 401 response, the transport tries to refresh the auth cookie (e.g. by
// re-reading the user's browser cookie store) and retries the request once.
// This is what lets the user just hit refresh in their Kibana browser tab to
// renew the JWT without restarting cloudcutter.
type dragosTransport struct {
	client     *http.Client
	baseURL    string
	kbnVersion string
	mu         sync.Mutex
	authCookie string
	refresh    func() (string, error)
}

func (t *dragosTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	originalPath := strings.TrimPrefix(req.URL.Path, "/")
	originalMethod := req.Method
	originalQuery := req.URL.RawQuery

	pathParam := originalPath
	if originalQuery != "" {
		pathParam = originalPath + "?" + originalQuery
	}

	proxyURL, err := url.Parse(t.baseURL + "/kibana/api/console/proxy")
	if err != nil {
		return nil, fmt.Errorf("invalid dragos base URL: %w", err)
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
	if resp.StatusCode != http.StatusUnauthorized || t.refresh == nil {
		return resp, nil
	}

	newToken, refreshErr := t.refresh()
	if refreshErr != nil || newToken == "" {
		return resp, nil
	}
	t.mu.Lock()
	stale := t.authCookie == newToken
	if !stale {
		t.authCookie = newToken
	}
	t.mu.Unlock()
	if stale {
		return resp, nil
	}

	// Drain and discard the 401 body, then retry once with the fresh token.
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return t.do(req.Context(), proxyURL.String(), bodyBytes)
}

func (t *dragosTransport) do(ctx context.Context, proxyURL string, bodyBytes []byte) (*http.Response, error) {
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

func (t *dragosTransport) applyHeaders(req *http.Request) {
	t.mu.Lock()
	cookie := t.authCookie
	t.mu.Unlock()
	req.Header.Set("Cookie", "dragos-auth-token="+cookie)
	req.Header.Set("kbn-xsrf", "cloudcutter")
	req.Header.Set("Content-Type", "application/json")
	if t.kbnVersion != "" {
		req.Header.Set("kbn-version", t.kbnVersion)
	}
}

// NewDragosService constructs an elastic Service that talks to Kibana's
// console/proxy on a Dragos platform deployment. Probes connectivity once at
// startup so a misconfigured token or disabled proxy fails loudly here rather
// than on first user query.
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
		Prefix: "es_svc_dragos",
		Level:  level,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %s", err)
	}

	transport := &dragosTransport{
		client:     &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(d.BaseURL, "/"),
		authCookie: d.AuthToken,
		kbnVersion: d.KbnVersion,
		refresh: func() (string, error) {
			cfg, err := auth.LoadDragosConfig()
			if err != nil {
				return "", err
			}
			return cfg.AuthToken, nil
		},
	}

	if err := probeDragosProxy(context.Background(), transport, l); err != nil {
		return nil, err
	}

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses:     []string{transport.baseURL},
		Transport:     transport,
		EnableMetrics: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	s := &Service{
		Client: client,
		log:    l,
		cache:  make(map[string]*IndexStats),
		mu:     sync.RWMutex{},
	}

	if err := s.PreloadIndexStats(context.Background()); err != nil {
		l.Warn("Initial cache preload failed: %v", err)
	}

	return s, nil
}

// ReinitializeDragos swaps the underlying client to a fresh Dragos transport.
// Used when the user re-selects the dragos profile (e.g. after refreshing
// their token).
func (s *Service) ReinitializeDragos(d *auth.DragosSession) error {
	if d == nil {
		return fmt.Errorf("nil DragosSession")
	}

	transport := &dragosTransport{
		client:     &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(d.BaseURL, "/"),
		authCookie: d.AuthToken,
		kbnVersion: d.KbnVersion,
		refresh: func() (string, error) {
			cfg, err := auth.LoadDragosConfig()
			if err != nil {
				return "", err
			}
			return cfg.AuthToken, nil
		},
	}

	if err := probeDragosProxy(context.Background(), transport, s.log); err != nil {
		return err
	}

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses:     []string{transport.baseURL},
		Transport:     transport,
		EnableMetrics: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create elasticsearch client: %w", err)
	}
	s.Client = client
	return nil
}

// probeDragosProxy sends one POST to console/proxy?path=_cluster/health to
// confirm we can reach Kibana with the given cookie. We can't trust the
// status code alone — Dragos's edge gateway answers any unauthenticated or
// unmatched route with the SPA login page (HTML, status 200) — so we also
// check the response body's content type / shape and treat HTML as auth
// failure.
func probeDragosProxy(ctx context.Context, t *dragosTransport, l *logger.Logger) error {
	probeURL := t.baseURL + "/kibana/api/console/proxy?path=_cluster/health&method=GET"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, probeURL, nil)
	if err != nil {
		return fmt.Errorf("dragos probe: build request: %w", err)
	}
	t.applyHeaders(req)

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req = req.WithContext(probeCtx)

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("dragos probe: request failed (check DRAGOS_BASE_URL and network): %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	ct := resp.Header.Get("Content-Type")

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("dragos probe: %d unauthorized — token is missing/expired/invalid", resp.StatusCode)
	}

	if isHTMLResponse(ct, body) {
		return fmt.Errorf("dragos probe: got HTML (login page) from %s — token is stale or the gateway rejected it", probeURL)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dragos probe: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	l.Debug("Dragos console/proxy probe ok", "content_type", ct)
	return nil
}

func isHTMLResponse(contentType string, body []byte) bool {
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
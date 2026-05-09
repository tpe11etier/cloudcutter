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
)

// ProbeDragos verifies a token+baseURL combination by hitting the Kibana
// console proxy with `_cluster/health`. The Dragos edge gateway answers
// unauthenticated requests with HTTP 200 + the SPA login HTML, so the probe
// also rejects HTML responses regardless of status code.
//
// Called from authenticateDragos so a stale token surfaces as an error during
// the synchronous SwitchProfile path — otherwise the failure happens deep
// inside the lazy view constructor where errors are swallowed.
func ProbeDragos(ctx context.Context, baseURL, token, kbnVersion string) error {
	if baseURL == "" {
		return fmt.Errorf("dragos baseURL is empty")
	}
	if token == "" {
		return fmt.Errorf("dragos auth token is empty")
	}

	probeURL := strings.TrimRight(baseURL, "/") + "/kibana/api/console/proxy?path=_cluster/health&method=GET"

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, probeURL, nil)
	if err != nil {
		return fmt.Errorf("dragos probe: build request: %w", err)
	}
	req.Header.Set("Cookie", "dragos-auth-token="+token)
	req.Header.Set("kbn-xsrf", "cloudcutter")
	req.Header.Set("Content-Type", "application/json")
	if kbnVersion != "" {
		req.Header.Set("kbn-version", kbnVersion)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("dragos probe: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	ct := resp.Header.Get("Content-Type")

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("dragos probe: %d unauthorized — token is missing/expired/invalid", resp.StatusCode)
	}
	if strings.Contains(strings.ToLower(ct), "text/html") {
		return fmt.Errorf("dragos probe: gateway returned HTML — token rejected or stale")
	}
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) > 16 {
		trimmed = trimmed[:16]
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html") {
		return fmt.Errorf("dragos probe: gateway returned HTML — token rejected or stale")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dragos probe: status %d", resp.StatusCode)
	}
	return nil
}

// LoginWithPassword exchanges a username/password for a Dragos session JWT by
// POSTing to /auth/api/v1/login/password?providerId=<providerID>. The JWT is
// returned by the server as a Set-Cookie header (`dragos-auth-token=...`); we
// extract and return that value.
//
// The caller is expected to persist the result (typically via SaveDragosToken)
// so subsequent runs don't need to log in again. Tokens last about an hour.
func LoginWithPassword(ctx context.Context, baseURL, providerID, username, password string) (string, error) {
	if baseURL == "" {
		return "", fmt.Errorf("dragos baseURL is empty")
	}
	if providerID == "" {
		providerID = DefaultDragosProviderID
	}
	if username == "" || password == "" {
		return "", fmt.Errorf("username and password are required")
	}

	loginURL, err := url.Parse(strings.TrimRight(baseURL, "/") + "/auth/api/v1/login/password")
	if err != nil {
		return "", fmt.Errorf("invalid baseURL: %w", err)
	}
	q := loginURL.Query()
	q.Set("providerId", providerID)
	loginURL.RawQuery = q.Encode()

	body, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return "", fmt.Errorf("marshal credentials: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, loginURL.String(), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Mimic what a real browser sends — some gateways reject non-browser UAs.
	req.Header.Set("User-Agent", "cloudcutter")

	// Explicit Timeout in addition to the context — context timeout depends on
	// the request actually getting far enough to respect cancellation, which
	// some failure modes (DNS issues with CGO_ENABLED=0, hung TLS handshake)
	// don't always honor. http.Client.Timeout always wins.
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("invalid username or password")
	}
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("login failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	for _, c := range resp.Cookies() {
		if c.Name == "dragos-auth-token" && c.Value != "" {
			return c.Value, nil
		}
	}
	return "", fmt.Errorf("login succeeded (200) but no dragos-auth-token cookie in response")
}

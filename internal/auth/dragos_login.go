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

// Package probe runs a single HTTP request to verify that auth and
// connectivity to a backend are working. It does not know what the
// backend's URL or headers should look like; the caller builds the
// request and probe inspects the response.
//
// probe is consumed by both internal/auth (validate-on-switch) and
// internal/services/elastic (validate-on-construct) so the two
// previously-separate probe functions are now one.
package probe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Run executes req via client and returns nil on success.
//
// Failure modes:
//   - 401 / 403 → "probe: NNN unauthorized — token is missing/expired/invalid"
//   - rejectHTML=true and response is HTML (by Content-Type OR body sniff
//     of the first 16 bytes for "<!doctype html"/"<html") →
//     "probe: gateway returned HTML — token rejected or stale"
//   - other non-200 status → "probe: unexpected status NNN"
//   - transport error → "probe: <wrapped>"
//
// rejectHTML exists because some gateways serve their SPA login page at
// HTTP 200 when auth fails, defeating naive status-code checks. The
// Dragos platform behaves this way.
//
// If client is nil, http.DefaultClient is used.
func Run(ctx context.Context, client *http.Client, req *http.Request, rejectHTML bool) error {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("probe: %d unauthorized — token is missing/expired/invalid", resp.StatusCode)
	}

	if rejectHTML && isHTML(resp.Header.Get("Content-Type"), body) {
		return fmt.Errorf("probe: gateway returned HTML — token rejected or stale")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("probe: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func isHTML(contentType string, body []byte) bool {
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

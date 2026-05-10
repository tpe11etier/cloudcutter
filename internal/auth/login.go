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

	"github.com/tpe11etier/cloudcutter/internal/config"
)

// LoginJWT exchanges form values for a JWT according to the LoginSpec.
// It POSTs to spec.URL with body in spec.BodyFormat (json | form),
// applies any spec.Query params, and extracts the token per
// spec.TokenExtract.
//
// Replaces the dragos-specific LoginWithPassword. The new function is
// vendor-neutral: a future env can declare its own URL, body format,
// query, and token extraction strategy without touching Go.
func LoginJWT(ctx context.Context, spec config.LoginSpec, formValues map[string]string) (string, error) {
	if spec.URL == "" {
		return "", fmt.Errorf("login: spec.URL is empty")
	}

	loginURL, err := url.Parse(spec.URL)
	if err != nil {
		return "", fmt.Errorf("login: invalid URL %q: %w", spec.URL, err)
	}
	if len(spec.Query) > 0 {
		q := loginURL.Query()
		for k, v := range spec.Query {
			q.Set(k, v)
		}
		loginURL.RawQuery = q.Encode()
	}

	body, contentType, err := encodeLoginBody(spec.BodyFormat, formValues)
	if err != nil {
		return "", err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, loginURL.String(), body)
	if err != nil {
		return "", fmt.Errorf("login: build request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cloudcutter")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("invalid credentials")
	}
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("login failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	return extractToken(resp, spec.TokenExtract)
}

func encodeLoginBody(format string, values map[string]string) (io.Reader, string, error) {
	switch format {
	case "", "json":
		raw, err := json.Marshal(values)
		if err != nil {
			return nil, "", fmt.Errorf("marshal credentials: %w", err)
		}
		return bytes.NewReader(raw), "application/json", nil
	case "form":
		formData := url.Values{}
		for k, v := range values {
			formData.Set(k, v)
		}
		return strings.NewReader(formData.Encode()), "application/x-www-form-urlencoded", nil
	default:
		return nil, "", fmt.Errorf("login: unknown body_format %q (want json|form)", format)
	}
}

func extractToken(resp *http.Response, spec config.TokenExtractSpec) (string, error) {
	switch spec.From {
	case "cookie":
		for _, c := range resp.Cookies() {
			if c.Name == spec.Name && c.Value != "" {
				return c.Value, nil
			}
		}
		return "", fmt.Errorf("login OK but no %q cookie in response", spec.Name)
	case "header":
		if v := resp.Header.Get(spec.Name); v != "" {
			return v, nil
		}
		return "", fmt.Errorf("login OK but no %q header in response", spec.Name)
	case "json_path":
		return "", fmt.Errorf("token_extract from=json_path not yet implemented")
	default:
		return "", fmt.Errorf("token_extract: unknown from %q", spec.From)
	}
}

package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tpe11etier/cloudcutter/internal/config"
)

func TestLoginJWTPostsJSONAndExtractsCookie(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotProviderID string
	var gotBody map[string]string
	var gotContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotProviderID = r.URL.Query().Get("providerId")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		http.SetCookie(w, &http.Cookie{Name: "auth-tok", Value: "the-jwt"})
		w.WriteHeader(200)
	}))
	defer srv.Close()

	spec := config.LoginSpec{
		URL:        srv.URL + "/auth/api/v1/login/password",
		BodyFormat: "json",
		BodyFields: []config.FormField{
			{Name: "username", Kind: "text"},
			{Name: "password", Kind: "password"},
		},
		Query: map[string]string{"providerId": "abc-123"},
		TokenExtract: config.TokenExtractSpec{
			From: "cookie",
			Name: "auth-tok",
		},
	}
	values := map[string]string{
		"username": "alice",
		"password": "s3cret",
	}

	tok, err := LoginJWT(context.Background(), spec, config.TransportSpec{},values)
	if err != nil {
		t.Fatalf("LoginJWT: %v", err)
	}
	if tok != "the-jwt" {
		t.Errorf("token = %q, want the-jwt", tok)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/auth/api/v1/login/password") {
		t.Errorf("path = %q", gotPath)
	}
	if gotProviderID != "abc-123" {
		t.Errorf("providerId = %q", gotProviderID)
	}
	if !strings.Contains(gotContentType, "application/json") {
		t.Errorf("content-type = %q", gotContentType)
	}
	if gotBody["username"] != "alice" || gotBody["password"] != "s3cret" {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestLoginJWTFormBodyFormat(t *testing.T) {
	var gotBody string
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("X-Auth", "headertok")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	spec := config.LoginSpec{
		URL:        srv.URL + "/login",
		BodyFormat: "form",
		BodyFields: []config.FormField{
			{Name: "u", Kind: "text"},
			{Name: "p", Kind: "password"},
		},
		TokenExtract: config.TokenExtractSpec{From: "header", Name: "X-Auth"},
	}
	values := map[string]string{"u": "bob", "p": "pw"}

	tok, err := LoginJWT(context.Background(), spec, config.TransportSpec{},values)
	if err != nil {
		t.Fatalf("LoginJWT: %v", err)
	}
	if tok != "headertok" {
		t.Errorf("token = %q, want headertok", tok)
	}
	if !strings.Contains(gotContentType, "x-www-form-urlencoded") {
		t.Errorf("content-type = %q", gotContentType)
	}
	parsed, _ := url.ParseQuery(gotBody)
	if parsed.Get("u") != "bob" || parsed.Get("p") != "pw" {
		t.Errorf("body = %q", gotBody)
	}
}

func TestLoginJWTRejects401As4InvalidCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	spec := config.LoginSpec{
		URL:          srv.URL + "/login",
		BodyFormat:   "json",
		BodyFields:   []config.FormField{{Name: "u", Kind: "text"}},
		TokenExtract: config.TokenExtractSpec{From: "cookie", Name: "x"},
	}
	_, err := LoginJWT(context.Background(), spec, config.TransportSpec{},map[string]string{"u": "x"})
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error should mention invalid credentials, got %q", err.Error())
	}
}

func TestLoginJWTRejectsJSONPathNotImplemented(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	spec := config.LoginSpec{
		URL:          srv.URL + "/login",
		BodyFormat:   "json",
		BodyFields:   []config.FormField{{Name: "u", Kind: "text"}},
		TokenExtract: config.TokenExtractSpec{From: "json_path", Name: "$.token"},
	}
	_, err := LoginJWT(context.Background(), spec, config.TransportSpec{},map[string]string{"u": "x"})
	if err == nil {
		t.Fatal("expected error for json_path")
	}
	if !strings.Contains(err.Error(), "json_path") {
		t.Errorf("error should mention json_path, got %q", err.Error())
	}
}

func TestLoginJWTMissingCookieErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200) // 200 but no cookie
	}))
	defer srv.Close()

	spec := config.LoginSpec{
		URL:          srv.URL + "/login",
		BodyFormat:   "json",
		BodyFields:   []config.FormField{{Name: "u", Kind: "text"}},
		TokenExtract: config.TokenExtractSpec{From: "cookie", Name: "auth-tok"},
	}
	_, err := LoginJWT(context.Background(), spec, config.TransportSpec{},map[string]string{"u": "x"})
	if err == nil {
		t.Fatal("expected error when cookie is absent")
	}
	if !strings.Contains(err.Error(), "auth-tok") {
		t.Errorf("error should name expected cookie, got %q", err.Error())
	}
}

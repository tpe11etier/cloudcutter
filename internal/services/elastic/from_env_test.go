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

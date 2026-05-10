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

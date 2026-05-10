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
	payloadHash := sha256Hex(body)
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

func sha256Hex(b []byte) string {
	if b == nil {
		return "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}
	h := sha256.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

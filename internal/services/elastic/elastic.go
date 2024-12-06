package elastic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/elastic/go-elasticsearch/v6"
)

type Service struct {
	Client *elasticsearch.Client
}

type awsTransport struct {
	client *http.Client
	cfg    aws.Config
	region string
}

func NewService(cfg aws.Config) (*Service, error) {
	// Create AWS transport with config
	transport := &awsTransport{
		client: &http.Client{},
		cfg:    cfg,
		region: cfg.Region,
	}

	// Create Elasticsearch client config
	esEndpoint := fmt.Sprintf("https://dev-%s-primary-es.darkbytes.io", cfg.Region)
	esConfig := elasticsearch.Config{
		Addresses:     []string{esEndpoint},
		Transport:     transport,
		EnableMetrics: true,
	}

	// Initialize the client
	client, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating Elasticsearch client: %v", err)
	}

	return &Service{Client: client}, nil
}

// Move the transport methods into the elastic package
func (t *awsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
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
	payloadHash := hashPayload(body)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	signer := v4.NewSigner()
	err = signer.SignHTTP(req.Context(), credentials, req, payloadHash, "es", t.region, time.Now())
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	return t.client.Do(req)
}

func hashPayload(b []byte) string {
	if b == nil {
		return "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}
	h := sha256.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Service) SearchDocuments(ctx context.Context, index string, query map[string]any) ([]map[string]any, int, error) {
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal query: %w", err)
	}

	res, err := s.Client.Search(
		s.Client.Search.WithContext(ctx),
		s.Client.Search.WithIndex(index),
		s.Client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("search request failed: %w", err)
	}
	defer res.Body.Close()

	var result struct {
		Hits struct {
			Total int `json:"total"`
			Hits  []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("decoding response failed: %w", err)
	}

	docs := make([]map[string]any, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		var doc map[string]any
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			// Skip malformed documents
			continue
		}
		docs = append(docs, doc)
	}

	return docs, result.Hits.Total, nil
}

func (s *Service) ListIndices(ctx context.Context, pattern string) ([]string, error) {
	if pattern == "" {
		pattern = "*"
	}

	res, err := s.Client.Cat.Indices(
		s.Client.Cat.Indices.WithContext(ctx),
		s.Client.Cat.Indices.WithFormat("json"),
		s.Client.Cat.Indices.WithS("index:desc"),
		s.Client.Cat.Indices.WithH("index"),
		s.Client.Cat.Indices.WithV(true),
		s.Client.Cat.Indices.WithIndex(pattern),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list indices: %w", err)
	}
	defer res.Body.Close()

	var indices []struct {
		Index string `json:"index"`
	}
	if err := json.NewDecoder(res.Body).Decode(&indices); err != nil {
		return nil, fmt.Errorf("decoding indices response failed: %w", err)
	}

	var names []string
	for _, idx := range indices {
		names = append(names, idx.Index)
	}
	return names, nil
}

func (s *Service) Reinitialize(cfg aws.Config, profile string) error {
	transport := &awsTransport{
		client: &http.Client{},
		cfg:    cfg,
		region: cfg.Region,
	}

	endpointPrefix := "dev"
	if profile == "opal_prod" {
		endpointPrefix = "prod"
	}

	esEndpoint := fmt.Sprintf("https://%s-%s-primary-es.darkbytes.io", endpointPrefix, cfg.Region)

	esConfig := elasticsearch.Config{
		Addresses:     []string{esEndpoint},
		Transport:     transport,
		EnableMetrics: true,
	}

	newClient, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return fmt.Errorf("error reinitializing Elasticsearch client: %v", err)
	}

	s.Client = newClient
	return nil
}

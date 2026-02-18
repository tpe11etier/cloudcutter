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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/tpelletiersophos/cloudcutter/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/elastic/go-elasticsearch/v6"
)

type Service struct {
	Client *elasticsearch.Client
	log    *logger.Logger
	cache  map[string]*IndexStats
	mu     sync.RWMutex
}

type awsTransport struct {
	client *http.Client
	cfg    aws.Config
	region string
}

func NewService(cfg aws.Config) (*Service, error) {
	logDir := viper.GetString("log_dir")
	if logDir == "" {
		logDir = "./logs"
	}
	logPrefix := "es_svc"
	logLevel := strings.ToLower(viper.GetString("logging"))
	level, err := logger.ParseLevel(logLevel)
	if err != nil {
		level = logger.INFO
	}

	logCfg := logger.Config{
		LogDir: logDir,
		Prefix: logPrefix,
		Level:  level,
	}

	l, err := logger.New(logCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %s", err)
	}

	var esConfig elasticsearch.Config
	var client *elasticsearch.Client

	if cfg.Region == "local" {
		l.Debug("Configuring local elasticsearch connection")
		esConfig = elasticsearch.Config{
			Addresses: []string{"http://localhost:9200"},
		}
	} else {
		l.Debug("Configuring AWS elasticsearch connection for region: %s", "region", cfg.Region)
		transport := &awsTransport{
			client: &http.Client{},
			cfg:    cfg,
			region: cfg.Region,
		}

		esEndpoint := fmt.Sprintf("https://dev-%s-primary-es.darkbytes.io", cfg.Region)
		esConfig = elasticsearch.Config{
			Addresses:     []string{esEndpoint},
			Transport:     transport,
			EnableMetrics: true,
		}
	}

	client, err = elasticsearch.NewClient(esConfig)
	if err != nil {
		l.Warn("Failed to create Elasticsearch client: %v", err)
		// Continue with nil client - service will operate in no-op mode
	}

	s := &Service{
		Client: client,
		log:    l,
		cache:  make(map[string]*IndexStats),
		mu:     sync.RWMutex{},
	}

	// Try to preload if we have a client
	if client != nil {
		if err := s.PreloadIndexStats(context.Background()); err != nil {
			l.Warn("Initial cache preload failed: %v", err)
			// Continue without preloaded cache
		}
	}

	return s, nil
}

func (s *Service) Reinitialize(cfg aws.Config, profile string) error {
	var esConfig elasticsearch.Config

	if cfg.Region == "local" {
		esConfig = elasticsearch.Config{
			Addresses: []string{"http://localhost:9200"},
		}
	} else {
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
		esConfig = elasticsearch.Config{
			Addresses:     []string{esEndpoint},
			Transport:     transport,
			EnableMetrics: true,
		}
	}

	newClient, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return fmt.Errorf("error reinitializing Elasticsearch client: %s", err)
	}

	s.Client = newClient
	return nil
}

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

func (s *Service) ListIndices(ctx context.Context, pattern string) ([]string, error) {
	if s.Client == nil {
		s.log.Debug("ListIndices called in no-op mode")
		return []string{}, nil
	}

	if pattern == "" {
		pattern = "*"
	}

	s.log.Debug("Listing indices with pattern: %s", "pattern", pattern)
	res, err := s.Client.Cat.Indices(
		s.Client.Cat.Indices.WithContext(ctx),
		s.Client.Cat.Indices.WithFormat("json"),
		s.Client.Cat.Indices.WithS("index:desc"),
		s.Client.Cat.Indices.WithH("index"),
		s.Client.Cat.Indices.WithV(true),
		s.Client.Cat.Indices.WithIndex(pattern),
	)
	if err != nil {
		s.log.Error("Failed to list indices", "error", err)
		return nil, fmt.Errorf("failed to list indices: %v", err)
	}
	defer res.Body.Close()

	var indices []struct {
		Index string `json:"index"`
	}

	if err := json.NewDecoder(res.Body).Decode(&indices); err != nil {
		s.log.Error("Failed to decode indices response",
			"error", err,
			"will_continue", true)

		if indices == nil {
			indices = make([]struct {
				Index string `json:"index"`
			}, 0)
		}
	}

	names := make([]string, 0, len(indices))
	for _, idx := range indices {
		if idx.Index != "" {
			names = append(names, idx.Index)
		}
	}

	if len(names) == 0 {
		s.log.Warn("No valid indices found",
			"pattern", pattern,
			"total_attempted", len(indices))
	} else {
		s.log.Debug("Found indices",
			"count", len(names),
			"total_attempted", len(indices))
	}

	return names, nil
}

type IndexStats struct {
	Health       string `json:"health"`
	Status       string `json:"status"`
	Index        string `json:"index"`
	UUID         string `json:"uuid"`
	Primary      string `json:"pri"`
	Replica      string `json:"rep"`
	DocsCount    string `json:"docs.count"`
	DocsDeleted  string `json:"docs.deleted"`
	StoreSize    string `json:"store.size"`
	PriStoreSize string `json:"pri.store.size"`
}

func parseSize(size string) (float64, string) {
	i := 0
	for i < len(size) && (size[i] == '.' || size[i] == '-' || (size[i] >= '0' && size[i] <= '9')) {
		i++
	}

	if i == 0 {
		return 0, "b"
	}

	value, err := strconv.ParseFloat(size[:i], 64)
	if err != nil {
		return 0, "b"
	}

	unit := strings.ToLower(strings.TrimSpace(size[i:]))

	switch unit {
	case "kb":
		return value * 1024, "kb"
	case "mb":
		return value * 1024 * 1024, "mb"
	case "gb":
		return value * 1024 * 1024 * 1024, "gb"
	default:
		return value, "b"
	}
}

func formatSize(bytes float64) string {
	units := []string{"b", "kb", "mb", "gb", "tb"}
	var i int
	value := bytes

	for value > 1024 && i < len(units)-1 {
		value /= 1024
		i++
	}

	return fmt.Sprintf("%.1f%s", value, units[i])
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = nil
	return nil
}

func (s *Service) PreloadIndexStats(ctx context.Context) error {
	if s.Client == nil {
		s.log.Debug("PreloadIndexStats called in no-op mode")
		return nil
	}

	s.log.Debug("Starting index stats preload")
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := s.Client.Cat.Indices(
		s.Client.Cat.Indices.WithContext(ctx),
		s.Client.Cat.Indices.WithFormat("json"),
		s.Client.Cat.Indices.WithH("health,status,index,uuid,pri,rep,docs.count,docs.deleted,store.size,pri.store.size"),
		s.Client.Cat.Indices.WithV(true),
	)
	if err != nil {
		return fmt.Errorf("failed to preload index stats: %s", err)
	}
	defer res.Body.Close()

	var stats []IndexStats
	if err := json.NewDecoder(res.Body).Decode(&stats); err != nil {
		return fmt.Errorf("failed to decode index stats: %s", err)
	}

	newCache := make(map[string]*IndexStats)

	for _, stat := range stats {
		newCache[stat.Index] = &stat
	}

	patternGroups := make(map[string][]string)
	for _, stat := range stats {
		for existingIndex := range newCache {
			// If this index matches any known pattern, add it to that group
			if strings.Contains(existingIndex, "*") {
				pattern := strings.TrimSuffix(existingIndex, "*")
				if strings.HasPrefix(stat.Index, pattern) {
					patternGroups[existingIndex] = append(patternGroups[existingIndex], stat.Index)
				}
			}
		}
	}

	for pattern, matchingIndices := range patternGroups {
		total := &IndexStats{
			Health: "green",
			Status: "open",
			Index:  pattern,
		}

		var totalDocs int64
		var totalSize float64

		for _, indexName := range matchingIndices {
			if stat := newCache[indexName]; stat != nil {
				if stat.Health == "yellow" && total.Health == "green" {
					total.Health = "yellow"
				} else if stat.Health == "red" {
					total.Health = "red"
				}

				// sum docs
				if docs, err := strconv.ParseInt(stat.DocsCount, 10, 64); err == nil {
					totalDocs += docs
				}

				// sum size
				if size, _ := parseSize(stat.StoreSize); size > 0 {
					totalSize += size
				}
			}
		}

		total.DocsCount = strconv.FormatInt(totalDocs, 10)
		total.StoreSize = formatSize(totalSize)

		newCache[pattern] = total
	}

	s.mu.Lock()
	s.cache = newCache
	s.mu.Unlock()

	return nil
}


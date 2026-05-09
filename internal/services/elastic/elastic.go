package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tpe11etier/cloudcutter/internal/config"
	"github.com/tpe11etier/cloudcutter/internal/environments"
	"github.com/tpe11etier/cloudcutter/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/elastic/go-elasticsearch/v6"
)

type Service struct {
	Client *elasticsearch.Client
	log    *logger.Logger
	cache  map[string]*IndexStats
	mu     sync.RWMutex
}

// NewService is the legacy entry point: callers pass an aws.Config
// (region pulled from there) and the Service is built against the Sophos
// darkbytes URL template. Phase 3 keeps this working by translating the
// AWS config into an Environment internally and delegating to
// NewServiceFromEnv. Phase 4 deletes this once the manager constructs
// Environment values from the Resolver and calls NewServiceFromEnv
// directly.
func NewService(cfg aws.Config) (*Service, error) {
	return NewServiceFromEnv(legacySophosEnvFromAWSConfig(cfg, "default"), cfg, "")
}

// Reinitialize is the legacy reinit entry point. profile is used only to
// pick the URL prefix ("dev" vs "prod") in the legacy darkbytes template.
func (s *Service) Reinitialize(cfg aws.Config, profile string) error {
	return s.ReinitializeFromEnv(legacySophosEnvFromAWSConfig(cfg, profile), cfg, "")
}

// legacySophosEnvFromAWSConfig builds a phase-3 bridge Environment that
// reproduces the URL the legacy awsTransport hit: localhost:9200 for
// region=local, otherwise https://{prefix}-{region}-primary-es.darkbytes.io
// where prefix is "prod" for opal_prod and "dev" otherwise.
//
// The vendor-named darkbytes URL is the only company-specific knob still
// present in the running code path after phase 3; phase 4 removes it
// when the manager supplies Environment values from the YAML resolver.
func legacySophosEnvFromAWSConfig(cfg aws.Config, profile string) environments.Environment {
	if cfg.Region == "local" {
		return environments.Environment{
			Name: "local",
			Auth: config.AuthSpec{Type: "none"},
			Transport: config.TransportSpec{
				Type:    "plain",
				BaseURL: "http://localhost:9200",
			},
		}
	}
	prefix := "dev"
	if profile == "opal_prod" {
		prefix = "prod"
	}
	return environments.Environment{
		Name:   profile,
		Region: cfg.Region,
		Auth:   config.AuthSpec{Type: "aws_sdk"},
		Transport: config.TransportSpec{
			Type:        "sigv4",
			Service:     "es",
			URLTemplate: fmt.Sprintf("https://%s-%s-primary-es.darkbytes.io", prefix, cfg.Region),
		},
	}
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

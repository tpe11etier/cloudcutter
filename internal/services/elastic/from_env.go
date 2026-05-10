package elastic

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/elastic/go-elasticsearch/v6"
	"github.com/spf13/viper"
	"github.com/tpe11etier/cloudcutter/internal/environments"
	"github.com/tpe11etier/cloudcutter/internal/logger"
)

// NewServiceFromEnv constructs an elastic Service whose transport is
// chosen from env.Transport.Type. The caller supplies the AWS config (for
// sigv4) and JWT token (for kibana_proxy); both are ignored when not
// applicable to the chosen transport.
//
// This is the unified entry point introduced in phase 3. Phase 4 makes
// the manager call this directly; until then, the legacy elastic.NewService
// and elastic.NewDragosService delegate here.
func NewServiceFromEnv(env environments.Environment, awsCfg aws.Config, token string) (*Service, error) {
	l, err := newServiceLogger("es_svc")
	if err != nil {
		return nil, err
	}

	transport, addr, err := buildTransport(env, awsCfg, token, l)
	if err != nil {
		return nil, err
	}

	esCfg := elasticsearch.Config{
		Addresses:     []string{addr},
		EnableMetrics: true,
	}
	if transport != nil {
		esCfg.Transport = transport
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	s := &Service{
		Client: client,
		log:    l,
		cache:  make(map[string]*IndexStats),
		mu:     sync.RWMutex{},
	}

	if client != nil {
		if err := s.PreloadIndexStats(context.Background()); err != nil {
			l.Warn("Initial cache preload failed: %v", err)
		}
	}

	return s, nil
}

// ReinitializeFromEnv rebuilds the underlying Client+Transport from the
// new Environment. Used when the user changes profile or region in-app.
func (s *Service) ReinitializeFromEnv(env environments.Environment, awsCfg aws.Config, token string) error {
	transport, addr, err := buildTransport(env, awsCfg, token, s.log)
	if err != nil {
		return err
	}

	esCfg := elasticsearch.Config{
		Addresses:     []string{addr},
		EnableMetrics: true,
	}
	if transport != nil {
		esCfg.Transport = transport
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return fmt.Errorf("failed to reinitialize elasticsearch client: %w", err)
	}

	s.mu.Lock()
	s.Client = client
	s.cache = make(map[string]*IndexStats)
	s.mu.Unlock()
	return nil
}

// buildTransport dispatches transport construction from env.Transport.Type
// and returns the transport plus the URL the SDK should target. A nil
// transport means "use http.DefaultTransport" (i.e., plain HTTP).
func buildTransport(env environments.Environment, awsCfg aws.Config, token string, l *logger.Logger) (http.RoundTripper, string, error) {
	tspec := env.Transport
	switch tspec.Type {
	case "plain":
		if tspec.BaseURL == "" {
			return nil, "", fmt.Errorf("transport=plain requires base_url")
		}
		return nil, tspec.BaseURL, nil

	case "sigv4":
		if tspec.URLTemplate == "" {
			return nil, "", fmt.Errorf("transport=sigv4 requires url_template")
		}
		service := tspec.Service
		if service == "" {
			service = "es"
		}
		return newSigV4Transport(awsCfg, service), tspec.URLTemplate, nil

	case "kibana_proxy":
		if tspec.BaseURL == "" {
			return nil, "", fmt.Errorf("transport=kibana_proxy requires base_url")
		}
		if tspec.ProxyPath == "" {
			return nil, "", fmt.Errorf("transport=kibana_proxy requires proxy_path")
		}
		return newKibanaProxyTransport(tspec, token, nil), strings.TrimRight(tspec.BaseURL, "/"), nil

	default:
		return nil, "", fmt.Errorf("unknown transport type %q (want plain|sigv4|kibana_proxy)", tspec.Type)
	}
}

// newServiceLogger builds the per-Service logger using viper config.
func newServiceLogger(prefix string) (*logger.Logger, error) {
	logDir := viper.GetString("log_dir")
	if logDir == "" {
		logDir = "./logs"
	}
	level, err := logger.ParseLevel(strings.ToLower(viper.GetString("logging")))
	if err != nil {
		level = logger.INFO
	}
	l, err := logger.New(logger.Config{
		LogDir: logDir,
		Prefix: prefix,
		Level:  level,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	return l, nil
}

package elastic

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ElasticViewConfig contains all configuration settings for the elastic view
type ElasticViewConfig struct {
	// Pagination settings
	Pagination PaginationConfig `json:"pagination"`

	// Search settings
	Search SearchConfig `json:"search"`

	// UI settings
	UI UIConfig `json:"ui"`

	// Async operation settings
	AsyncOps AsyncOperationsConfig `json:"async_ops"`

	// Rate limiting settings
	RateLimit RateLimitConfig `json:"rate_limit"`

	// Field management settings
	Fields FieldConfig `json:"fields"`
}

// PaginationConfig contains pagination-related settings
type PaginationConfig struct {
	DefaultPageSize int `json:"default_page_size" env:"ELASTIC_VIEW_PAGE_SIZE"`
	MaxPageSize     int `json:"max_page_size" env:"ELASTIC_VIEW_MAX_PAGE_SIZE"`
	MinPageSize     int `json:"min_page_size" env:"ELASTIC_VIEW_MIN_PAGE_SIZE"`
}

// SearchConfig contains search-related settings
type SearchConfig struct {
	DefaultTimeframe  string        `json:"default_timeframe" env:"ELASTIC_VIEW_DEFAULT_TIMEFRAME"`
	ValidTimeframes   []string      `json:"valid_timeframes"`
	DefaultNumResults int           `json:"default_num_results" env:"ELASTIC_VIEW_NUM_RESULTS"`
	MaxResults        int           `json:"max_results" env:"ELASTIC_VIEW_MAX_RESULTS"`
	LargeResultLimit  int           `json:"large_result_limit" env:"ELASTIC_VIEW_LARGE_RESULT_LIMIT"`
	ScrollBatchSize   int           `json:"scroll_batch_size" env:"ELASTIC_VIEW_SCROLL_BATCH_SIZE"`
	ScrollTimeout     time.Duration `json:"scroll_timeout" env:"ELASTIC_VIEW_SCROLL_TIMEOUT"`
	MaxRetries        int           `json:"max_retries" env:"ELASTIC_VIEW_MAX_RETRIES"`
	BaseRetryDelay    time.Duration `json:"base_retry_delay" env:"ELASTIC_VIEW_BASE_RETRY_DELAY"`
}

// UIConfig contains user interface settings
type UIConfig struct {
	ShowRowNumbers         bool   `json:"show_row_numbers" env:"ELASTIC_VIEW_SHOW_ROW_NUMBERS"`
	DefaultFieldListFilter string `json:"default_field_list_filter"`
	FieldListVisible       bool   `json:"field_list_visible" env:"ELASTIC_VIEW_FIELD_LIST_VISIBLE"`
}

// AsyncOperationsConfig contains async operation timeout and behavior settings
type AsyncOperationsConfig struct {
	DefaultTimeout       time.Duration `json:"default_timeout" env:"ELASTIC_VIEW_DEFAULT_TIMEOUT"`
	DocumentFetchTimeout time.Duration `json:"document_fetch_timeout" env:"ELASTIC_VIEW_DOC_FETCH_TIMEOUT"`
	FieldLoadTimeout     time.Duration `json:"field_load_timeout" env:"ELASTIC_VIEW_FIELD_LOAD_TIMEOUT"`
	SearchRefreshTimeout time.Duration `json:"search_refresh_timeout" env:"ELASTIC_VIEW_SEARCH_REFRESH_TIMEOUT"`
	FieldInitTimeout     time.Duration `json:"field_init_timeout" env:"ELASTIC_VIEW_FIELD_INIT_TIMEOUT"`
}

// RateLimitConfig contains rate limiting settings
type RateLimitConfig struct {
	MaxConcurrentOps  int           `json:"max_concurrent_ops" env:"ELASTIC_VIEW_MAX_CONCURRENT_OPS"`
	InitialRetryDelay time.Duration `json:"initial_retry_delay" env:"ELASTIC_VIEW_INITIAL_RETRY_DELAY"`
	MaxRetryDelay     time.Duration `json:"max_retry_delay" env:"ELASTIC_VIEW_MAX_RETRY_DELAY"`
	RetryMultiplier   float64       `json:"retry_multiplier" env:"ELASTIC_VIEW_RETRY_MULTIPLIER"`
}

// FieldConfig contains field management settings
type FieldConfig struct {
	CacheTimeout      time.Duration `json:"cache_timeout" env:"ELASTIC_VIEW_FIELD_CACHE_TIMEOUT"`
	MaxCachedFields   int           `json:"max_cached_fields" env:"ELASTIC_VIEW_MAX_CACHED_FIELDS"`
	DefaultFieldOrder []string      `json:"default_field_order"`
	AutoSelectFields  []string      `json:"auto_select_fields"`
}

// ConfigManager handles loading and validation of configuration
type ConfigManager struct {
	config *ElasticViewConfig
}

// NewConfigManager creates a new configuration manager with default settings
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		config: getDefaultConfig(),
	}
}

// getDefaultConfig returns the default configuration values
func getDefaultConfig() *ElasticViewConfig {
	return &ElasticViewConfig{
		Pagination: PaginationConfig{
			DefaultPageSize: 50,
			MaxPageSize:     1000,
			MinPageSize:     10,
		},
		Search: SearchConfig{
			DefaultTimeframe:  "today",
			ValidTimeframes:   []string{"today", "yesterday", "week", "month", "quarter", "year", "hour", "day"},
			DefaultNumResults: 1000,
			MaxResults:        50000,
			LargeResultLimit:  10000,
			ScrollBatchSize:   1000,
			ScrollTimeout:     5 * time.Minute,
			MaxRetries:        3,
			BaseRetryDelay:    500 * time.Millisecond,
		},
		UI: UIConfig{
			ShowRowNumbers:         true,
			DefaultFieldListFilter: "",
			FieldListVisible:       false,
		},
		AsyncOps: AsyncOperationsConfig{
			DefaultTimeout:       30 * time.Second,
			DocumentFetchTimeout: 15 * time.Second,
			FieldLoadTimeout:     20 * time.Second,
			SearchRefreshTimeout: 45 * time.Second,
			FieldInitTimeout:     30 * time.Second,
		},
		RateLimit: RateLimitConfig{
			MaxConcurrentOps:  10,
			InitialRetryDelay: 1 * time.Second,
			MaxRetryDelay:     30 * time.Second,
			RetryMultiplier:   2.0,
		},
		Fields: FieldConfig{
			CacheTimeout:    1 * time.Hour,
			MaxCachedFields: 1000,
			DefaultFieldOrder: []string{
				"@timestamp", "message", "level", "logger", "thread",
				"host", "service", "environment", "trace_id", "span_id",
			},
			AutoSelectFields: []string{"@timestamp", "message"},
		},
	}
}

// LoadFromEnvironment loads configuration from environment variables
func (cm *ConfigManager) LoadFromEnvironment() error {
	config := cm.config

	// Load pagination settings
	if val, exists := os.LookupEnv("ELASTIC_VIEW_PAGE_SIZE"); exists {
		if pageSize, err := strconv.Atoi(val); err == nil {
			config.Pagination.DefaultPageSize = pageSize
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_PAGE_SIZE: %w", err)
		}
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_MAX_PAGE_SIZE"); exists {
		if maxSize, err := strconv.Atoi(val); err == nil {
			config.Pagination.MaxPageSize = maxSize
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_MAX_PAGE_SIZE: %w", err)
		}
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_MIN_PAGE_SIZE"); exists {
		if minSize, err := strconv.Atoi(val); err == nil {
			config.Pagination.MinPageSize = minSize
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_MIN_PAGE_SIZE: %w", err)
		}
	}

	// Load search settings
	if val, exists := os.LookupEnv("ELASTIC_VIEW_DEFAULT_TIMEFRAME"); exists {
		config.Search.DefaultTimeframe = val
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_NUM_RESULTS"); exists {
		if numResults, err := strconv.Atoi(val); err == nil {
			config.Search.DefaultNumResults = numResults
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_NUM_RESULTS: %w", err)
		}
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_MAX_RESULTS"); exists {
		if maxResults, err := strconv.Atoi(val); err == nil {
			config.Search.MaxResults = maxResults
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_MAX_RESULTS: %w", err)
		}
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_LARGE_RESULT_LIMIT"); exists {
		if limit, err := strconv.Atoi(val); err == nil {
			config.Search.LargeResultLimit = limit
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_LARGE_RESULT_LIMIT: %w", err)
		}
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_MAX_RETRIES"); exists {
		if retries, err := strconv.Atoi(val); err == nil {
			config.Search.MaxRetries = retries
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_MAX_RETRIES: %w", err)
		}
	}

	// Load timeout settings
	if val, exists := os.LookupEnv("ELASTIC_VIEW_DEFAULT_TIMEOUT"); exists {
		if timeout, err := time.ParseDuration(val); err == nil {
			config.AsyncOps.DefaultTimeout = timeout
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_DEFAULT_TIMEOUT: %w", err)
		}
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_DOC_FETCH_TIMEOUT"); exists {
		if timeout, err := time.ParseDuration(val); err == nil {
			config.AsyncOps.DocumentFetchTimeout = timeout
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_DOC_FETCH_TIMEOUT: %w", err)
		}
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_FIELD_LOAD_TIMEOUT"); exists {
		if timeout, err := time.ParseDuration(val); err == nil {
			config.AsyncOps.FieldLoadTimeout = timeout
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_FIELD_LOAD_TIMEOUT: %w", err)
		}
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_SEARCH_REFRESH_TIMEOUT"); exists {
		if timeout, err := time.ParseDuration(val); err == nil {
			config.AsyncOps.SearchRefreshTimeout = timeout
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_SEARCH_REFRESH_TIMEOUT: %w", err)
		}
	}

	// Load UI settings
	if val, exists := os.LookupEnv("ELASTIC_VIEW_SHOW_ROW_NUMBERS"); exists {
		config.UI.ShowRowNumbers = strings.ToLower(val) == "true"
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_FIELD_LIST_VISIBLE"); exists {
		config.UI.FieldListVisible = strings.ToLower(val) == "true"
	}

	// Load rate limit settings
	if val, exists := os.LookupEnv("ELASTIC_VIEW_MAX_CONCURRENT_OPS"); exists {
		if maxOps, err := strconv.Atoi(val); err == nil {
			config.RateLimit.MaxConcurrentOps = maxOps
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_MAX_CONCURRENT_OPS: %w", err)
		}
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_INITIAL_RETRY_DELAY"); exists {
		if delay, err := time.ParseDuration(val); err == nil {
			config.RateLimit.InitialRetryDelay = delay
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_INITIAL_RETRY_DELAY: %w", err)
		}
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_RETRY_MULTIPLIER"); exists {
		if multiplier, err := strconv.ParseFloat(val, 64); err == nil {
			config.RateLimit.RetryMultiplier = multiplier
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_RETRY_MULTIPLIER: %w", err)
		}
	}

	// Load field settings
	if val, exists := os.LookupEnv("ELASTIC_VIEW_FIELD_CACHE_TIMEOUT"); exists {
		if timeout, err := time.ParseDuration(val); err == nil {
			config.Fields.CacheTimeout = timeout
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_FIELD_CACHE_TIMEOUT: %w", err)
		}
	}

	if val, exists := os.LookupEnv("ELASTIC_VIEW_MAX_CACHED_FIELDS"); exists {
		if maxFields, err := strconv.Atoi(val); err == nil {
			config.Fields.MaxCachedFields = maxFields
		} else {
			return fmt.Errorf("invalid ELASTIC_VIEW_MAX_CACHED_FIELDS: %w", err)
		}
	}

	return cm.Validate()
}

// Validate ensures the configuration values are valid
func (cm *ConfigManager) Validate() error {
	config := cm.config

	// Validate pagination
	if config.Pagination.DefaultPageSize < 1 {
		return fmt.Errorf("pagination.default_page_size must be >= 1, got %d", config.Pagination.DefaultPageSize)
	}
	if config.Pagination.MaxPageSize < config.Pagination.DefaultPageSize {
		return fmt.Errorf("pagination.max_page_size (%d) must be >= default_page_size (%d)",
			config.Pagination.MaxPageSize, config.Pagination.DefaultPageSize)
	}
	if config.Pagination.MinPageSize > config.Pagination.DefaultPageSize {
		return fmt.Errorf("pagination.min_page_size (%d) must be <= default_page_size (%d)",
			config.Pagination.MinPageSize, config.Pagination.DefaultPageSize)
	}

	// Validate search settings
	if config.Search.DefaultNumResults < 1 {
		return fmt.Errorf("search.default_num_results must be >= 1, got %d", config.Search.DefaultNumResults)
	}
	if config.Search.MaxResults < config.Search.DefaultNumResults {
		return fmt.Errorf("search.max_results (%d) must be >= default_num_results (%d)",
			config.Search.MaxResults, config.Search.DefaultNumResults)
	}
	if config.Search.MaxRetries < 1 {
		return fmt.Errorf("search.max_retries must be >= 1, got %d", config.Search.MaxRetries)
	}

	// Validate timeframe
	validTimeframes := make(map[string]bool)
	for _, tf := range config.Search.ValidTimeframes {
		validTimeframes[tf] = true
	}
	if !validTimeframes[config.Search.DefaultTimeframe] {
		return fmt.Errorf("search.default_timeframe '%s' is not in valid_timeframes %v",
			config.Search.DefaultTimeframe, config.Search.ValidTimeframes)
	}

	// Validate timeouts
	if config.AsyncOps.DefaultTimeout <= 0 {
		return fmt.Errorf("async_ops.default_timeout must be > 0, got %v", config.AsyncOps.DefaultTimeout)
	}
	if config.AsyncOps.DocumentFetchTimeout <= 0 {
		return fmt.Errorf("async_ops.document_fetch_timeout must be > 0, got %v", config.AsyncOps.DocumentFetchTimeout)
	}
	if config.AsyncOps.FieldLoadTimeout <= 0 {
		return fmt.Errorf("async_ops.field_load_timeout must be > 0, got %v", config.AsyncOps.FieldLoadTimeout)
	}

	// Validate rate limit settings
	if config.RateLimit.MaxConcurrentOps < 1 {
		return fmt.Errorf("rate_limit.max_concurrent_ops must be >= 1, got %d", config.RateLimit.MaxConcurrentOps)
	}
	if config.RateLimit.RetryMultiplier <= 1.0 {
		return fmt.Errorf("rate_limit.retry_multiplier must be > 1.0, got %f", config.RateLimit.RetryMultiplier)
	}

	// Validate field settings
	if config.Fields.MaxCachedFields < 1 {
		return fmt.Errorf("fields.max_cached_fields must be >= 1, got %d", config.Fields.MaxCachedFields)
	}

	return nil
}

// GetConfig returns the current configuration
func (cm *ConfigManager) GetConfig() *ElasticViewConfig {
	return cm.config
}

// UpdateConfig allows runtime updates to configuration with validation
func (cm *ConfigManager) UpdateConfig(updateFn func(*ElasticViewConfig)) error {
	// Create a copy for validation
	configCopy := *cm.config
	updateFn(&configCopy)

	// Validate the updated config
	tempManager := &ConfigManager{config: &configCopy}
	if err := tempManager.Validate(); err != nil {
		return fmt.Errorf("configuration update validation failed: %w", err)
	}

	// Apply the validated changes
	cm.config = &configCopy
	return nil
}

// IsValidTimeframe checks if a timeframe is valid
func (cm *ConfigManager) IsValidTimeframe(timeframe string) bool {
	for _, valid := range cm.config.Search.ValidTimeframes {
		if valid == timeframe {
			return true
		}
	}
	return false
}

// GetPageSizeInRange returns a page size within the configured range
func (cm *ConfigManager) GetPageSizeInRange(requested int) int {
	if requested < cm.config.Pagination.MinPageSize {
		return cm.config.Pagination.MinPageSize
	}
	if requested > cm.config.Pagination.MaxPageSize {
		return cm.config.Pagination.MaxPageSize
	}
	return requested
}

// ShouldUseScrollAPI determines if the scroll API should be used based on result count
func (cm *ConfigManager) ShouldUseScrollAPI(numResults int) bool {
	return numResults > cm.config.Search.LargeResultLimit
}

// Global configuration instance
var globalConfigManager *ConfigManager

// InitializeConfig initializes the global configuration
func InitializeConfig() error {
	globalConfigManager = NewConfigManager()
	return globalConfigManager.LoadFromEnvironment()
}

// GetGlobalConfig returns the global configuration instance
func GetGlobalConfig() *ElasticViewConfig {
	if globalConfigManager == nil {
		// Initialize with defaults if not already initialized
		globalConfigManager = NewConfigManager()
		_ = globalConfigManager.LoadFromEnvironment() // Ignore errors for default initialization
	}
	return globalConfigManager.GetConfig()
}

// GetGlobalConfigManager returns the global configuration manager
func GetGlobalConfigManager() *ConfigManager {
	if globalConfigManager == nil {
		globalConfigManager = NewConfigManager()
		_ = globalConfigManager.LoadFromEnvironment()
	}
	return globalConfigManager
}

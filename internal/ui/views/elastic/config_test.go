package elastic

import (
	"os"
	"testing"
	"time"
)

func TestNewConfigManager(t *testing.T) {
	cm := NewConfigManager()
	if cm == nil {
		t.Fatal("ConfigManager should not be nil")
	}

	config := cm.GetConfig()
	if config == nil {
		t.Fatal("Config should not be nil")
	}

	// Test default values
	if config.Pagination.DefaultPageSize != 50 {
		t.Errorf("Expected default page size 50, got %d", config.Pagination.DefaultPageSize)
	}

	if config.Search.DefaultTimeframe != "today" {
		t.Errorf("Expected default timeframe 'today', got '%s'", config.Search.DefaultTimeframe)
	}

	if config.Search.DefaultNumResults != 1000 {
		t.Errorf("Expected default num results 1000, got %d", config.Search.DefaultNumResults)
	}

	if config.AsyncOps.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.AsyncOps.DefaultTimeout)
	}
}

func TestDefaultConfigValidation(t *testing.T) {
	cm := NewConfigManager()
	if err := cm.Validate(); err != nil {
		t.Fatalf("Default configuration should be valid: %v", err)
	}
}

func TestLoadFromEnvironment_PaginationSettings(t *testing.T) {
	// Set environment variables
	os.Setenv("ELASTIC_VIEW_PAGE_SIZE", "25")
	os.Setenv("ELASTIC_VIEW_MAX_PAGE_SIZE", "500")
	os.Setenv("ELASTIC_VIEW_MIN_PAGE_SIZE", "5")
	defer func() {
		os.Unsetenv("ELASTIC_VIEW_PAGE_SIZE")
		os.Unsetenv("ELASTIC_VIEW_MAX_PAGE_SIZE")
		os.Unsetenv("ELASTIC_VIEW_MIN_PAGE_SIZE")
	}()

	cm := NewConfigManager()
	err := cm.LoadFromEnvironment()
	if err != nil {
		t.Fatalf("LoadFromEnvironment failed: %v", err)
	}

	config := cm.GetConfig()
	if config.Pagination.DefaultPageSize != 25 {
		t.Errorf("Expected page size 25, got %d", config.Pagination.DefaultPageSize)
	}
	if config.Pagination.MaxPageSize != 500 {
		t.Errorf("Expected max page size 500, got %d", config.Pagination.MaxPageSize)
	}
	if config.Pagination.MinPageSize != 5 {
		t.Errorf("Expected min page size 5, got %d", config.Pagination.MinPageSize)
	}
}

func TestLoadFromEnvironment_SearchSettings(t *testing.T) {
	os.Setenv("ELASTIC_VIEW_DEFAULT_TIMEFRAME", "week")
	os.Setenv("ELASTIC_VIEW_NUM_RESULTS", "2000")
	os.Setenv("ELASTIC_VIEW_MAX_RESULTS", "100000")
	os.Setenv("ELASTIC_VIEW_MAX_RETRIES", "5")
	defer func() {
		os.Unsetenv("ELASTIC_VIEW_DEFAULT_TIMEFRAME")
		os.Unsetenv("ELASTIC_VIEW_NUM_RESULTS")
		os.Unsetenv("ELASTIC_VIEW_MAX_RESULTS")
		os.Unsetenv("ELASTIC_VIEW_MAX_RETRIES")
	}()

	cm := NewConfigManager()
	err := cm.LoadFromEnvironment()
	if err != nil {
		t.Fatalf("LoadFromEnvironment failed: %v", err)
	}

	config := cm.GetConfig()
	if config.Search.DefaultTimeframe != "week" {
		t.Errorf("Expected timeframe 'week', got '%s'", config.Search.DefaultTimeframe)
	}
	if config.Search.DefaultNumResults != 2000 {
		t.Errorf("Expected num results 2000, got %d", config.Search.DefaultNumResults)
	}
	if config.Search.MaxResults != 100000 {
		t.Errorf("Expected max results 100000, got %d", config.Search.MaxResults)
	}
	if config.Search.MaxRetries != 5 {
		t.Errorf("Expected max retries 5, got %d", config.Search.MaxRetries)
	}
}

func TestLoadFromEnvironment_TimeoutSettings(t *testing.T) {
	os.Setenv("ELASTIC_VIEW_DEFAULT_TIMEOUT", "45s")
	os.Setenv("ELASTIC_VIEW_DOC_FETCH_TIMEOUT", "20s")
	os.Setenv("ELASTIC_VIEW_FIELD_LOAD_TIMEOUT", "25s")
	defer func() {
		os.Unsetenv("ELASTIC_VIEW_DEFAULT_TIMEOUT")
		os.Unsetenv("ELASTIC_VIEW_DOC_FETCH_TIMEOUT")
		os.Unsetenv("ELASTIC_VIEW_FIELD_LOAD_TIMEOUT")
	}()

	cm := NewConfigManager()
	err := cm.LoadFromEnvironment()
	if err != nil {
		t.Fatalf("LoadFromEnvironment failed: %v", err)
	}

	config := cm.GetConfig()
	if config.AsyncOps.DefaultTimeout != 45*time.Second {
		t.Errorf("Expected default timeout 45s, got %v", config.AsyncOps.DefaultTimeout)
	}
	if config.AsyncOps.DocumentFetchTimeout != 20*time.Second {
		t.Errorf("Expected doc fetch timeout 20s, got %v", config.AsyncOps.DocumentFetchTimeout)
	}
	if config.AsyncOps.FieldLoadTimeout != 25*time.Second {
		t.Errorf("Expected field load timeout 25s, got %v", config.AsyncOps.FieldLoadTimeout)
	}
}

func TestLoadFromEnvironment_UISettings(t *testing.T) {
	os.Setenv("ELASTIC_VIEW_SHOW_ROW_NUMBERS", "false")
	os.Setenv("ELASTIC_VIEW_FIELD_LIST_VISIBLE", "true")
	defer func() {
		os.Unsetenv("ELASTIC_VIEW_SHOW_ROW_NUMBERS")
		os.Unsetenv("ELASTIC_VIEW_FIELD_LIST_VISIBLE")
	}()

	cm := NewConfigManager()
	err := cm.LoadFromEnvironment()
	if err != nil {
		t.Fatalf("LoadFromEnvironment failed: %v", err)
	}

	config := cm.GetConfig()
	if config.UI.ShowRowNumbers != false {
		t.Errorf("Expected show row numbers false, got %v", config.UI.ShowRowNumbers)
	}
	if config.UI.FieldListVisible != true {
		t.Errorf("Expected field list visible true, got %v", config.UI.FieldListVisible)
	}
}

func TestLoadFromEnvironment_InvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		envVal string
	}{
		{"Invalid page size", "ELASTIC_VIEW_PAGE_SIZE", "invalid"},
		{"Invalid timeout", "ELASTIC_VIEW_DEFAULT_TIMEOUT", "invalid"},
		{"Invalid num results", "ELASTIC_VIEW_NUM_RESULTS", "not-a-number"},
		{"Invalid retry multiplier", "ELASTIC_VIEW_RETRY_MULTIPLIER", "not-a-float"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tt.envVar, tt.envVal)
			defer os.Unsetenv(tt.envVar)

			cm := NewConfigManager()
			err := cm.LoadFromEnvironment()
			if err == nil {
				t.Errorf("Expected error for invalid value '%s', got nil", tt.envVal)
			}
		})
	}
}

func TestValidation_PaginationSettings(t *testing.T) {
	tests := []struct {
		name        string
		pageSize    int
		maxPageSize int
		minPageSize int
		expectError bool
	}{
		{"Valid settings", 50, 1000, 10, false},
		{"Zero page size", 0, 1000, 10, true},
		{"Negative page size", -1, 1000, 10, true},
		{"Max less than default", 50, 25, 10, true},
		{"Min greater than default", 50, 1000, 75, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewConfigManager()
			config := cm.GetConfig()
			config.Pagination.DefaultPageSize = tt.pageSize
			config.Pagination.MaxPageSize = tt.maxPageSize
			config.Pagination.MinPageSize = tt.minPageSize

			err := cm.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}

func TestValidation_SearchSettings(t *testing.T) {
	tests := []struct {
		name        string
		numResults  int
		maxResults  int
		maxRetries  int
		timeframe   string
		expectError bool
	}{
		{"Valid settings", 1000, 50000, 3, "today", false},
		{"Zero num results", 0, 50000, 3, "today", true},
		{"Max less than default", 1000, 500, 3, "today", true},
		{"Zero retries", 1000, 50000, 0, "today", true},
		{"Invalid timeframe", 1000, 50000, 3, "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewConfigManager()
			config := cm.GetConfig()
			config.Search.DefaultNumResults = tt.numResults
			config.Search.MaxResults = tt.maxResults
			config.Search.MaxRetries = tt.maxRetries
			config.Search.DefaultTimeframe = tt.timeframe

			err := cm.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}

func TestValidation_TimeoutSettings(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		expectError bool
	}{
		{"Valid timeout", 30 * time.Second, false},
		{"Zero timeout", 0, true},
		{"Negative timeout", -1 * time.Second, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewConfigManager()
			config := cm.GetConfig()
			config.AsyncOps.DefaultTimeout = tt.timeout

			err := cm.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}

func TestUpdateConfig(t *testing.T) {
	cm := NewConfigManager()

	// Test valid update
	err := cm.UpdateConfig(func(config *ElasticViewConfig) {
		config.Pagination.DefaultPageSize = 100
		config.Search.DefaultTimeframe = "week"
	})

	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	config := cm.GetConfig()
	if config.Pagination.DefaultPageSize != 100 {
		t.Errorf("Expected page size 100, got %d", config.Pagination.DefaultPageSize)
	}
	if config.Search.DefaultTimeframe != "week" {
		t.Errorf("Expected timeframe 'week', got '%s'", config.Search.DefaultTimeframe)
	}

	// Test invalid update
	err = cm.UpdateConfig(func(config *ElasticViewConfig) {
		config.Pagination.DefaultPageSize = -1 // Invalid
	})

	if err == nil {
		t.Error("Expected validation error for invalid update")
	}

	// Original config should be unchanged
	config = cm.GetConfig()
	if config.Pagination.DefaultPageSize != 100 {
		t.Errorf("Config should not change on failed update, got %d", config.Pagination.DefaultPageSize)
	}
}

func TestIsValidTimeframe(t *testing.T) {
	cm := NewConfigManager()

	tests := []struct {
		timeframe string
		expected  bool
	}{
		{"today", true},
		{"week", true},
		{"month", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.timeframe, func(t *testing.T) {
			result := cm.IsValidTimeframe(tt.timeframe)
			if result != tt.expected {
				t.Errorf("IsValidTimeframe('%s') = %v, expected %v", tt.timeframe, result, tt.expected)
			}
		})
	}
}

func TestGetPageSizeInRange(t *testing.T) {
	cm := NewConfigManager()
	config := cm.GetConfig()
	config.Pagination.MinPageSize = 10
	config.Pagination.MaxPageSize = 1000

	tests := []struct {
		name      string
		requested int
		expected  int
	}{
		{"Within range", 50, 50},
		{"Below minimum", 5, 10},
		{"Above maximum", 2000, 1000},
		{"At minimum", 10, 10},
		{"At maximum", 1000, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cm.GetPageSizeInRange(tt.requested)
			if result != tt.expected {
				t.Errorf("GetPageSizeInRange(%d) = %d, expected %d", tt.requested, result, tt.expected)
			}
		})
	}
}

func TestShouldUseScrollAPI(t *testing.T) {
	cm := NewConfigManager()
	config := cm.GetConfig()
	config.Search.LargeResultLimit = 10000

	tests := []struct {
		name       string
		numResults int
		expected   bool
	}{
		{"Small result set", 1000, false},
		{"At limit", 10000, false},
		{"Above limit", 10001, true},
		{"Large result set", 50000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cm.ShouldUseScrollAPI(tt.numResults)
			if result != tt.expected {
				t.Errorf("ShouldUseScrollAPI(%d) = %v, expected %v", tt.numResults, result, tt.expected)
			}
		})
	}
}

func TestGlobalConfig(t *testing.T) {
	// Reset global state
	globalConfigManager = nil

	// Test initialization
	config := GetGlobalConfig()
	if config == nil {
		t.Fatal("Global config should not be nil")
	}

	// Test that subsequent calls return the same instance
	config2 := GetGlobalConfig()
	if config2 != config {
		t.Error("Global config should return the same instance")
	}

	// Test global config manager
	manager := GetGlobalConfigManager()
	if manager == nil {
		t.Fatal("Global config manager should not be nil")
	}

	manager2 := GetGlobalConfigManager()
	if manager2 != manager {
		t.Error("Global config manager should return the same instance")
	}
}

func TestInitializeConfig(t *testing.T) {
	// Reset global state
	globalConfigManager = nil

	// Test initialization
	err := InitializeConfig()
	if err != nil {
		t.Fatalf("InitializeConfig failed: %v", err)
	}

	// Verify global config is available
	config := GetGlobalConfig()
	if config == nil {
		t.Fatal("Global config should be available after initialization")
	}
}

func TestConfigStructDefaults(t *testing.T) {
	config := getDefaultConfig()

	// Test that all default field orders contain expected fields
	expectedFields := []string{"@timestamp", "message"}
	for _, field := range expectedFields {
		found := false
		for _, defaultField := range config.Fields.DefaultFieldOrder {
			if defaultField == field {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected field '%s' in default field order", field)
		}
	}

	// Test that auto-select fields are reasonable
	if len(config.Fields.AutoSelectFields) == 0 {
		t.Error("Expected at least one auto-select field")
	}

	// Test valid timeframes include common ones
	expectedTimeframes := []string{"today", "week", "month"}
	for _, expected := range expectedTimeframes {
		found := false
		for _, valid := range config.Search.ValidTimeframes {
			if valid == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected timeframe '%s' in valid timeframes", expected)
		}
	}
}

// Benchmark tests
func BenchmarkConfigValidation(b *testing.B) {
	cm := NewConfigManager()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cm.Validate()
	}
}

func BenchmarkConfigUpdate(b *testing.B) {
	cm := NewConfigManager()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cm.UpdateConfig(func(config *ElasticViewConfig) {
			config.Pagination.DefaultPageSize = 50 + (i % 10)
		})
	}
}

func BenchmarkGetGlobalConfig(b *testing.B) {
	// Initialize once
	_ = InitializeConfig()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = GetGlobalConfig()
	}
}

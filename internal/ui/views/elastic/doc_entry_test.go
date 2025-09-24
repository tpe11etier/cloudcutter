package elastic_test

import (
	"sync"
	"testing"

	"github.com/tpelletiersophos/cloudcutter/internal/ui/views/elastic"
)

func TestNewDocEntry(t *testing.T) {
	tests := []struct {
		name     string
		source   []byte
		id       string
		index    string
		typeName string
		score    *float64
		version  *int64
		expectID string
		isError  bool
	}{
		{
			name:     "Valid JSON",
			source:   []byte(`{"field1":"value1","field2":42}`),
			id:       "1",
			index:    "test-index",
			typeName: "test-type",
			score:    func() *float64 { v := 1.23; return &v }(),
			version:  func() *int64 { v := int64(1); return &v }(),
			expectID: "1",
			isError:  false,
		},
		{
			name:     "Invalid JSON",
			source:   []byte(`{"invalid-json`),
			id:       "1",
			index:    "test-index",
			typeName: "test-type",
			isError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := elastic.NewDocEntry(tt.source, tt.id, tt.index, tt.typeName, tt.score, tt.version)
			if tt.isError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if doc.ID != tt.expectID {
				t.Errorf("Expected ID %s, got %s", tt.expectID, doc.ID)
			}
		})
	}
}

func TestGetAvailableFields(t *testing.T) {
	tests := []struct {
		name         string
		source       []byte
		expectFields []string
	}{
		{
			name:         "Nested and List Fields",
			source:       []byte(`{"nested":{"innerField":"value"},"list":["item1","item2"]}`),
			expectFields: []string{"_id", "_index", "_type", "nested.innerField", "list"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := elastic.NewDocEntry(tt.source, "1", "index", "type", nil, nil)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			fields := doc.GetAvailableFields()
			// Check lengths first
			if len(fields) != len(tt.expectFields) {
				t.Errorf("Expected %v fields, got %v", len(tt.expectFields), len(fields))
			}

			// Check all expected fields exist
			expectedSet := make(map[string]struct{})
			for _, f := range tt.expectFields {
				expectedSet[f] = struct{}{}
			}

			for _, field := range fields {
				if _, ok := expectedSet[field]; !ok {
					t.Errorf("Unexpected field: %s", field)
				}
			}
		})
	}
}

func TestGetValue(t *testing.T) {
	tests := []struct {
		name      string
		source    []byte
		path      string
		expectVal any
	}{
		{
			name:      "Simple Field",
			source:    []byte(`{"field1":"value1","nested":{"inner":"value2"},"list":[{"key":"value3"}]}`),
			path:      "field1",
			expectVal: "value1",
		},
		{
			name:      "Nested Field",
			source:    []byte(`{"field1":"value1","nested":{"inner":"value2"},"list":[{"key":"value3"}]}`),
			path:      "nested.inner",
			expectVal: "value2",
		},
		{
			name:      "List Field",
			source:    []byte(`{"field1":"value1","nested":{"inner":"value2"},"list":[{"key":"value3"}]}`),
			path:      "list[0].key",
			expectVal: "value3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := elastic.NewDocEntry(tt.source, "1", "index", "type", nil, nil)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			value := doc.GetValue(tt.path)
			if value != tt.expectVal {
				t.Errorf("Expected value %v, got %v", tt.expectVal, value)
			}
		})
	}
}

func TestGetFormattedValue(t *testing.T) {
	tests := []struct {
		name        string
		source      []byte
		field       string
		expectValue string
	}{
		{
			name:        "Integer Field",
			source:      []byte(`{"field1":12345,"severity":5}`),
			field:       "field1",
			expectValue: "12345",
		},
		{
			name:        "Severity Field",
			source:      []byte(`{"field1":12345,"severity":5}`),
			field:       "severity",
			expectValue: "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := elastic.NewDocEntry(tt.source, "1", "index", "type", nil, nil)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			value := doc.GetFormattedValue(tt.field)
			if value != tt.expectValue {
				t.Errorf("Expected value %s, got %s", tt.expectValue, value)
			}
		})
	}
}

func TestDocEntry_ConcurrentAccess(t *testing.T) {
	doc, err := elastic.NewDocEntry(
		[]byte(`{"field1":"value1","nested":{"inner":"value2"},"list":[{"key":"value3"}]}`),
		"1", "index", "type", nil, nil,
	)
	if err != nil {
		t.Fatalf("Failed to create DocEntry: %v", err)
	}

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			value := doc.GetValue("nested.inner")
			if value != "value2" {
				t.Errorf("Goroutine %d: Expected value 'value2', got %v", i, value)
			}
		}(i)
	}

	wg.Wait()
}

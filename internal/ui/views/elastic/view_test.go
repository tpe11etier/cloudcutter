package elastic

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestConcurrentFiltering(t *testing.T) {
	// Create mock documents
	mockDocs := []*DocEntry{
		{ID: "1", data: map[string]any{"field": "value1"}},
		{ID: "2", data: map[string]any{"field": "value2"}},
	}

	fieldCache := NewFieldCache()
	fieldState := NewFieldState(fieldCache)

	view := &View{
		state: State{
			data: DataState{
				fieldCache:       fieldCache,
				fieldState:       fieldState,
				filteredResults:  mockDocs,
				displayedResults: []*DocEntry{},
				currentFilter:    "",
			},
			pagination: PaginationState{
				pageSize: 50,
			},
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			view.state.mu.Lock()
			results := make([]*DocEntry, len(view.state.data.filteredResults))
			copy(results, view.state.data.filteredResults)
			view.state.mu.Unlock()

			if len(results) < 1 {
				t.Error("Expected results to be present")
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - possible deadlock")
	}
}

func TestConcurrentFieldStateAccess(t *testing.T) {
	fieldCache := NewFieldCache()
	fieldState := NewFieldState(fieldCache)

	// Create mock document with fields
	mockDoc := &DocEntry{
		ID: "test",
		data: map[string]any{
			"field1": "value1",
			"field2": "value2",
			"field3": "value3",
		},
	}

	// Update field state with mock document
	fieldState.UpdateFromDocuments([]*DocEntry{mockDoc})

	view := &View{
		state: State{
			data: DataState{
				fieldCache: fieldCache,
				fieldState: fieldState,
			},
		},
	}

	// Run multiple goroutines trying to select/unselect fields
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			field := fmt.Sprintf("field%d", (i%3)+1)
			if i%2 == 0 {
				view.state.data.fieldState.SelectField(field)
			} else {
				view.state.data.fieldState.UnselectField(field)
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - possible deadlock")
	}
}

func TestConcurrentFieldMovement(t *testing.T) {
	fieldCache := NewFieldCache()
	fieldState := NewFieldState(fieldCache)

	// Create mock document
	mockDoc := &DocEntry{
		ID: "test",
		data: map[string]any{
			"field1": "value1",
			"field2": "value2",
			"field3": "value3",
		},
	}

	fieldState.UpdateFromDocuments([]*DocEntry{mockDoc})

	// Select all fields initially
	for _, field := range []string{"field1", "field2", "field3"} {
		fieldState.SelectField(field)
	}

	view := &View{
		state: State{
			data: DataState{
				fieldCache: fieldCache,
				fieldState: fieldState,
			},
		},
	}

	// Run multiple goroutines trying to move fields
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			field := fmt.Sprintf("field%d", (i%3)+1)
			moveUp := i%2 == 0
			view.state.data.fieldState.MoveField(field, moveUp)
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - possible deadlock")
	}

	// Verify we still have all fields selected
	selectedFields := view.state.data.fieldState.GetOrderedSelectedFields()
	if len(selectedFields) != 3 {
		t.Errorf("Expected 3 selected fields, got %d", len(selectedFields))
	}
}

func TestViewFiltering(t *testing.T) {
	tests := []struct {
		name        string
		filters     []string
		timeframe   string
		mockDocs    []*DocEntry
		wantErr     bool
		wantResults int
	}{
		{
			name:      "Basic filter",
			filters:   []string{"status=active"},
			timeframe: "12h",
			mockDocs: []*DocEntry{
				{
					ID: "1",
					data: map[string]any{
						"status": "active",
					},
				},
				{
					ID: "2",
					data: map[string]any{
						"status": "inactive",
					},
				},
			},
			wantErr:     false,
			wantResults: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := createTestView(t)

			// Set up field metadata
			fieldCache := view.state.data.fieldCache
			for _, doc := range tt.mockDocs {
				for field := range doc.data {
					fieldCache.Set(field, &FieldMetadata{
						Type:         "keyword",
						Searchable:   true,
						Aggregatable: true,
					})
				}
			}

			// Add filters
			view.state.mu.Lock()
			for _, f := range tt.filters {
				view.addFilter(f)
			}
			view.state.mu.Unlock()

			view.components.timeframeInput.SetText(tt.timeframe)

			// TODO: Mock search service and verify results
		})
	}
}

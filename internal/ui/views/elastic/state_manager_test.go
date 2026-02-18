package elastic

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/spinner"
)

// Mock logger for testing
type mockLogger struct {
	logs []logEntry
	mu   sync.Mutex
}

type logEntry struct {
	level string
	msg   string
	args  []interface{}
}

func (m *mockLogger) Info(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, logEntry{"info", msg, args})
}

func (m *mockLogger) Error(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, logEntry{"error", msg, args})
}

func (m *mockLogger) Debug(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, logEntry{"debug", msg, args})
}

func (m *mockLogger) getLogs() []logEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	logs := make([]logEntry, len(m.logs))
	copy(logs, m.logs)
	return logs
}

func createTestStateManager() (*StateManager, *mockLogger) {
	logger := &mockLogger{}
	fieldCache := NewFieldCache()
	fieldState := NewFieldState(fieldCache)

	state := &State{
		pagination: PaginationState{
			currentPage: 1,
			pageSize:    50,
			totalPages:  1,
		},
		ui: UIState{
			showRowNumbers:   true,
			isLoading:        false,
			fieldListFilter:  "",
			fieldListVisible: false,
		},
		data: DataState{
			fieldCache:       fieldCache,
			fieldState:       fieldState,
			filters:          []string{},
			currentFilter:    "",
			currentResults:   []*DocEntry{},
			filteredResults:  []*DocEntry{},
			displayedResults: []*DocEntry{},
			columnCache:      make(map[string][]string),
		},
		search: SearchState{
			currentIndex:    "test-index",
			matchingIndices: []string{"test-index"},
			numResults:      1000,
			timeframe:       "today",
		},
		misc: MiscState{
			visibleRows:       0,
			lastDisplayHeight: 0,
			spinner:           nil,
			rateLimit:         NewRateLimiter(),
		},
	}

	return NewStateManager(state, logger), logger
}

func TestNewStateManager(t *testing.T) {
	sm, logger := createTestStateManager()

	if sm == nil {
		t.Fatal("StateManager should not be nil")
	}

	if sm.state == nil {
		t.Fatal("State should not be nil")
	}

	if len(sm.hooks) != 0 {
		t.Error("Hooks should be empty initially")
	}

	if len(sm.validators) != 0 {
		t.Error("Validators should be empty initially")
	}

	if sm.logger != logger {
		t.Error("Logger should be set correctly")
	}
}

func TestStateManager_AddUpdateHook(t *testing.T) {
	sm, _ := createTestStateManager()

	var hookCalled bool
	hook := func(operation string, oldState, newState *State) {
		hookCalled = true
	}

	sm.AddUpdateHook(hook)

	if len(sm.hooks) != 1 {
		t.Errorf("Expected 1 hook, got %d", len(sm.hooks))
	}

	// Test hook is called during update
	err := sm.SetLoading(true)
	if err != nil {
		t.Fatalf("SetLoading failed: %v", err)
	}

	if !hookCalled {
		t.Error("Hook should have been called")
	}
}

func TestStateManager_AddValidator(t *testing.T) {
	sm, _ := createTestStateManager()

	validator := func(operation string, state *State) error {
		if state.pagination.currentPage < 1 {
			return fmt.Errorf("invalid page")
		}
		return nil
	}

	sm.AddValidator(validator)

	if len(sm.validators) != 1 {
		t.Errorf("Expected 1 validator, got %d", len(sm.validators))
	}

	// Test validator is called and prevents invalid updates
	err := sm.UpdateState("test_invalid", func(s *State) error {
		s.pagination.currentPage = 0 // Invalid
		return nil
	})

	if err == nil {
		t.Error("Expected validation error")
	}
}

func TestStateManager_UpdateState(t *testing.T) {
	sm, logger := createTestStateManager()

	var hookCalled bool
	var hookOldState, hookNewState *State
	var hookOperation string

	sm.AddUpdateHook(func(operation string, oldState, newState *State) {
		hookCalled = true
		hookOperation = operation
		hookOldState = oldState
		hookNewState = newState
	})

	// Test successful update
	err := sm.UpdateState("test_update", func(s *State) error {
		s.ui.isLoading = true
		return nil
	})

	if err != nil {
		t.Fatalf("UpdateState failed: %v", err)
	}

	if !hookCalled {
		t.Error("Hook should have been called")
	}

	if hookOperation != "test_update" {
		t.Errorf("Expected operation 'test_update', got '%s'", hookOperation)
	}

	if hookOldState.ui.isLoading {
		t.Error("Old state should have isLoading = false")
	}

	if !hookNewState.ui.isLoading {
		t.Error("New state should have isLoading = true")
	}

	// Verify the actual state was updated
	uiState := sm.GetUIState()
	if !uiState.isLoading {
		t.Error("State should be updated to isLoading = true")
	}

	// Check logs
	logs := logger.getLogs()
	found := false
	for _, log := range logs {
		if log.level == "debug" && log.msg == "State updated successfully" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected debug log for successful state update")
	}
}

func TestStateManager_UpdateState_Error(t *testing.T) {
	sm, logger := createTestStateManager()

	// Test update function error
	err := sm.UpdateState("test_error", func(s *State) error {
		return fmt.Errorf("update failed")
	})

	if err == nil {
		t.Error("Expected error from update function")
	}

	// Check error logs
	logs := logger.getLogs()
	found := false
	for _, log := range logs {
		if log.level == "error" && log.msg == "State update failed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error log for failed state update")
	}
}

func TestStateManager_ReadState(t *testing.T) {
	sm, _ := createTestStateManager()

	var readValue bool
	err := sm.ReadState(func(s *State) error {
		readValue = s.ui.isLoading
		return nil
	})

	if err != nil {
		t.Fatalf("ReadState failed: %v", err)
	}

	if readValue {
		t.Error("Expected isLoading to be false")
	}
}

func TestStateManager_GetSnapshot(t *testing.T) {
	sm, _ := createTestStateManager()

	snapshot := sm.GetSnapshot()

	if snapshot == nil {
		t.Fatal("Snapshot should not be nil")
	}

	// Verify it's a deep copy by modifying original
	sm.SetLoading(true)

	if snapshot.ui.isLoading {
		t.Error("Snapshot should not be affected by changes to original state")
	}
}

func TestStateManager_PaginationOperations(t *testing.T) {
	sm, _ := createTestStateManager()

	// Test GetPagination
	pagination := sm.GetPagination()
	if pagination.currentPage != 1 {
		t.Errorf("Expected currentPage 1, got %d", pagination.currentPage)
	}

	// Test UpdatePagination
	err := sm.UpdatePagination(2, 10, 25)
	if err != nil {
		t.Fatalf("UpdatePagination failed: %v", err)
	}

	pagination = sm.GetPagination()
	if pagination.currentPage != 2 || pagination.totalPages != 10 || pagination.pageSize != 25 {
		t.Errorf("Pagination not updated correctly: %+v", pagination)
	}

	// Test validation
	err = sm.UpdatePagination(0, 10, 25) // Invalid currentPage
	if err == nil {
		t.Error("Expected validation error for currentPage = 0")
	}

	// Test SetCurrentPage
	err = sm.SetCurrentPage(5)
	if err != nil {
		t.Fatalf("SetCurrentPage failed: %v", err)
	}

	pagination = sm.GetPagination()
	if pagination.currentPage != 5 {
		t.Errorf("Expected currentPage 5, got %d", pagination.currentPage)
	}

	// Test validation for SetCurrentPage
	err = sm.SetCurrentPage(15) // Exceeds totalPages
	if err == nil {
		t.Error("Expected validation error for page exceeding totalPages")
	}
}

func TestStateManager_UIOperations(t *testing.T) {
	sm, _ := createTestStateManager()

	// Test SetLoading
	err := sm.SetLoading(true)
	if err != nil {
		t.Fatalf("SetLoading failed: %v", err)
	}

	ui := sm.GetUIState()
	if !ui.isLoading {
		t.Error("Expected isLoading = true")
	}

	// Test SetFieldListVisible
	err = sm.SetFieldListVisible(true)
	if err != nil {
		t.Fatalf("SetFieldListVisible failed: %v", err)
	}

	ui = sm.GetUIState()
	if !ui.fieldListVisible {
		t.Error("Expected fieldListVisible = true")
	}

	// Test SetFieldListFilter
	err = sm.SetFieldListFilter("test-filter")
	if err != nil {
		t.Fatalf("SetFieldListFilter failed: %v", err)
	}

	ui = sm.GetUIState()
	if ui.fieldListFilter != "test-filter" {
		t.Errorf("Expected fieldListFilter 'test-filter', got '%s'", ui.fieldListFilter)
	}

	// Test SetRowNumbers
	err = sm.SetRowNumbers(false)
	if err != nil {
		t.Fatalf("SetRowNumbers failed: %v", err)
	}

	ui = sm.GetUIState()
	if ui.showRowNumbers {
		t.Error("Expected showRowNumbers = false")
	}
}

func TestStateManager_SearchOperations(t *testing.T) {
	sm, _ := createTestStateManager()

	// Test SetCurrentIndex
	err := sm.SetCurrentIndex("new-index")
	if err != nil {
		t.Fatalf("SetCurrentIndex failed: %v", err)
	}

	search := sm.GetSearchState()
	if search.currentIndex != "new-index" {
		t.Errorf("Expected currentIndex 'new-index', got '%s'", search.currentIndex)
	}

	// Test validation for empty index
	err = sm.SetCurrentIndex("")
	if err == nil {
		t.Error("Expected validation error for empty index")
	}

	// Test SetTimeframe
	err = sm.SetTimeframe("last-hour")
	if err != nil {
		t.Fatalf("SetTimeframe failed: %v", err)
	}

	search = sm.GetSearchState()
	if search.timeframe != "last-hour" {
		t.Errorf("Expected timeframe 'last-hour', got '%s'", search.timeframe)
	}

	// Test SetMatchingIndices
	indices := []string{"index1", "index2", "index3"}
	err = sm.SetMatchingIndices(indices)
	if err != nil {
		t.Fatalf("SetMatchingIndices failed: %v", err)
	}

	search = sm.GetSearchState()
	if len(search.matchingIndices) != 3 {
		t.Errorf("Expected 3 indices, got %d", len(search.matchingIndices))
	}

	// Test SetIndexStats
	stats := &elastic.IndexStats{DocsCount: "1000"}
	err = sm.SetIndexStats(stats)
	if err != nil {
		t.Fatalf("SetIndexStats failed: %v", err)
	}

	search = sm.GetSearchState()
	if search.indexStats == nil || search.indexStats.DocsCount != "1000" {
		t.Error("IndexStats not set correctly")
	}

	// Test SetCancelFunc
	ctx, cancel := context.WithCancel(context.Background())
	err = sm.SetCancelFunc(cancel)
	if err != nil {
		t.Fatalf("SetCancelFunc failed: %v", err)
	}

	search = sm.GetSearchState()
	if search.cancelCurrentOp == nil {
		t.Error("CancelFunc not set")
	}
	ctx.Done() // Clean up
}

func TestStateManager_DataOperations(t *testing.T) {
	sm, _ := createTestStateManager()

	// Test AddFilter
	err := sm.AddFilter("test-filter")
	if err != nil {
		t.Fatalf("AddFilter failed: %v", err)
	}

	data := sm.GetDataState()
	if len(data.filters) != 1 || data.filters[0] != "test-filter" {
		t.Error("Filter not added correctly")
	}

	// Test duplicate filter prevention
	err = sm.AddFilter("test-filter")
	if err == nil {
		t.Error("Expected error for duplicate filter")
	}

	// Test empty filter validation
	err = sm.AddFilter("")
	if err == nil {
		t.Error("Expected error for empty filter")
	}

	// Add more filters for testing removal
	sm.AddFilter("filter2")
	sm.AddFilter("filter3")

	// Test RemoveFilterByIndex
	err = sm.RemoveFilterByIndex(1) // Remove "filter2"
	if err != nil {
		t.Fatalf("RemoveFilterByIndex failed: %v", err)
	}

	data = sm.GetDataState()
	if len(data.filters) != 2 || data.filters[1] != "filter3" {
		t.Error("Filter not removed correctly")
	}

	// Test invalid index
	err = sm.RemoveFilterByIndex(10)
	if err == nil {
		t.Error("Expected error for invalid filter index")
	}

	// Test ClearFilters
	err = sm.ClearFilters()
	if err != nil {
		t.Fatalf("ClearFilters failed: %v", err)
	}

	data = sm.GetDataState()
	if len(data.filters) != 0 {
		t.Error("Filters not cleared")
	}

	// Test SetFieldActive
	err = sm.SetFieldActive("test-field", true)
	if err != nil {
		t.Fatalf("SetFieldActive failed: %v", err)
	}

	// Test empty field validation
	err = sm.SetFieldActive("", true)
	if err == nil {
		t.Error("Expected error for empty field name")
	}
}

func TestStateManager_MiscOperations(t *testing.T) {
	sm, _ := createTestStateManager()

	// Test SetSpinner
	testSpinner := spinner.NewSpinner("Loading...")
	err := sm.SetSpinner(testSpinner)
	if err != nil {
		t.Fatalf("SetSpinner failed: %v", err)
	}

	misc := sm.GetMiscState()
	if misc.spinner != testSpinner {
		t.Error("Spinner not set correctly")
	}

	// Test SetVisibleRows
	err = sm.SetVisibleRows(25)
	if err != nil {
		t.Fatalf("SetVisibleRows failed: %v", err)
	}

	misc = sm.GetMiscState()
	if misc.visibleRows != 25 {
		t.Errorf("Expected visibleRows 25, got %d", misc.visibleRows)
	}

	// Test negative validation
	err = sm.SetVisibleRows(-1)
	if err == nil {
		t.Error("Expected error for negative visible rows")
	}
}

func TestStateManager_BulkOperations(t *testing.T) {
	sm, _ := createTestStateManager()

	// Create test entries
	entries := []*DocEntry{
		{ID: "1", Index: "test", data: map[string]any{"field1": "value1"}},
		{ID: "2", Index: "test", data: map[string]any{"field2": "value2"}},
		{ID: "3", Index: "test", data: map[string]any{"field3": "value3"}},
	}

	// Test UpdateSearchResults
	err := sm.UpdateSearchResults(entries, 100, 20)
	if err != nil {
		t.Fatalf("UpdateSearchResults failed: %v", err)
	}

	data := sm.GetDataState()
	if len(data.filteredResults) != 3 {
		t.Errorf("Expected 3 filtered results, got %d", len(data.filteredResults))
	}

	if len(data.displayedResults) != 3 {
		t.Errorf("Expected 3 displayed results, got %d", len(data.displayedResults))
	}

	pagination := sm.GetPagination()
	if pagination.totalPages != 1 {
		t.Errorf("Expected 1 total page, got %d", pagination.totalPages)
	}

	if pagination.currentPage != 1 {
		t.Errorf("Expected current page 1, got %d", pagination.currentPage)
	}

	if pagination.pageSize != 20 {
		t.Errorf("Expected page size 20, got %d", pagination.pageSize)
	}
}

func TestStateManager_ConcurrentAccess(t *testing.T) {
	sm, _ := createTestStateManager()

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	// Test concurrent reads and writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// Alternate between reads and writes
				if j%2 == 0 {
					err := sm.SetLoading(j%4 == 0)
					if err != nil {
						errors <- err
					}
				} else {
					_ = sm.GetUIState()
				}
			}
		}(i)
	}

	// Wait for all goroutines
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}
}

func TestStateManager_HookPanicRecovery(t *testing.T) {
	sm, logger := createTestStateManager()

	// Add a hook that panics
	sm.AddUpdateHook(func(operation string, oldState, newState *State) {
		panic("test panic")
	})

	// Update should still succeed despite hook panic
	err := sm.SetLoading(true)
	if err != nil {
		t.Fatalf("UpdateState should succeed despite hook panic: %v", err)
	}

	// Check that panic was logged
	logs := logger.getLogs()
	found := false
	for _, log := range logs {
		if log.level == "error" && log.msg == "State update hook panicked" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error log for hook panic")
	}

	// Verify state was still updated
	ui := sm.GetUIState()
	if !ui.isLoading {
		t.Error("State should be updated despite hook panic")
	}
}

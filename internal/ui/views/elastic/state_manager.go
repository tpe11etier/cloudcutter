package elastic

import (
	"context"
	"fmt"
	"sync"

	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/spinner"
)

// StateManager provides centralized, thread-safe state management with hooks and validation
type StateManager struct {
	state      *State
	mu         sync.RWMutex
	hooks      []StateUpdateHook
	validators []StateValidator
	logger     Logger
}

// StateUpdateHook is called after successful state updates
type StateUpdateHook func(operation string, oldState, newState *State)

// StateValidator validates state changes before they are applied
type StateValidator func(operation string, newState *State) error

// Logger interface for state management logging
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewStateManager creates a new centralized state manager
func NewStateManager(initialState *State, logger Logger) *StateManager {
	return &StateManager{
		state:      initialState,
		hooks:      make([]StateUpdateHook, 0),
		validators: make([]StateValidator, 0),
		logger:     logger,
	}
}

// AddUpdateHook registers a hook to be called after state updates
func (sm *StateManager) AddUpdateHook(hook StateUpdateHook) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.hooks = append(sm.hooks, hook)
}

// AddValidator registers a validator to be called before state updates
func (sm *StateManager) AddValidator(validator StateValidator) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.validators = append(sm.validators, validator)
}

// UpdateState performs a thread-safe state update with validation and hooks
func (sm *StateManager) UpdateState(operation string, updateFn func(*State) error) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Create a deep copy for validation
	oldState := sm.cloneState()

	// Apply the update to a copy first
	newState := sm.cloneState()
	if err := updateFn(newState); err != nil {
		sm.logger.Error("State update failed", "operation", operation, "error", err)
		return fmt.Errorf("state update failed for operation %s: %w", operation, err)
	}

	// Validate the new state
	for _, validator := range sm.validators {
		if err := validator(operation, newState); err != nil {
			sm.logger.Error("State validation failed", "operation", operation, "error", err)
			return fmt.Errorf("state validation failed for operation %s: %w", operation, err)
		}
	}

	// Apply the validated changes
	sm.state = newState
	sm.logger.Debug("State updated successfully", "operation", operation)

	// Call update hooks
	for _, hook := range sm.hooks {
		func() {
			defer func() {
				if r := recover(); r != nil {
					sm.logger.Error("State update hook panicked", "operation", operation, "panic", r)
				}
			}()
			hook(operation, oldState, sm.state)
		}()
	}

	return nil
}

// ReadState provides thread-safe read access to state
func (sm *StateManager) ReadState(readFn func(*State) error) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return readFn(sm.state)
}

// GetSnapshot returns a deep copy of the current state for safe reading
func (sm *StateManager) GetSnapshot() *State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.cloneState()
}

// Pagination state accessors and mutators
func (sm *StateManager) GetPagination() PaginationState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state.pagination
}

func (sm *StateManager) UpdatePagination(currentPage, totalPages, pageSize int) error {
	return sm.UpdateState("update_pagination", func(s *State) error {
		if currentPage < 1 {
			return fmt.Errorf("currentPage must be >= 1, got %d", currentPage)
		}
		if totalPages < 1 {
			return fmt.Errorf("totalPages must be >= 1, got %d", totalPages)
		}
		if pageSize < 1 {
			return fmt.Errorf("pageSize must be >= 1, got %d", pageSize)
		}

		s.pagination.currentPage = currentPage
		s.pagination.totalPages = totalPages
		s.pagination.pageSize = pageSize
		return nil
	})
}

func (sm *StateManager) SetCurrentPage(page int) error {
	return sm.UpdateState("set_current_page", func(s *State) error {
		if page < 1 {
			return fmt.Errorf("page must be >= 1, got %d", page)
		}
		if page > s.pagination.totalPages {
			return fmt.Errorf("page %d exceeds total pages %d", page, s.pagination.totalPages)
		}
		s.pagination.currentPage = page
		return nil
	})
}

// UI state accessors and mutators
func (sm *StateManager) GetUIState() UIState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state.ui
}

func (sm *StateManager) SetLoading(isLoading bool) error {
	return sm.UpdateState("set_loading", func(s *State) error {
		s.ui.isLoading = isLoading
		return nil
	})
}

func (sm *StateManager) SetFieldListVisible(visible bool) error {
	return sm.UpdateState("set_field_list_visible", func(s *State) error {
		s.ui.fieldListVisible = visible
		return nil
	})
}

func (sm *StateManager) SetFieldListFilter(filter string) error {
	return sm.UpdateState("set_field_list_filter", func(s *State) error {
		s.ui.fieldListFilter = filter
		return nil
	})
}

func (sm *StateManager) SetRowNumbers(show bool) error {
	return sm.UpdateState("set_row_numbers", func(s *State) error {
		s.ui.showRowNumbers = show
		return nil
	})
}

// Search state accessors and mutators
func (sm *StateManager) GetSearchState() SearchState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state.search
}

func (sm *StateManager) SetCurrentIndex(index string) error {
	return sm.UpdateState("set_current_index", func(s *State) error {
		if index == "" {
			return fmt.Errorf("index cannot be empty")
		}
		s.search.currentIndex = index
		return nil
	})
}

func (sm *StateManager) SetTimeframe(timeframe string) error {
	return sm.UpdateState("set_timeframe", func(s *State) error {
		s.search.timeframe = timeframe
		return nil
	})
}

func (sm *StateManager) SetMatchingIndices(indices []string) error {
	return sm.UpdateState("set_matching_indices", func(s *State) error {
		s.search.matchingIndices = make([]string, len(indices))
		copy(s.search.matchingIndices, indices)
		return nil
	})
}

func (sm *StateManager) SetIndexStats(stats *elastic.IndexStats) error {
	return sm.UpdateState("set_index_stats", func(s *State) error {
		s.search.indexStats = stats
		return nil
	})
}

func (sm *StateManager) SetCancelFunc(cancelFunc context.CancelFunc) error {
	return sm.UpdateState("set_cancel_func", func(s *State) error {
		s.search.cancelCurrentOp = cancelFunc
		return nil
	})
}

// Data state accessors and mutators
func (sm *StateManager) GetDataState() DataState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state.data
}

func (sm *StateManager) AddFilter(filter string) error {
	return sm.UpdateState("add_filter", func(s *State) error {
		if filter == "" {
			return fmt.Errorf("filter cannot be empty")
		}
		// Check for duplicates
		for _, existing := range s.data.filters {
			if existing == filter {
				return fmt.Errorf("filter already exists: %s", filter)
			}
		}
		s.data.filters = append(s.data.filters, filter)
		return nil
	})
}

func (sm *StateManager) RemoveFilterByIndex(index int) error {
	return sm.UpdateState("remove_filter_by_index", func(s *State) error {
		if index < 0 || index >= len(s.data.filters) {
			return fmt.Errorf("invalid filter index %d, must be between 0 and %d", index, len(s.data.filters)-1)
		}
		s.data.filters = append(s.data.filters[:index], s.data.filters[index+1:]...)
		return nil
	})
}

func (sm *StateManager) ClearFilters() error {
	return sm.UpdateState("clear_filters", func(s *State) error {
		s.data.filters = make([]string, 0)
		return nil
	})
}

func (sm *StateManager) SetResults(current, filtered, displayed []*DocEntry) error {
	return sm.UpdateState("set_results", func(s *State) error {
		s.data.currentResults = make([]*DocEntry, len(current))
		copy(s.data.currentResults, current)

		s.data.filteredResults = make([]*DocEntry, len(filtered))
		copy(s.data.filteredResults, filtered)

		s.data.displayedResults = make([]*DocEntry, len(displayed))
		copy(s.data.displayedResults, displayed)

		return nil
	})
}

func (sm *StateManager) ResetFields() error {
	return sm.UpdateState("reset_fields", func(s *State) error {
		s.data.ResetFields()
		return nil
	})
}

func (sm *StateManager) SetFieldActive(field string, active bool) error {
	return sm.UpdateState("set_field_active", func(s *State) error {
		if field == "" {
			return fmt.Errorf("field name cannot be empty")
		}
		s.data.SetFieldActive(field, active)
		return nil
	})
}

// Misc state accessors and mutators
func (sm *StateManager) GetMiscState() MiscState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state.misc
}

func (sm *StateManager) SetSpinner(spinner *spinner.Spinner) error {
	return sm.UpdateState("set_spinner", func(s *State) error {
		s.misc.spinner = spinner
		return nil
	})
}

func (sm *StateManager) SetVisibleRows(rows int) error {
	return sm.UpdateState("set_visible_rows", func(s *State) error {
		if rows < 0 {
			return fmt.Errorf("visible rows cannot be negative, got %d", rows)
		}
		s.misc.visibleRows = rows
		return nil
	})
}

// Helper methods
func (sm *StateManager) cloneState() *State {
	// Deep clone the state for safe copying
	cloned := &State{
		pagination: sm.state.pagination,
		ui:         sm.state.ui,
		search:     sm.state.search,
		misc:       sm.state.misc,
		data: DataState{
			fieldCache:    sm.state.data.fieldCache, // TODO: Implement proper cloning if needed
			fieldState:    sm.state.data.fieldState, // TODO: Implement proper cloning if needed
			currentFilter: sm.state.data.currentFilter,
			columnCache:   make(map[string][]string),
		},
	}

	// Deep copy filters
	cloned.data.filters = make([]string, len(sm.state.data.filters))
	copy(cloned.data.filters, sm.state.data.filters)

	// Deep copy results
	cloned.data.currentResults = make([]*DocEntry, len(sm.state.data.currentResults))
	copy(cloned.data.currentResults, sm.state.data.currentResults)

	cloned.data.filteredResults = make([]*DocEntry, len(sm.state.data.filteredResults))
	copy(cloned.data.filteredResults, sm.state.data.filteredResults)

	cloned.data.displayedResults = make([]*DocEntry, len(sm.state.data.displayedResults))
	copy(cloned.data.displayedResults, sm.state.data.displayedResults)

	// Deep copy column cache
	for k, v := range sm.state.data.columnCache {
		cloned.data.columnCache[k] = make([]string, len(v))
		copy(cloned.data.columnCache[k], v)
	}

	// Deep copy search indices
	cloned.search.matchingIndices = make([]string, len(sm.state.search.matchingIndices))
	copy(cloned.search.matchingIndices, sm.state.search.matchingIndices)

	return cloned
}

// Validation helpers
func (sm *StateManager) ValidatePaginationState(operation string, state *State) error {
	if state.pagination.currentPage < 1 {
		return fmt.Errorf("invalid currentPage: %d", state.pagination.currentPage)
	}
	if state.pagination.totalPages < 1 {
		return fmt.Errorf("invalid totalPages: %d", state.pagination.totalPages)
	}
	if state.pagination.pageSize < 1 {
		return fmt.Errorf("invalid pageSize: %d", state.pagination.pageSize)
	}
	if state.pagination.currentPage > state.pagination.totalPages {
		return fmt.Errorf("currentPage (%d) exceeds totalPages (%d)",
			state.pagination.currentPage, state.pagination.totalPages)
	}
	return nil
}

// Bulk operations for complex state updates
func (sm *StateManager) UpdateSearchResults(entries []*DocEntry, totalHits int, pageSize int) error {
	return sm.UpdateState("update_search_results", func(s *State) error {
		// Set results
		s.data.filteredResults = make([]*DocEntry, len(entries))
		copy(s.data.filteredResults, entries)

		s.data.displayedResults = make([]*DocEntry, len(entries))
		copy(s.data.displayedResults, entries)

		// Update pagination
		totalPages := (len(entries) + pageSize - 1) / pageSize // Ceiling division
		if totalPages < 1 {
			totalPages = 1
		}

		s.pagination.totalPages = totalPages
		s.pagination.pageSize = pageSize

		// Reset to first page for new results
		s.pagination.currentPage = 1

		return nil
	})
}

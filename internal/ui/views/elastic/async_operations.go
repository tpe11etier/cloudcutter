package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// AsyncOperations provides centralized asynchronous operations with proper error handling,
// timeout management, and UI update patterns
type AsyncOperations struct {
	view *View
}

// NewAsyncOperations creates a new async operations helper
func NewAsyncOperations(view *View) *AsyncOperations {
	return &AsyncOperations{view: view}
}

// AsyncOperation represents a generic async operation with proper lifecycle management
type AsyncOperation struct {
	view          *View
	loadingMsg    string
	timeout       time.Duration
	cleanupFuncs  []func()
	successAction func()
	errorAction   func(error)
}

// NewAsyncOperation creates a new async operation with default settings
func (ao *AsyncOperations) NewAsyncOperation(loadingMsg string) *AsyncOperation {
	return &AsyncOperation{
		view:          ao.view,
		loadingMsg:    loadingMsg,
		timeout:       30 * time.Second, // Default timeout
		cleanupFuncs:  make([]func(), 0),
		successAction: func() {},
		errorAction: func(err error) {
			ao.view.manager.App().QueueUpdateDraw(func() {
				ao.view.manager.UpdateStatusBar(fmt.Sprintf("Error: %v", err))
			})
		},
	}
}

// WithTimeout sets a custom timeout for the operation
func (op *AsyncOperation) WithTimeout(timeout time.Duration) *AsyncOperation {
	op.timeout = timeout
	return op
}

// WithCleanup adds a cleanup function to be called when the operation completes
func (op *AsyncOperation) WithCleanup(cleanup func()) *AsyncOperation {
	op.cleanupFuncs = append(op.cleanupFuncs, cleanup)
	return op
}

// WithSuccess sets the success callback
func (op *AsyncOperation) WithSuccess(success func()) *AsyncOperation {
	op.successAction = success
	return op
}

// WithError sets the error callback
func (op *AsyncOperation) WithError(errorHandler func(error)) *AsyncOperation {
	op.errorAction = errorHandler
	return op
}

// Execute runs the async operation with proper lifecycle management
func (op *AsyncOperation) Execute(operation func(ctx context.Context) error) {
	op.view.showLoading(op.loadingMsg)

	go func() {
		defer func() {
			op.view.hideLoading()
			for _, cleanup := range op.cleanupFuncs {
				cleanup()
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), op.timeout)
		defer cancel()

		if err := operation(ctx); err != nil {
			op.view.manager.Logger().Error("Async operation failed", "error", err, "operation", op.loadingMsg)
			op.errorAction(err)
			return
		}

		op.successAction()
	}()
}

// UIUpdateOperation wraps a UI update in QueueUpdateDraw
func (ao *AsyncOperations) UIUpdateOperation(update func()) {
	ao.view.manager.App().QueueUpdateDraw(update)
}

// FetchFullDocument fetches and displays a full document with enhanced error handling and timeout
func (ao *AsyncOperations) FetchFullDocument(entry *DocEntry) {
	ao.NewAsyncOperation("Fetching document...").
		WithTimeout(15 * time.Second).
		WithSuccess(func() {
			ao.UIUpdateOperation(func() {
				ao.view.showJSONModal(entry)
			})
		}).
		WithError(func(err error) {
			ao.UIUpdateOperation(func() {
				ao.view.manager.UpdateStatusBar(fmt.Sprintf("Error fetching document: %v", err))
			})
		}).
		Execute(func(ctx context.Context) error {
			res, err := ao.view.service.Client.Get(entry.Index, entry.ID)
			if err != nil {
				return fmt.Errorf("failed to fetch document: %w", err)
			}
			defer res.Body.Close()

			var fullDoc struct {
				Source map[string]any `json:"_source"`
			}
			if err := json.NewDecoder(res.Body).Decode(&fullDoc); err != nil {
				return fmt.Errorf("failed to decode document: %w", err)
			}

			entry.data = fullDoc.Source
			return nil
		})
}

// ReloadFieldsForNewIndex reloads fields when the index changes with enhanced error handling
func (ao *AsyncOperations) ReloadFieldsForNewIndex() {
	// Clear existing fields immediately
	ao.UIUpdateOperation(func() {
		ao.view.components.fieldList.Clear()
		ao.view.components.selectedList.Clear()
	})

	ao.NewAsyncOperation("Loading fields...").
		WithTimeout(20 * time.Second).
		WithSuccess(func() {
			ao.UIUpdateOperation(func() {
				ao.view.rebuildFieldList()
				ao.view.manager.UpdateStatusBar("Fields loaded successfully")
			})
			// Refresh with current timeframe after UI update
			go func() {
				ao.view.refreshWithCurrentTimeframe()
			}()
		}).
		WithError(func(err error) {
			ao.UIUpdateOperation(func() {
				ao.view.manager.UpdateStatusBar(fmt.Sprintf("Error loading fields: %v", err))
			})
		}).
		Execute(func(ctx context.Context) error {
			return ao.view.loadFields()
		})
}

// ConcurrentFieldInitialization handles the complex field initialization pattern from fields.go
func (ao *AsyncOperations) ConcurrentFieldInitialization() error {
	type initResult struct {
		indices []string
		err     error
	}

	// Use channels for coordinating multiple async operations
	resultChan := make(chan initResult, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	// Start indices loading
	wg.Add(1)
	go func() {
		defer wg.Done()
		indices, err := ao.view.service.ListIndices(ctx, "*")
		if err != nil {
			ao.view.manager.Logger().Error("Failed to list indices", "error", err)
			resultChan <- initResult{nil, fmt.Errorf("failed to list indices: %w", err)}
			return
		}

		ao.view.state.mu.Lock()
		ao.view.state.search.matchingIndices = indices
		ao.view.state.mu.Unlock()

		resultChan <- initResult{indices, nil}
	}()

	// Start field loading
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ao.view.loadFields(); err != nil {
			resultChan <- initResult{nil, fmt.Errorf("failed to load fields: %w", err)}
			return
		}

		ao.UIUpdateOperation(func() {
			ao.view.rebuildFieldList()
		})

		resultChan <- initResult{nil, nil}
	}()

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results with timeout handling
	select {
	case <-ctx.Done():
		return fmt.Errorf("field initialization timed out: %w", ctx.Err())
	case <-time.After(30 * time.Second):
		return fmt.Errorf("field initialization timed out after %v", 30*time.Second)
	default:
		// Process results
		for result := range resultChan {
			if result.err != nil {
				return result.err
			}
		}
	}

	return nil
}

// RefreshResults handles the complex search refresh pattern from search.go
func (ao *AsyncOperations) RefreshResults() {
	// Check if already loading
	ao.view.state.mu.Lock()
	if ao.view.state.ui.isLoading {
		ao.view.state.mu.Unlock()
		return
	}
	ao.view.state.ui.isLoading = true
	ao.view.state.mu.Unlock()

	currentFocus := ao.view.manager.App().GetFocus()

	ao.NewAsyncOperation("Refreshing results").
		WithTimeout(45 * time.Second).
		WithCleanup(func() {
			// Always reset loading state
			ao.view.state.mu.Lock()
			ao.view.state.ui.isLoading = false
			ao.view.state.mu.Unlock()

			// Restore focus
			ao.UIUpdateOperation(func() {
				ao.view.manager.SetFocus(currentFocus)
			})
		}).
		WithSuccess(func() {
			// Success message is handled in the operation itself
		}).
		WithError(func(err error) {
			ao.view.manager.Logger().Error("Error fetching results", "error", err)
			ao.UIUpdateOperation(func() {
				ao.view.manager.UpdateStatusBar(fmt.Sprintf("Error: %v", err))
			})
		}).
		Execute(func(ctx context.Context) error {
			searchResult, err := ao.view.fetchResults()
			if err != nil {
				return fmt.Errorf("failed to fetch results: %w", err)
			}

			ao.view.updateFieldsFromResults(searchResult.entries)

			// Update results state
			ao.view.state.mu.Lock()
			ao.view.state.data.filteredResults = searchResult.entries
			ao.view.state.data.displayedResults = append([]*DocEntry(nil), searchResult.entries...)
			ao.view.state.pagination.totalPages = ao.calculateTotalPages(len(searchResult.entries))
			ao.view.state.mu.Unlock()

			// Update UI
			ao.UIUpdateOperation(func() {
				ao.view.displayCurrentPage()
				ao.view.updateHeader()
				ao.view.manager.UpdateStatusBar(fmt.Sprintf("Found %d results total (displaying %d)",
					searchResult.totalHits, len(searchResult.entries)))
			})

			return nil
		})
}

// calculateTotalPages calculates the total number of pages for pagination
func (ao *AsyncOperations) calculateTotalPages(totalResults int) int {
	pageSize := ao.view.state.pagination.pageSize
	if pageSize <= 0 {
		return 1
	}
	pages := (totalResults + pageSize - 1) / pageSize // Ceiling division
	if pages < 1 {
		return 1
	}
	return pages
}

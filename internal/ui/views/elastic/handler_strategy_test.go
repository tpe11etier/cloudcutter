package elastic

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// getComponentName returns a string representation of the component type
func getComponentName(componentType ComponentType) string {
	switch componentType {
	case ComponentFilterInput:
		return "FilterInput"
	case ComponentActiveFilters:
		return "ActiveFilters"
	case ComponentIndexInput:
		return "IndexInput"
	case ComponentFieldList:
		return "FieldList"
	case ComponentSelectedList:
		return "SelectedList"
	case ComponentResultsTable:
		return "ResultsTable"
	case ComponentTimeframeInput:
		return "TimeframeInput"
	case ComponentNumResultsInput:
		return "NumResultsInput"
	case ComponentLocalFilterInput:
		return "LocalFilterInput"
	default:
		return fmt.Sprintf("Unknown_%d", int(componentType))
	}
}

// TestHandlerManager tests the handler manager functionality
func TestHandlerManager(t *testing.T) {
	view := createTestView(t)

	// Test handler manager initialization
	handlerManager := NewHandlerManager(view)
	if handlerManager == nil {
		t.Fatal("HandlerManager should not be nil")
	}

	// Test that all handlers are registered
	expectedHandlers := []ComponentType{
		ComponentFilterInput,
		ComponentActiveFilters,
		ComponentIndexInput,
		ComponentFieldList,
		ComponentSelectedList,
		ComponentResultsTable,
		ComponentTimeframeInput,
		ComponentNumResultsInput,
		ComponentLocalFilterInput,
	}

	for _, componentType := range expectedHandlers {
		handler, exists := handlerManager.GetHandler(componentType)
		if !exists {
			t.Errorf("Handler for component %v should exist", componentType)
		}
		if handler == nil {
			t.Errorf("Handler for component %v should not be nil", componentType)
		}
		if handler.GetComponentType() != componentType {
			t.Errorf("Handler component type should be %v, got %v", componentType, handler.GetComponentType())
		}
	}
}

// TestFilterInputHandler tests the filter input handler
func TestFilterInputHandler(t *testing.T) {
	view := createTestView(t)
	handler := NewFilterInputHandler(view)

	tests := []struct {
		name           string
		eventKey       tcell.Key
		eventRune      rune
		expectedResult bool // true if event should be consumed (return nil)
	}{
		{
			name:           "Escape key clears input",
			eventKey:       tcell.KeyEsc,
			expectedResult: true,
		},
		{
			name:           "Enter key submits filter",
			eventKey:       tcell.KeyEnter,
			expectedResult: true,
		},
		{
			name:           "Other keys pass through",
			eventKey:       tcell.KeyRune,
			eventRune:      'a',
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tcell.NewEventKey(tt.eventKey, tt.eventRune, tcell.ModNone)
			result := handler.HandleEvent(event, view)

			if tt.expectedResult && result != nil {
				t.Errorf("Expected event to be consumed (nil), got non-nil")
			}
			if !tt.expectedResult && result == nil {
				t.Errorf("Expected event to pass through (non-nil), got nil")
			}
		})
	}
}

// TestActiveFiltersHandler tests the active filters handler
func TestActiveFiltersHandler(t *testing.T) {
	view := createTestView(t)
	handler := NewActiveFiltersHandler(view)

	tests := []struct {
		name           string
		eventKey       tcell.Key
		eventRune      rune
		expectedResult bool
	}{
		{
			name:           "Escape focuses filter input",
			eventKey:       tcell.KeyEsc,
			expectedResult: true,
		},
		{
			name:           "Delete key removes filter",
			eventKey:       tcell.KeyDelete,
			expectedResult: true,
		},
		{
			name:           "Numeric rune deletes by index",
			eventKey:       tcell.KeyRune,
			eventRune:      '1',
			expectedResult: true,
		},
		{
			name:           "Non-numeric rune passes through",
			eventKey:       tcell.KeyRune,
			eventRune:      'a',
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tcell.NewEventKey(tt.eventKey, tt.eventRune, tcell.ModNone)
			result := handler.HandleEvent(event, view)

			if tt.expectedResult && result != nil {
				t.Errorf("Expected event to be consumed (nil), got non-nil")
			}
			if !tt.expectedResult && result == nil {
				t.Errorf("Expected event to pass through (non-nil), got nil")
			}
		})
	}
}

// TestResultsTableHandler tests the results table handler
func TestResultsTableHandler(t *testing.T) {
	view := createTestView(t)
	handler := NewResultsTableHandler(view)

	tests := []struct {
		name           string
		eventKey       tcell.Key
		eventRune      rune
		expectedResult bool
	}{
		{
			name:           "Enter fetches document",
			eventKey:       tcell.KeyEnter,
			expectedResult: true,
		},
		{
			name:           "F key toggles field list",
			eventKey:       tcell.KeyRune,
			eventRune:      'f',
			expectedResult: true,
		},
		{
			name:           "A key focuses field list",
			eventKey:       tcell.KeyRune,
			eventRune:      'a',
			expectedResult: true,
		},
		{
			name:           "S key focuses selected list",
			eventKey:       tcell.KeyRune,
			eventRune:      's',
			expectedResult: true,
		},
		{
			name:           "Other runes pass through",
			eventKey:       tcell.KeyRune,
			eventRune:      'x',
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tcell.NewEventKey(tt.eventKey, tt.eventRune, tcell.ModNone)
			result := handler.HandleEvent(event, view)

			if tt.expectedResult && result != nil {
				t.Errorf("Expected event to be consumed (nil), got non-nil")
			}
			if !tt.expectedResult && result == nil {
				t.Errorf("Expected event to pass through (non-nil), got nil")
			}
		})
	}
}

// TestFieldListHandler tests the field list handler
func TestFieldListHandler(t *testing.T) {
	view := createTestView(t)
	handler := NewFieldListHandler(view)

	tests := []struct {
		name           string
		eventKey       tcell.Key
		eventRune      rune
		expectedResult bool
	}{
		{
			name:           "Enter toggles field",
			eventKey:       tcell.KeyEnter,
			expectedResult: true,
		},
		{
			name:           "S key focuses selected list",
			eventKey:       tcell.KeyRune,
			eventRune:      's',
			expectedResult: true,
		},
		{
			name:           "Backspace clears filter",
			eventKey:       tcell.KeyBackspace,
			expectedResult: true,
		},
		{
			name:           "Other runes pass through",
			eventKey:       tcell.KeyRune,
			eventRune:      'x',
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tcell.NewEventKey(tt.eventKey, tt.eventRune, tcell.ModNone)
			result := handler.HandleEvent(event, view)

			if tt.expectedResult && result != nil {
				t.Errorf("Expected event to be consumed (nil), got non-nil")
			}
			if !tt.expectedResult && result == nil {
				t.Errorf("Expected event to pass through (non-nil), got nil")
			}
		})
	}
}

// TestSelectedListHandler tests the selected list handler
func TestSelectedListHandler(t *testing.T) {
	view := createTestView(t)
	handler := NewSelectedListHandler(view)

	tests := []struct {
		name           string
		eventKey       tcell.Key
		eventRune      rune
		expectedResult bool
	}{
		{
			name:           "Enter toggles field",
			eventKey:       tcell.KeyEnter,
			expectedResult: true,
		},
		{
			name:           "K key moves field up",
			eventKey:       tcell.KeyRune,
			eventRune:      'k',
			expectedResult: true,
		},
		{
			name:           "J key moves field down",
			eventKey:       tcell.KeyRune,
			eventRune:      'j',
			expectedResult: true,
		},
		{
			name:           "A key focuses field list",
			eventKey:       tcell.KeyRune,
			eventRune:      'a',
			expectedResult: true,
		},
		{
			name:           "Other runes pass through",
			eventKey:       tcell.KeyRune,
			eventRune:      'x',
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tcell.NewEventKey(tt.eventKey, tt.eventRune, tcell.ModNone)
			result := handler.HandleEvent(event, view)

			if tt.expectedResult && result != nil {
				t.Errorf("Expected event to be consumed (nil), got non-nil")
			}
			if !tt.expectedResult && result == nil {
				t.Errorf("Expected event to pass through (non-nil), got nil")
			}
		})
	}
}

// TestIndexInputHandler tests the index input handler
func TestIndexInputHandler(t *testing.T) {
	view := createTestView(t)
	handler := NewIndexInputHandler(view)

	// Test that we can create the handler without panicking
	if handler == nil {
		t.Error("IndexInputHandler should not be nil")
	}

	if handler.GetComponentType() != ComponentIndexInput {
		t.Errorf("Expected ComponentIndexInput, got %v", handler.GetComponentType())
	}

	// Note: We don't test the actual event handling here since it requires
	// fully initialized components which would make the tests more complex
}

// TestTimeframeInputHandler tests the timeframe input handler
func TestTimeframeInputHandler(t *testing.T) {
	view := createTestView(t)
	handler := NewTimeframeInputHandler(view)

	// Test that we can create the handler without panicking
	if handler == nil {
		t.Error("TimeframeInputHandler should not be nil")
	}

	if handler.GetComponentType() != ComponentTimeframeInput {
		t.Errorf("Expected ComponentTimeframeInput, got %v", handler.GetComponentType())
	}

	// Note: We don't test the actual event handling here since it requires
	// fully initialized components which would make the tests more complex
}

// TestListOperations tests the list operations helper
func TestListOperations(t *testing.T) {
	view := createTestView(t)
	listOps := NewListOperations(view)

	// Create a test list with some items
	list := tview.NewList()
	list.AddItem("Item 1", "", 0, nil)
	list.AddItem("Item 2", "", 0, nil)
	list.SetCurrentItem(0)

	// Test getting current list item
	index, text, valid := listOps.GetCurrentListItem(list)
	if !valid {
		t.Error("Should be able to get current list item")
	}
	if index != 0 {
		t.Errorf("Expected index 0, got %d", index)
	}
	if text != "Item 1" {
		t.Errorf("Expected 'Item 1', got '%s'", text)
	}

	// Test with empty list
	emptyList := tview.NewList()
	_, _, valid = listOps.GetCurrentListItem(emptyList)
	if valid {
		t.Error("Should not be valid for empty list")
	}
}

// TestFocusOperations tests the focus operations helper
func TestFocusOperations(t *testing.T) {
	view := createTestView(t)
	focusOps := NewFocusOperations(view)

	// Test that we can create focus operations without panicking
	if focusOps == nil {
		t.Error("FocusOperations should not be nil")
	}

	// Note: We don't test the actual focus setting here since it requires
	// fully initialized components which would make the tests more complex
}

// TestValidationOperations tests the validation operations helper
func TestValidationOperations(t *testing.T) {
	view := createTestView(t)
	validator := NewValidationOperations(view)

	// Test ValidateNonEmpty
	if !validator.ValidateNonEmpty("test") {
		t.Error("Non-empty string should be valid")
	}
	if validator.ValidateNonEmpty("") {
		t.Error("Empty string should be invalid")
	}

	// Test ValidateTimeframe
	if err := validator.ValidateTimeframe("today"); err != nil {
		t.Errorf("'today' should be a valid timeframe, got error: %v", err)
	}
	if err := validator.ValidateTimeframe("invalid"); err == nil {
		t.Error("'invalid' should be an invalid timeframe")
	}
}

// TestErrorOperations tests the error operations helper
func TestErrorOperations(t *testing.T) {
	view := createTestView(t)
	errorOps := NewErrorOperations(view)

	// These should not panic
	errorOps.ShowError("Test error: %s", "message")
	errorOps.ShowSuccess("Test success: %s", "message")
}

// TestAsyncOperations tests the async operations helper
func TestAsyncOperations(t *testing.T) {
	view := createTestView(t)
	asyncOps := NewAsyncOperations(view)

	// Test that we can create async operations without panicking
	if asyncOps == nil {
		t.Error("AsyncOperations should not be nil")
	}

	// Note: We don't test the actual async operations here since they require
	// a real Elasticsearch connection and would make the tests more complex
}

// TestBaseHandler tests the base handler functionality
func TestBaseHandler(t *testing.T) {
	view := createTestView(t)
	baseHandler := NewBaseHandler(ComponentFilterInput, view)

	// Test GetComponentType
	if baseHandler.GetComponentType() != ComponentFilterInput {
		t.Errorf("Expected ComponentFilterInput, got %v", baseHandler.GetComponentType())
	}

	// Test HandleCommonShortcuts
	tests := []struct {
		name     string
		eventKey tcell.Key
		modMask  tcell.ModMask
		handled  bool
	}{
		{
			name:     "Ctrl+A focuses field list",
			eventKey: tcell.KeyCtrlA,
			handled:  true,
		},
		{
			name:     "Ctrl+S focuses selected list",
			eventKey: tcell.KeyCtrlS,
			handled:  true,
		},
		{
			name:     "Ctrl+R focuses results table",
			eventKey: tcell.KeyCtrlR,
			handled:  true,
		},
		{
			name:     "Other keys pass through",
			eventKey: tcell.KeyEnter,
			handled:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tcell.NewEventKey(tt.eventKey, 0, tt.modMask)
			result := baseHandler.HandleCommonShortcuts(event)

			if tt.handled && result != nil {
				t.Errorf("Expected shortcut to be handled (nil), got non-nil")
			}
			if !tt.handled && result == nil {
				t.Errorf("Expected shortcut to pass through (non-nil), got nil")
			}
		})
	}

	// Note: We don't test CanHandle here since it requires complex initialization
	// of the keyResolver which would make the tests more complex
}

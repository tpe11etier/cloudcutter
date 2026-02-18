package elastic

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/logger"
	"github.com/tpelletiersophos/cloudcutter/internal/ui"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/types"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
)

// createTestView creates and initializes the View for testing.
func createTestView(t *testing.T) *View {
	log := createTestLogger(t)
	manager := manager.NewViewManager(context.Background(), ui.NewApp(), aws.Config{}, log)

	// Create field management components
	fieldCache := NewFieldCache()
	fieldState := NewFieldState(fieldCache)

	view := &View{
		manager: manager,
		components: viewComponents{
			filterPrompt: components.NewPrompt(),
		},
		service: nil, // nil for testing
		layout:  nil, // nil for testing
		state: State{
			pagination: PaginationState{
				currentPage: 1,
				pageSize:    50,
				totalPages:  1,
			},
			ui: UIState{
				showRowNumbers:  true,
				isLoading:       false,
				fieldListFilter: "",
			},
			data: DataState{
				fieldCache: fieldCache,
				fieldState: fieldState,
				filters:       []string{},
				currentFilter: "",
				currentResults:   []*DocEntry{},
				filteredResults:  []*DocEntry{},
				displayedResults: []*DocEntry{},
				columnCache:      make(map[string][]string),
			},
			search: SearchState{
				currentIndex:    "test-index",
				matchingIndices: []string{},
				numResults:      1000,
				timeframe:       "today",
			},
			misc: MiscState{
				visibleRows:       0,
				lastDisplayHeight: 0,
				spinner:           nil,
				rateLimit:         NewRateLimiter(),
			},
		},
	}

	// Set up the layout and components
	cfg := types.LayoutConfig{
		Direction: tview.FlexRow,
		Components: []types.Component{
			{
				Type:      types.ComponentFlex,
				Direction: tview.FlexRow,
				FixedSize: 12,
				Children: []types.Component{
					{
						ID:        "filterInput",
						Type:      types.ComponentInputField,
						FixedSize: 3,
						Style: types.InputFieldStyle{
							BaseStyle: types.BaseStyle{
								Border:      true,
								BorderColor: tcell.ColorBeige,
								TitleAlign:  tview.AlignLeft,
							},
							LabelColor:           tcell.ColorMediumTurquoise,
							FieldBackgroundColor: tcell.ColorBlack,
							FieldTextColor:       tcell.ColorBeige,
						},
					},
					{
						ID:        "activeFilters",
						Type:      types.ComponentTextView,
						FixedSize: 3,
						Style: types.TextViewStyle{
							BaseStyle: types.BaseStyle{
								Border:      true,
								BorderColor: tcell.ColorBeige,
							},
						},
					},
					{
						ID:        "timeframeInput",
						Type:      types.ComponentInputField,
						FixedSize: 3,
						Style: types.InputFieldStyle{
							BaseStyle: types.BaseStyle{
								Border:      true,
								BorderColor: tcell.ColorBeige,
							},
						},
					},
					{
						ID:        "fieldList",
						Type:      types.ComponentList,
						FixedSize: 3,
						Style: types.TextViewStyle{
							BaseStyle: types.BaseStyle{
								Border:      true,
								BorderColor: tcell.ColorBeige,
							},
						},
					},
					{
						ID:        "selectedList",
						Type:      types.ComponentList,
						FixedSize: 3,
						Style: types.TextViewStyle{
							BaseStyle: types.BaseStyle{
								Border:      true,
								BorderColor: tcell.ColorBeige,
							},
						},
					},
					{
						ID:        "resultsTable",
						Type:      types.ComponentTable,
						FixedSize: 6,
						Style: types.TableStyle{
							BaseStyle: types.BaseStyle{
								Border:      true,
								BorderColor: tcell.ColorBeige,
							},
						},
					},
				},
			},
		},
	}

	view.components.content = view.manager.CreateLayout(cfg).(*tview.Flex)
	pages := view.manager.Pages()
	pages.AddPage("elastic", view.components.content, true, true)

	view.components.filterInput = view.manager.GetPrimitiveByID("filterInput").(*tview.InputField)
	view.components.activeFilters = view.manager.GetPrimitiveByID("activeFilters").(*tview.TextView)
	view.components.timeframeInput = view.manager.GetPrimitiveByID("timeframeInput").(*tview.InputField)
	view.components.resultsTable = view.manager.GetPrimitiveByID("resultsTable").(*tview.Table)

	view.components.fieldList = tview.NewList()
	view.components.selectedList = tview.NewList()

	return view
}

// createTestLogger helps us capture logs in a temporary directory.
func createTestLogger(t *testing.T) *logger.Logger {
	tempDir, err := os.MkdirTemp("", "test-logs")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cfg := logger.Config{
		LogDir: tempDir,
		Prefix: "test",
		Level:  logger.DEBUG,
	}

	l, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	t.Cleanup(func() {
		l.Close()
		os.RemoveAll(tempDir)
	})

	return l
}

// TestHandleFilterInput verifies that valid filters get added to state, while
// invalid or empty filters do not.
func TestHandleFilterInput(t *testing.T) {
	view := createTestView(t)
	fieldCache := NewFieldCache()
	fieldCache.Set("status", &FieldMetadata{
		Type:         "keyword",
		Searchable:   true,
		Aggregatable: true,
		Active:       false,
	})

	view.state.data.fieldCache = fieldCache

	tests := []struct {
		name            string
		inputText       string
		expectClearText bool
		expectAddFilter bool
	}{
		{
			name:            "Add valid filter",
			inputText:       "status=active",
			expectClearText: true,
			expectAddFilter: true,
		},
		{
			name:            "Empty input",
			inputText:       "",
			expectClearText: false,
			expectAddFilter: false,
		},
		{
			name:            "Invalid filter",
			inputText:       "invalid_filter",
			expectClearText: false,
			expectAddFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view.state.mu.Lock()
			view.state.data.filters = []string{}
			view.state.mu.Unlock()

			view.components.filterInput.SetText(tt.inputText)

			// Simulate Enter key press using test handler system
			event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
			handler := NewTestFilterInputHandler(view)
			handler.HandleEvent(event, view)

			view.state.mu.RLock()
			numFilters := len(view.state.data.filters)
			filters := append([]string{}, view.state.data.filters...) // copy
			view.state.mu.RUnlock()

			if tt.expectClearText && view.components.filterInput.GetText() != "" {
				t.Errorf("expected input field to be cleared, but got %q",
					view.components.filterInput.GetText())
			}

			if tt.expectAddFilter && numFilters == 0 {
				t.Errorf("expected filter to be added, but none was added")
			}

			if !tt.expectAddFilter && numFilters > 0 {
				t.Errorf("expected no filter to be added, but got filters: %v", filters)
			}
		})
	}
}

// TestHandleFieldList_NoPanicIfEmpty ensures we don't panic if fieldList or selectedList
// are empty when pressing Enter.
func TestHandleFieldList_NoPanicIfEmpty(t *testing.T) {
	view := createTestView(t)
	view.components.fieldList.Clear()
	view.components.selectedList.Clear()

	// Simulate Enter key press for fieldList
	fieldListEvent := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	fieldListHandler := NewTestFieldListHandler(view)
	fieldListHandler.HandleEvent(fieldListEvent, view)

	// Simulate Enter key press for selectedList
	selectedListEvent := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	selectedListHandler := NewTestSelectedListHandler(view)
	selectedListHandler.HandleEvent(selectedListEvent, view)

}

func TestHandleFieldList_ToggleSelection(t *testing.T) {
	view := createTestView(t)

	// Create a mock DocEntry with test fields
	mockDoc := &DocEntry{
		ID:    "test-id",
		Index: "test-index",
		Type:  "test-type",
		data: map[string]any{
			"status": "active",
			"user":   "testuser",
		},
	}

	// Set up field metadata
	fieldCache := NewFieldCache()
	fieldCache.Set("status", &FieldMetadata{Type: "keyword"})
	fieldCache.Set("user", &FieldMetadata{Type: "keyword"})
	view.state.data.fieldCache = fieldCache

	// Update field state with the mock document
	view.state.data.fieldState.UpdateFromDocuments([]*DocEntry{mockDoc})

	// Add discovered fields to the fieldList
	discoveredFields := mockDoc.GetAvailableFields()
	for _, field := range discoveredFields {
		if !strings.HasPrefix(field, "_") { // Skip metadata fields for this test
			view.components.fieldList.AddItem(field, "", rune(0), nil)
		}
	}

	// Select "status" field
	view.components.fieldList.SetCurrentItem(0)
	event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	fieldListHandler := NewTestFieldListHandler(view)
	fieldListHandler.HandleEvent(event, view)

	if !view.state.data.fieldState.IsFieldSelected("status") {
		t.Errorf("expected field 'status' to be selected after toggling, but it's not")
	}

	// Verify UI state
	if view.components.selectedList.GetItemCount() != 1 {
		t.Errorf("expected 1 item in selectedList, found %d", view.components.selectedList.GetItemCount())
	}

	// Verify the selected field value can be retrieved
	selectedText, _ := view.components.selectedList.GetItemText(0)
	if selectedText != "status" {
		t.Errorf("expected selected field to be 'status', got %s", selectedText)
	}
}

func TestHandleSelectedList_ToggleBack(t *testing.T) {
	view := createTestView(t)

	// Create a mock DocEntry with test fields
	mockDoc := &DocEntry{
		ID:    "test-id",
		Index: "test-index",
		Type:  "test-type",
		data: map[string]any{
			"status": "active",
		},
	}

	// Set up field metadata
	fieldCache := NewFieldCache()
	fieldCache.Set("status", &FieldMetadata{Type: "keyword"})
	view.state.data.fieldCache = fieldCache

	// Update field state with the mock document
	view.state.data.fieldState.UpdateFromDocuments([]*DocEntry{mockDoc})

	// Select the field programmatically
	view.state.data.fieldState.SelectField("status")

	// Rebuild UI
	view.rebuildFieldList()

	// Verify initial state
	if !view.state.data.fieldState.IsFieldSelected("status") {
		t.Fatalf("initial setup failed: 'status' should be selected")
	}

	// Toggle the field back
	view.components.selectedList.SetCurrentItem(0)
	event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	selectedListHandler := NewTestSelectedListHandler(view)
	selectedListHandler.HandleEvent(event, view)

	// Verify field was unselected
	if view.state.data.fieldState.IsFieldSelected("status") {
		t.Errorf("expected field 'status' to be unselected, but it is still selected")
	}

	// Verify UI state
	if view.components.selectedList.GetItemCount() != 0 {
		t.Errorf("expected 0 items in selectedList, found %d", view.components.selectedList.GetItemCount())
	}
}

func TestFieldState_ComplexDocument(t *testing.T) {
	view := createTestView(t)

	// Create a mock DocEntry with nested fields
	mockDoc := &DocEntry{
		ID:    "test-id",
		Index: "test-index",
		Type:  "test-type",
		data: map[string]any{
			"user": map[string]any{
				"id":   "u123",
				"name": "Test User",
				"preferences": map[string]any{
					"theme": "dark",
				},
			},
			"metadata": map[string]any{
				"tags": []any{"test", "example"},
			},
		},
	}

	// Set up field metadata
	fieldCache := NewFieldCache()
	for _, field := range mockDoc.GetAvailableFields() {
		fieldCache.Set(field, &FieldMetadata{Type: "keyword"})
	}
	view.state.data.fieldCache = fieldCache

	// Update field state
	view.state.data.fieldState.UpdateFromDocuments([]*DocEntry{mockDoc})

	// Get discovered fields
	discoveredFields := view.state.data.fieldState.GetDiscoveredFields()

	// Verify expected nested fields are discovered
	expectedFields := []string{
		"user.id",
		"user.name",
		"user.preferences.theme",
		"metadata.tags",
	}

	for _, expected := range expectedFields {
		found := false
		for _, field := range discoveredFields {
			if field == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected field %s not found in discovered fields", expected)
		}
	}

	// Test selection of nested fields
	for _, field := range expectedFields[:2] { // Select first two fields
		view.state.data.fieldState.SelectField(field)
	}

	selectedFields := view.state.data.fieldState.GetOrderedSelectedFields()
	if len(selectedFields) != 2 {
		t.Errorf("expected 2 selected fields, got %d", len(selectedFields))
	}
}

// TestAddingMultipleFilters checks if multiple valid filters are correctly appended.
func TestAddingMultipleFilters(t *testing.T) {
	view := createTestView(t)
	view.state.mu.Lock()
	view.state.data.filters = []string{}
	view.state.mu.Unlock()

	fieldCache := NewFieldCache()
	fieldCache.Set("status", &FieldMetadata{Type: "keyword", Searchable: true})
	fieldCache.Set("user", &FieldMetadata{Type: "keyword", Searchable: true})
	view.state.data.fieldCache = fieldCache

	inputs := []string{"status=active", "user=jdoe"}

	for _, text := range inputs {
		view.components.filterInput.SetText(text)
		event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
		handler := NewTestFilterInputHandler(view)
		handler.HandleEvent(event, view)
	}

	view.state.mu.RLock()
	defer view.state.mu.RUnlock()

	if len(view.state.data.filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(view.state.data.filters))
	}
}

func TestTimeframeChange(t *testing.T) {
	view := createTestView(t)

	// default timeframe is today"
	if view.state.search.timeframe != "today" {
		t.Fatalf("expected initial timeframe to be 'today', got %q", view.state.search.timeframe)
	}

	// change the timeframe input
	newTimeframe := "24h"
	view.components.timeframeInput.SetText(newTimeframe)

	// simulate pressing Enter
	event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	timeframeHandler := NewTestTimeframeInputHandler(view)
	timeframeHandler.HandleEvent(event, view)

	if view.state.search.timeframe != newTimeframe {
		t.Errorf("expected timeframe to be %q, but got %q",
			newTimeframe, view.state.search.timeframe)
	}
}

// TestNoPanicWhenRefreshIsNil ensures that if refreshWithCurrentTimeframe is nil,
// calling handleTimeframeInput won't panic.
func TestNoPanicWhenRefreshIsNil(t *testing.T) {
	view := createTestView(t)

	// Enter key on timeframeInput
	event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	timeframeHandler := NewTestTimeframeInputHandler(view)
	timeframeHandler.HandleEvent(event, view)

}

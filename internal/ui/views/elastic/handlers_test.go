package elastic

import (
	"context"
	"os"
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

	view := &View{
		manager: manager,
		components: viewComponents{
			filterPrompt: components.NewPrompt(),
		},
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
				activeFields:     make(map[string]bool),
				filters:          []string{},
				currentResults:   []*DocEntry{},
				fieldOrder:       []string{},
				originalFields:   []string{},
				fieldMatches:     []string{},
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

			// Simulate Enter key press
			event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
			view.handleFilterInput(event)

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
	view.handleFieldList(fieldListEvent)

	// Simulate Enter key press for selectedList
	selectedListEvent := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	view.handleSelectedList(selectedListEvent)

}

// TestHandleFieldList_ToggleSelection demonstrates toggling a field from fieldList to selectedList.
func TestHandleFieldList_ToggleSelection(t *testing.T) {
	view := createTestView(t)

	// Add some mock fields to the fieldList
	view.components.fieldList.AddItem("status", "A keyword field", rune(0), nil)
	view.components.fieldList.AddItem("user", "Another field", rune(0), nil)

	// We must set the state fieldCache accordingly
	fieldCache := NewFieldCache()
	fieldCache.Set("status", &FieldMetadata{Type: "keyword"})
	fieldCache.Set("user", &FieldMetadata{Type: "keyword"})
	view.state.data.fieldCache = fieldCache

	// The first item in the list is "status" -> simulate selecting it
	view.components.fieldList.SetCurrentItem(0) // index 0 => "status"
	event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	view.handleFieldList(event)

	view.state.mu.RLock()
	active := view.state.data.activeFields["status"]
	view.state.mu.RUnlock()

	if !active {
		t.Errorf("expected field 'status' to be active after toggling, but it's inactive")
	}

	// Now check if it shows up in selectedList visually
	if view.components.selectedList.GetItemCount() != 1 {
		t.Errorf("expected 1 item in selectedList, found %d", view.components.selectedList.GetItemCount())
	}
}

// TestHandleSelectedList_ToggleBack tests that toggling a field from the selectedList
// (i.e., removing from active fields) works.
func TestHandleSelectedList_ToggleBack(t *testing.T) {
	view := createTestView(t)

	// make "status" active
	view.state.mu.Lock()
	view.state.data.activeFields["status"] = true
	view.state.mu.Unlock()

	// Populate the selectedList
	view.components.selectedList.AddItem("status", "", rune(0), nil)

	// The first item in the selectedList is "status", select it
	view.components.selectedList.SetCurrentItem(0)
	event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	view.handleSelectedList(event)

	view.state.mu.RLock()
	active := view.state.data.activeFields["status"]
	view.state.mu.RUnlock()

	if active {
		t.Errorf("expected field 'status' to be deactivated, but it is still active")
	}

	// should be removed from the selectedList
	if view.components.selectedList.GetItemCount() != 0 {
		t.Errorf("expected 0 items in selectedList, found %d", view.components.selectedList.GetItemCount())
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
		view.handleFilterInput(event)
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
	view.handleTimeframeInput(event)

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
	view.handleTimeframeInput(event)

}

package elastic

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/logger"
	"github.com/tpelletiersophos/cloudcutter/internal/ui"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/types"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	"os"
	"testing"
)

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
				timeframe:       "12h",
			},
			misc: MiscState{
				visibleRows:       0,
				lastDisplayHeight: 0,
				spinner:           nil,
			},
		},
	}

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

	// Set up the layout and components
	view.components.content = view.manager.CreateLayout(cfg).(*tview.Flex)
	pages := view.manager.Pages()
	pages.AddPage("elastic", view.components.content, true, true)

	// Initialize all required components
	view.components.filterInput = view.manager.GetPrimitiveByID("filterInput").(*tview.InputField)
	view.components.activeFilters = view.manager.GetPrimitiveByID("activeFilters").(*tview.TextView)
	view.components.timeframeInput = view.manager.GetPrimitiveByID("timeframeInput").(*tview.InputField)
	view.components.resultsTable = view.manager.GetPrimitiveByID("resultsTable").(*tview.Table)

	view.initTimeframeState()

	return view
}

func TestHandleFilterInput(t *testing.T) {
	view := createTestView(t)

	originalRefresh := view.doRefreshWithCurrentTimeframe
	view.refreshWithCurrentTimeframe = func() {
		// do nothing in tests
	}
	defer func() {
		view.refreshWithCurrentTimeframe = originalRefresh
	}()

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
			// Reset state for each test
			view.state.data.filters = []string{}

			view.components.filterInput.SetText(tt.inputText)

			// Simulate Enter key press
			event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
			view.handleFilterInput(event)

			// Check if input was cleared when expected
			if tt.expectClearText && view.components.filterInput.GetText() != "" {
				t.Errorf("Expected input field to be cleared, but got %s", view.components.filterInput.GetText())
			}

			// Check if filter was added when expected
			if tt.expectAddFilter && len(view.state.data.filters) == 0 {
				t.Errorf("Expected filter to be added, but none was added")
			}

			// Check if filter was not added when not expected
			if !tt.expectAddFilter && len(view.state.data.filters) > 0 {
				t.Errorf("Expected no filter to be added, but got filters: %v", view.state.data.filters)
			}
		})
	}
}

func createTestLogger(t *testing.T) *logger.Logger {
	// Create a temporary directory for logs
	tempDir, err := os.MkdirTemp("", "test-logs")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cfg := logger.Config{
		LogDir: tempDir,
		Prefix: "test",
		Level:  logger.DEBUG,
	}

	// Initialize the logger
	l, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Ensure cleanup
	t.Cleanup(func() {
		l.Close()
		os.RemoveAll(tempDir)
	})

	return l
}

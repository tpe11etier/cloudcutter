package elastic

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/logger"
	"github.com/tpelletiersophos/cloudcutter/internal/ui"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/types"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	"os"
	"testing"
)

func createTestView(t *testing.T) *View {
	log := createTestLogger(t)

	manager := manager.NewViewManager(context.Background(), ui.NewApp(), aws.Config{}, log)

	// Define layout configuration
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
						Properties: types.InputFieldProperties{
							Label:      " ES Filter >_ ",
							FieldWidth: 0,
							OnFocus: func(inputField *tview.InputField) {
								inputField.SetBorderColor(tcell.ColorMediumTurquoise)
							},
							OnBlur: func(inputField *tview.InputField) {
								inputField.SetBorderColor(tcell.ColorBeige)
							},
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
								Title:       " Active Filters ",
								TitleColor:  tcell.ColorYellow,
							},
						},
						Properties: types.TextViewProperties{
							Text:          "No active filters",
							DynamicColors: true,
							OnFocus: func(textView *tview.TextView) {
								textView.SetBorderColor(tcell.ColorMediumTurquoise)
							},
							OnBlur: func(textView *tview.TextView) {
								textView.SetBorderColor(tcell.ColorBeige)
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
								Title:       " Results ",
								TitleColor:  tcell.ColorYellow,
							},
						},
					},
				},
			},
		},
	}

	// Create the view
	view := &View{
		manager: manager,
	}

	// Set up the layout and components
	view.components.content = view.manager.CreateLayout(cfg).(*tview.Flex)
	pages := view.manager.Pages()
	pages.AddPage("elastic", view.components.content, true, true)

	view.components.filterInput = view.manager.GetPrimitiveByID("filterInput").(*tview.InputField)
	view.components.activeFilters = view.manager.GetPrimitiveByID("activeFilters").(*tview.TextView)
	view.components.resultsTable = view.manager.GetPrimitiveByID("resultsTable").(*tview.Table)

	return view
}

func TestHandleFilterInput(t *testing.T) {
	view := createTestView(t)
	view.state = State{
		data: DataState{
			filters: []string{"filter1", "filter2"},
		},
	}

	tests := []struct {
		name            string
		inputText       string
		expectClearText bool
		expectAddFilter bool
	}{
		{
			name:            "Add valid filter",
			inputText:       "filter1",
			expectClearText: true,
			expectAddFilter: true,
		},
		{
			name:            "Empty input",
			inputText:       "",
			expectClearText: false,
			expectAddFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view.components.filterInput.SetText(tt.inputText)
			view.handleFilterInput(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

			if tt.expectClearText && view.components.filterInput.GetText() != "" {
				t.Errorf("Expected input field to be cleared, but got %s", view.components.filterInput.GetText())
			}
			if tt.expectAddFilter && len(view.state.data.filters) == 0 {
				t.Errorf("Expected filter to be added, but none was added")
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

	// Configure the logger
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

//func TestHandleFieldList(t *testing.T) {
//	view := createTestView(t)
//
//	tests := []struct {
//		name          string
//		key           tcell.Key
//		expectedFocus tview.Primitive
//	}{
//		{
//			name:          "Focus remains after Enter",
//			key:           tcell.KeyEnter,
//			expectedFocus: view.components.fieldList,
//		},
//		{
//			name:          "Reset filter on Backspace",
//			key:           tcell.KeyBackspace,
//			expectedFocus: view.components.fieldList,
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			event := tcell.NewEventKey(tt.key, 0, tcell.ModNone)
//			view.handleFieldList(event)
//
//			if view.manager.App().GetFocus() != tt.expectedFocus {
//				t.Errorf("Expected focus to remain on %v, got %v", tt.expectedFocus, view.manager.App().GetFocus())
//			}
//		})
//	}
//}
//
//func TestHandleResultsTable(t *testing.T) {
//	view := createTestView(t)
//	view.state = State{
//		data: DataState{
//			displayedResults: []*DocEntry{
//				{Index: "index1", ID: "doc1"},
//				{Index: "index2", ID: "doc2"},
//			},
//		},
//	}
//
//	tests := []struct {
//		name       string
//		row        int
//		expectCall bool
//	}{
//		{
//			name:       "Valid row selected",
//			row:        1,
//			expectCall: true,
//		},
//		{
//			name:       "Invalid row selected",
//			row:        3,
//			expectCall: false,
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			view.components.resultsTable.Select(tt.row, 0)
//			event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
//			view.handleResultsTable(event)
//
//			// Add assertions for API call mocks or modal visibility
//			// Example:
//			// if tt.expectCall && !mockAPICallInvoked {
//			//     t.Errorf("Expected API call to be invoked for row %d", tt.row)
//			// }
//		})
//	}
//}

//func TestHandleActiveFilters(t *testing.T) {
//	view := createTestView(t)
//	view.state = State{
//		data: DataState{
//			filters: []string{"filter1", "filter2", "filter3"},
//		},
//	}
//
//	tests := []struct {
//		name              string
//		key               tcell.Key
//		expectFilterCount int
//	}{
//		{
//			name:              "Delete a selected filter",
//			key:               tcell.KeyDelete,
//			expectFilterCount: 2,
//		},
//		{
//			name:              "Escape from activeFilters",
//			key:               tcell.KeyEsc,
//			expectFilterCount: 3, // No filters removed
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			event := tcell.NewEventKey(tt.key, 0, tcell.ModNone)
//			view.handleActiveFilters(event)
//
//			if len(view.state.data.filters) != tt.expectFilterCount {
//				t.Errorf("Expected %d filters, got %d", tt.expectFilterCount, len(view.state.data.filters))
//			}
//		})
//	}
//}

//func TestHandleTabKey(t *testing.T) {
//	view := createTestView(t)
//
//	tests := []struct {
//		name         string
//		currentFocus tview.Primitive
//		nextFocus    tview.Primitive
//	}{
//		{
//			name:         "Cycle from filterInput to activeFilters",
//			currentFocus: view.components.filterInput,
//			nextFocus:    view.components.activeFilters,
//		},
//		{
//			name:         "Cycle from activeFilters to indexView",
//			currentFocus: view.components.activeFilters,
//			nextFocus:    view.components.resultsFlex,
//		},
//		{
//			name:         "Cycle back to filterInput from resultsTable",
//			currentFocus: view.components.resultsTable,
//			nextFocus:    view.components.filterInput,
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			view.manager.App().SetFocus(tt.currentFocus)
//			view.handleTabKey(tt.currentFocus)
//			if view.manager.App().GetFocus() != tt.nextFocus {
//				t.Errorf("Expected focus on %v, got %v", tt.nextFocus, view.manager.App().GetFocus())
//			}
//		})
//	}
//}

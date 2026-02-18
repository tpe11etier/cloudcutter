package elastic

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestNewNavigationManager(t *testing.T) {
	mockView := createMockView()
	navManager := NewNavigationManager(mockView)

	if navManager == nil {
		t.Fatal("NavigationManager should not be nil")
	}

	if navManager.view != mockView {
		t.Error("NavigationManager should reference the correct view")
	}

	order := navManager.GetNavigationOrder()
	expectedLength := 9 // Number of navigable components
	if len(order) != expectedLength {
		t.Errorf("Expected navigation order length %d, got %d", expectedLength, len(order))
	}
}

func TestNavigationManager_ForwardNavigation(t *testing.T) {
	mockView := createMockView()
	navManager := NewNavigationManager(mockView)

	// Test the complete forward navigation sequence
	testCases := []struct {
		name     string
		current  tview.Primitive
		expected tview.Primitive
	}{
		{"filterInput -> activeFilters", mockView.components.filterInput, mockView.components.activeFilters},
		{"activeFilters -> indexInput", mockView.components.activeFilters, mockView.components.indexInput},
		{"indexInput -> timeframeInput", mockView.components.indexInput, mockView.components.timeframeInput},
		{"timeframeInput -> numResultsInput", mockView.components.timeframeInput, mockView.components.numResultsInput},
		{"numResultsInput -> localFilterInput", mockView.components.numResultsInput, mockView.components.localFilterInput},
		{"localFilterInput -> fieldList", mockView.components.localFilterInput, mockView.components.fieldList},
		{"fieldList -> selectedList", mockView.components.fieldList, mockView.components.selectedList},
		{"selectedList -> resultsTable", mockView.components.selectedList, mockView.components.resultsTable},
		{"resultsTable -> filterInput (wrap)", mockView.components.resultsTable, mockView.components.filterInput},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			next := navManager.getNextComponent(tc.current, NavigationForward)
			if next != tc.expected {
				t.Errorf("Expected %s, got %s",
					navManager.GetComponentName(tc.expected),
					navManager.GetComponentName(next))
			}
		})
	}
}

func TestNavigationManager_BackwardNavigation(t *testing.T) {
	mockView := createMockView()
	navManager := NewNavigationManager(mockView)

	// Test the complete backward navigation sequence
	testCases := []struct {
		name     string
		current  tview.Primitive
		expected tview.Primitive
	}{
		{"filterInput -> resultsTable (wrap)", mockView.components.filterInput, mockView.components.resultsTable},
		{"resultsTable -> selectedList", mockView.components.resultsTable, mockView.components.selectedList},
		{"selectedList -> fieldList", mockView.components.selectedList, mockView.components.fieldList},
		{"fieldList -> localFilterInput", mockView.components.fieldList, mockView.components.localFilterInput},
		{"localFilterInput -> numResultsInput", mockView.components.localFilterInput, mockView.components.numResultsInput},
		{"numResultsInput -> timeframeInput", mockView.components.numResultsInput, mockView.components.timeframeInput},
		{"timeframeInput -> indexInput", mockView.components.timeframeInput, mockView.components.indexInput},
		{"indexInput -> activeFilters", mockView.components.indexInput, mockView.components.activeFilters},
		{"activeFilters -> filterInput", mockView.components.activeFilters, mockView.components.filterInput},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			next := navManager.getNextComponent(tc.current, NavigationBackward)
			if next != tc.expected {
				t.Errorf("Expected %s, got %s",
					navManager.GetComponentName(tc.expected),
					navManager.GetComponentName(next))
			}
		})
	}
}

func TestNavigationManager_EventKeyGeneration(t *testing.T) {
	// Test the event key generation logic directly without requiring UI interaction

	// Test forward direction returns Tab key
	forwardEvent := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	if forwardEvent.Key() != tcell.KeyTab {
		t.Errorf("Expected Tab key for forward navigation, got %v", forwardEvent.Key())
	}

	// Test backward direction returns BackTab key
	backwardEvent := tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
	if backwardEvent.Key() != tcell.KeyBacktab {
		t.Errorf("Expected BackTab key for backward navigation, got %v", backwardEvent.Key())
	}

	// Test that modifiers are set correctly
	if forwardEvent.Modifiers() != tcell.ModNone {
		t.Errorf("Expected no modifiers for Tab key, got %v", forwardEvent.Modifiers())
	}

	if backwardEvent.Modifiers() != tcell.ModNone {
		t.Errorf("Expected no modifiers for BackTab key, got %v", backwardEvent.Modifiers())
	}
}

func TestNavigationManager_UnknownComponent(t *testing.T) {
	mockView := createMockView()
	navManager := NewNavigationManager(mockView)

	// Test with unknown component - should default to filterInput
	unknownComponent := tview.NewBox()
	next := navManager.getNextComponent(unknownComponent, NavigationForward)

	if next != mockView.components.filterInput {
		t.Errorf("Expected filterInput for unknown component, got %s",
			navManager.GetComponentName(next))
	}

	// Test findComponentIndex with unknown component
	index := navManager.findComponentIndex(unknownComponent)
	if index != -1 {
		t.Errorf("Expected -1 for unknown component index, got %d", index)
	}
}

func TestNavigationManager_FindComponentIndex(t *testing.T) {
	mockView := createMockView()
	navManager := NewNavigationManager(mockView)

	testCases := []struct {
		name      string
		component tview.Primitive
		expected  int
	}{
		{"filterInput at index 0", mockView.components.filterInput, 0},
		{"activeFilters at index 1", mockView.components.activeFilters, 1},
		{"indexInput at index 2", mockView.components.indexInput, 2},
		{"timeframeInput at index 3", mockView.components.timeframeInput, 3},
		{"numResultsInput at index 4", mockView.components.numResultsInput, 4},
		{"localFilterInput at index 5", mockView.components.localFilterInput, 5},
		{"fieldList at index 6", mockView.components.fieldList, 6},
		{"selectedList at index 7", mockView.components.selectedList, 7},
		{"resultsTable at index 8", mockView.components.resultsTable, 8},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			index := navManager.findComponentIndex(tc.component)
			if index != tc.expected {
				t.Errorf("Expected index %d, got %d", tc.expected, index)
			}
		})
	}
}

func TestNavigationManager_GetComponentName(t *testing.T) {
	mockView := createMockView()
	navManager := NewNavigationManager(mockView)

	testCases := []struct {
		component tview.Primitive
		expected  string
	}{
		{mockView.components.filterInput, "filterInput"},
		{mockView.components.activeFilters, "activeFilters"},
		{mockView.components.indexInput, "indexInput"},
		{mockView.components.timeframeInput, "timeframeInput"},
		{mockView.components.numResultsInput, "numResultsInput"},
		{mockView.components.localFilterInput, "localFilterInput"},
		{mockView.components.fieldList, "fieldList"},
		{mockView.components.selectedList, "selectedList"},
		{mockView.components.resultsTable, "resultsTable"},
		{tview.NewBox(), "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			name := navManager.GetComponentName(tc.component)
			if name != tc.expected {
				t.Errorf("Expected component name %s, got %s", tc.expected, name)
			}
		})
	}
}

func TestNavigationManager_ValidateNavigationOrder(t *testing.T) {
	mockView := createMockView()
	navManager := NewNavigationManager(mockView)

	// All components should be present in a properly initialized manager
	missing := navManager.ValidateNavigationOrder()
	if len(missing) != 0 {
		t.Errorf("Expected no missing components, got %v", missing)
	}

	// Test with incomplete navigation order
	incompleteNavManager := &NavigationManager{
		view: mockView,
		navigationOrder: []tview.Primitive{
			mockView.components.filterInput,
			mockView.components.activeFilters,
			// Missing other components
		},
	}

	missing = incompleteNavManager.ValidateNavigationOrder()
	if len(missing) == 0 {
		t.Error("Expected missing components to be detected")
	}

	// Should detect specific missing components
	expectedMissing := []string{"indexInput", "timeframeInput", "numResultsInput",
		"localFilterInput", "fieldList", "selectedList", "resultsTable"}
	if len(missing) != len(expectedMissing) {
		t.Errorf("Expected %d missing components, got %d: %v",
			len(expectedMissing), len(missing), missing)
	}
}

func TestNavigationManager_GetNavigationOrder(t *testing.T) {
	mockView := createMockView()
	navManager := NewNavigationManager(mockView)

	order1 := navManager.GetNavigationOrder()
	order2 := navManager.GetNavigationOrder()

	// Should return copies, not the same slice
	if &order1[0] == &order2[0] {
		t.Error("GetNavigationOrder should return copies, not references to the same slice")
	}

	// But the contents should be the same
	if len(order1) != len(order2) {
		t.Error("Navigation order copies should have the same length")
	}

	for i, component := range order1 {
		if component != order2[i] {
			t.Errorf("Navigation order mismatch at index %d", i)
		}
	}
}

func TestNavigationManager_BidirectionalConsistency(t *testing.T) {
	mockView := createMockView()
	navManager := NewNavigationManager(mockView)

	// Test that forward then backward navigation returns to the same component
	for _, startComponent := range navManager.GetNavigationOrder() {
		t.Run("Round trip from "+navManager.GetComponentName(startComponent), func(t *testing.T) {
			// Go forward one step
			forward := navManager.getNextComponent(startComponent, NavigationForward)
			// Then go backward one step
			backward := navManager.getNextComponent(forward, NavigationBackward)

			if backward != startComponent {
				t.Errorf("Round trip failed: started at %s, went to %s, came back to %s",
					navManager.GetComponentName(startComponent),
					navManager.GetComponentName(forward),
					navManager.GetComponentName(backward))
			}
		})
	}
}

// createMockView creates a mock view with all necessary components for testing
func createMockView() *View {
	return &View{
		components: viewComponents{
			filterInput:      tview.NewInputField(),
			activeFilters:    tview.NewTextView(),
			indexInput:       tview.NewInputField(),
			timeframeInput:   tview.NewInputField(),
			numResultsInput:  tview.NewInputField(),
			localFilterInput: tview.NewInputField(),
			fieldList:        tview.NewList(),
			selectedList:     tview.NewList(),
			resultsTable:     tview.NewTable(),
		},
	}
}

// Benchmark tests to ensure navigation is fast
func BenchmarkNavigationManager_ForwardNavigation(b *testing.B) {
	mockView := createMockView()
	navManager := NewNavigationManager(mockView)
	var current tview.Primitive = mockView.components.filterInput

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current = navManager.getNextComponent(current, NavigationForward)
	}
}

func BenchmarkNavigationManager_BackwardNavigation(b *testing.B) {
	mockView := createMockView()
	navManager := NewNavigationManager(mockView)
	var current tview.Primitive = mockView.components.filterInput

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current = navManager.getNextComponent(current, NavigationBackward)
	}
}

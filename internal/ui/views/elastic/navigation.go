package elastic

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NavigationDirection represents the direction of navigation
type NavigationDirection int

const (
	NavigationForward NavigationDirection = iota
	NavigationBackward
)

// NavigationManager handles component focus navigation
type NavigationManager struct {
	view *View
	// Navigation order defines the sequence of components for Tab navigation
	navigationOrder []tview.Primitive
}

// NewNavigationManager creates a new navigation manager
func NewNavigationManager(view *View) *NavigationManager {
	return &NavigationManager{
		view: view,
		navigationOrder: []tview.Primitive{
			view.components.filterInput,
			view.components.activeFilters,
			view.components.indexInput,
			view.components.timeframeInput,
			view.components.numResultsInput,
			view.components.localFilterInput,
			view.components.fieldList,
			view.components.selectedList,
			view.components.resultsTable,
		},
	}
}

// Navigate moves focus in the specified direction from the current focus
func (n *NavigationManager) Navigate(currentFocus tview.Primitive, direction NavigationDirection) *tcell.EventKey {
	targetComponent := n.getNextComponent(currentFocus, direction)
	n.view.manager.App().SetFocus(targetComponent)

	// Return appropriate event key to maintain compatibility
	switch direction {
	case NavigationForward:
		return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	case NavigationBackward:
		return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
	default:
		return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	}
}

// getNextComponent finds the next component in the navigation sequence
func (n *NavigationManager) getNextComponent(currentFocus tview.Primitive, direction NavigationDirection) tview.Primitive {
	currentIndex := n.findComponentIndex(currentFocus)
	if currentIndex == -1 {
		// Unknown component, default to filter input
		return n.view.components.filterInput
	}

	var nextIndex int
	switch direction {
	case NavigationForward:
		nextIndex = (currentIndex + 1) % len(n.navigationOrder)
	case NavigationBackward:
		nextIndex = (currentIndex - 1 + len(n.navigationOrder)) % len(n.navigationOrder)
	default:
		nextIndex = (currentIndex + 1) % len(n.navigationOrder)
	}

	return n.navigationOrder[nextIndex]
}

// findComponentIndex finds the index of a component in the navigation order
func (n *NavigationManager) findComponentIndex(component tview.Primitive) int {
	for i, navComponent := range n.navigationOrder {
		if navComponent == component {
			return i
		}
	}
	return -1 // Component not found in navigation order
}

// GetNavigationOrder returns a copy of the current navigation order
func (n *NavigationManager) GetNavigationOrder() []tview.Primitive {
	order := make([]tview.Primitive, len(n.navigationOrder))
	copy(order, n.navigationOrder)
	return order
}

// GetComponentName returns a human-readable name for a component (useful for testing and debugging)
func (n *NavigationManager) GetComponentName(component tview.Primitive) string {
	switch component {
	case n.view.components.filterInput:
		return "filterInput"
	case n.view.components.activeFilters:
		return "activeFilters"
	case n.view.components.indexInput:
		return "indexInput"
	case n.view.components.timeframeInput:
		return "timeframeInput"
	case n.view.components.numResultsInput:
		return "numResultsInput"
	case n.view.components.localFilterInput:
		return "localFilterInput"
	case n.view.components.fieldList:
		return "fieldList"
	case n.view.components.selectedList:
		return "selectedList"
	case n.view.components.resultsTable:
		return "resultsTable"
	default:
		return "unknown"
	}
}

// ValidateNavigationOrder checks that all expected components are in the navigation order
func (n *NavigationManager) ValidateNavigationOrder() []string {
	var missing []string
	expectedComponents := map[string]tview.Primitive{
		"filterInput":      n.view.components.filterInput,
		"activeFilters":    n.view.components.activeFilters,
		"indexInput":       n.view.components.indexInput,
		"timeframeInput":   n.view.components.timeframeInput,
		"numResultsInput":  n.view.components.numResultsInput,
		"localFilterInput": n.view.components.localFilterInput,
		"fieldList":        n.view.components.fieldList,
		"selectedList":     n.view.components.selectedList,
		"resultsTable":     n.view.components.resultsTable,
	}

	for name, component := range expectedComponents {
		if n.findComponentIndex(component) == -1 {
			missing = append(missing, name)
		}
	}

	return missing
}

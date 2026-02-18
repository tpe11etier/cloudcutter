package elastic

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// HandlerStrategy defines the interface for component-specific event handlers
type HandlerStrategy interface {
	// HandleEvent processes a key event for this component
	HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey

	// GetComponentType returns the component type this handler manages
	GetComponentType() ComponentType

	// CanHandle checks if this handler can process the given event
	CanHandle(event *tcell.EventKey, currentFocus tview.Primitive) bool
}

// BaseHandler provides common functionality for all handlers
type BaseHandler struct {
	componentType ComponentType
	view          *View
}

// NewBaseHandler creates a new base handler
func NewBaseHandler(componentType ComponentType, view *View) *BaseHandler {
	return &BaseHandler{
		componentType: componentType,
		view:          view,
	}
}

// GetComponentType returns the component type
func (h *BaseHandler) GetComponentType() ComponentType {
	return h.componentType
}

// CanHandle checks if this handler can process the event based on current focus
func (h *BaseHandler) CanHandle(event *tcell.EventKey, currentFocus tview.Primitive) bool {
	// Simple check based on component type matching
	switch h.componentType {
	case ComponentFilterInput:
		return currentFocus == h.view.components.filterInput
	case ComponentActiveFilters:
		return currentFocus == h.view.components.activeFilters
	case ComponentIndexInput:
		return currentFocus == h.view.components.indexInput
	case ComponentFieldList:
		return currentFocus == h.view.components.fieldList
	case ComponentSelectedList:
		return currentFocus == h.view.components.selectedList
	case ComponentResultsTable:
		return currentFocus == h.view.components.resultsTable
	case ComponentTimeframeInput:
		return currentFocus == h.view.components.timeframeInput
	case ComponentNumResultsInput:
		return currentFocus == h.view.components.numResultsInput
	case ComponentLocalFilterInput:
		return currentFocus == h.view.components.localFilterInput
	}
	return false
}

// HandleCommonShortcuts processes common shortcuts that work across components
func (h *BaseHandler) HandleCommonShortcuts(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlA:
		h.view.manager.SetFocus(h.view.components.fieldList)
		return nil
	case tcell.KeyCtrlS:
		h.view.manager.SetFocus(h.view.components.selectedList)
		return nil
	case tcell.KeyCtrlR:
		h.view.manager.SetFocus(h.view.components.resultsTable)
		return nil
	}
	return event
}

// HandlerManager manages all component handlers using the strategy pattern
type HandlerManager struct {
	view     *View
	handlers map[ComponentType]HandlerStrategy
}

// NewHandlerManager creates a new handler manager
func NewHandlerManager(view *View) *HandlerManager {
	manager := &HandlerManager{
		view:     view,
		handlers: make(map[ComponentType]HandlerStrategy),
	}

	// Register all component handlers
	manager.registerHandlers()
	return manager
}

// registerHandlers registers all component-specific handlers
func (hm *HandlerManager) registerHandlers() {
	hm.handlers[ComponentFilterInput] = NewFilterInputHandler(hm.view)
	hm.handlers[ComponentActiveFilters] = NewActiveFiltersHandler(hm.view)
	hm.handlers[ComponentIndexInput] = NewIndexInputHandler(hm.view)
	hm.handlers[ComponentFieldList] = NewFieldListHandler(hm.view)
	hm.handlers[ComponentSelectedList] = NewSelectedListHandler(hm.view)
	hm.handlers[ComponentResultsTable] = NewResultsTableHandler(hm.view)
	hm.handlers[ComponentTimeframeInput] = NewTimeframeInputHandler(hm.view)
	hm.handlers[ComponentNumResultsInput] = NewNumResultsInputHandler(hm.view)
	hm.handlers[ComponentLocalFilterInput] = NewLocalFilterInputHandler(hm.view)
}

// HandleEvent routes the event to the appropriate handler strategy
func (hm *HandlerManager) HandleEvent(event *tcell.EventKey, currentFocus tview.Primitive) *tcell.EventKey {
	// Try to find a handler for the current component
	for _, handler := range hm.handlers {
		if handler.CanHandle(event, currentFocus) {
			return handler.HandleEvent(event, hm.view)
		}
	}

	// Fallback for unmapped keys
	return event
}

// GetHandler returns the handler for a specific component type
func (hm *HandlerManager) GetHandler(componentType ComponentType) (HandlerStrategy, bool) {
	handler, exists := hm.handlers[componentType]
	return handler, exists
}

// ListOperations provides common operations for list components
type ListOperations struct {
	view *View
}

// NewListOperations creates a new list operations helper
func NewListOperations(view *View) *ListOperations {
	return &ListOperations{view: view}
}

// GetCurrentListItem safely gets the current item from a list
func (lo *ListOperations) GetCurrentListItem(list *tview.List) (index int, text string, valid bool) {
	index = list.GetCurrentItem()
	if index < 0 || index >= list.GetItemCount() {
		return -1, "", false
	}

	mainText, _ := list.GetItemText(index)
	return index, mainText, true
}

// FocusOperations provides common focus management operations
type FocusOperations struct {
	view *View
}

// NewFocusOperations creates a new focus operations helper
func NewFocusOperations(view *View) *FocusOperations {
	return &FocusOperations{view: view}
}

// SetFocusToComponent sets focus to a specific component
func (fo *FocusOperations) SetFocusToComponent(componentType ComponentType) {
	switch componentType {
	case ComponentFilterInput:
		fo.view.manager.SetFocus(fo.view.components.filterInput)
	case ComponentActiveFilters:
		fo.view.manager.SetFocus(fo.view.components.activeFilters)
	case ComponentIndexInput:
		fo.view.manager.SetFocus(fo.view.components.indexInput)
	case ComponentFieldList:
		fo.view.manager.SetFocus(fo.view.components.fieldList)
	case ComponentSelectedList:
		fo.view.manager.SetFocus(fo.view.components.selectedList)
	case ComponentResultsTable:
		fo.view.manager.SetFocus(fo.view.components.resultsTable)
	case ComponentTimeframeInput:
		fo.view.manager.SetFocus(fo.view.components.timeframeInput)
	case ComponentNumResultsInput:
		fo.view.manager.SetFocus(fo.view.components.numResultsInput)
	case ComponentLocalFilterInput:
		fo.view.manager.SetFocus(fo.view.components.localFilterInput)
	}
}

// ErrorOperations provides common error handling operations
type ErrorOperations struct {
	view *View
}

// NewErrorOperations creates a new error operations helper
func NewErrorOperations(view *View) *ErrorOperations {
	return &ErrorOperations{view: view}
}

// ShowError displays an error message in the status bar
func (eo *ErrorOperations) ShowError(format string, args ...interface{}) {
	eo.view.manager.UpdateStatusBar(fmt.Sprintf(format, args...))
}

// ShowSuccess displays a success message in the status bar
func (eo *ErrorOperations) ShowSuccess(format string, args ...interface{}) {
	eo.view.manager.UpdateStatusBar(fmt.Sprintf(format, args...))
}

// ValidationOperations provides common validation operations
type ValidationOperations struct {
	view *View
}

// NewValidationOperations creates a new validation operations helper
func NewValidationOperations(view *View) *ValidationOperations {
	return &ValidationOperations{view: view}
}

// ValidateNonEmpty checks if text is not empty
func (vo *ValidationOperations) ValidateNonEmpty(text string) bool {
	return text != ""
}

// ValidateFilter validates a filter string
func (vo *ValidationOperations) ValidateFilter(text string) error {
	_, err := ParseFilter(text, vo.view.state.data.fieldCache)
	return err
}

// ValidateTimeframe validates a timeframe string
func (vo *ValidationOperations) ValidateTimeframe(timeframe string) error {
	return ValidateTimeframe(timeframe)
}

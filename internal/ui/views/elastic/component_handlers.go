package elastic

import (
	"fmt"
	"strconv"

	"github.com/gdamore/tcell/v2"
)

// FilterInputHandler handles events for the filter input component
type FilterInputHandler struct {
	*BaseHandler
	validator *ValidationOperations
	error     *ErrorOperations
}

// NewFilterInputHandler creates a new filter input handler
func NewFilterInputHandler(view *View) *FilterInputHandler {
	return &FilterInputHandler{
		BaseHandler: NewBaseHandler(ComponentFilterInput, view),
		validator:   NewValidationOperations(view),
		error:       NewErrorOperations(view),
	}
}

// HandleEvent processes key events for the filter input
func (h *FilterInputHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	// Check common shortcuts first
	if result := h.HandleCommonShortcuts(event); result == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		view.components.filterInput.SetText("")
		return nil
	case tcell.KeyEnter:
		return h.handleFilterSubmission(view)
	}
	return event
}

// handleFilterSubmission processes filter submission
func (h *FilterInputHandler) handleFilterSubmission(view *View) *tcell.EventKey {
	text := view.components.filterInput.GetText()
	if !h.validator.ValidateNonEmpty(text) {
		return nil
	}

	view.state.mu.Lock()
	defer view.state.mu.Unlock()

	if err := h.validator.ValidateFilter(text); err != nil {
		h.error.ShowError("Invalid filter: %v", err)
		return nil
	}

	view.components.filterInput.SetText("")
	view.addFilter(text)
	view.refreshWithCurrentTimeframe()
	return nil
}

// ActiveFiltersHandler handles events for the active filters component
type ActiveFiltersHandler struct {
	*BaseHandler
	focus *FocusOperations
}

// NewActiveFiltersHandler creates a new active filters handler
func NewActiveFiltersHandler(view *View) *ActiveFiltersHandler {
	return &ActiveFiltersHandler{
		BaseHandler: NewBaseHandler(ComponentActiveFilters, view),
		focus:       NewFocusOperations(view),
	}
}

// HandleEvent processes key events for active filters
func (h *ActiveFiltersHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if result := h.HandleCommonShortcuts(event); result == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.focus.SetFocusToComponent(ComponentFilterInput)
		return nil
	case tcell.KeyDelete, tcell.KeyBackspace2, tcell.KeyBackspace:
		return h.handleFilterDeletion(view)
	case tcell.KeyRune:
		return h.handleNumericFilterDeletion(view, event.Rune())
	}
	return event
}

// handleFilterDeletion deletes the currently selected filter
func (h *ActiveFiltersHandler) handleFilterDeletion(view *View) *tcell.EventKey {
	if len(view.state.data.filters) > 0 {
		view.deleteSelectedFilter()
	}
	return nil
}

// handleNumericFilterDeletion deletes filter by numeric key
func (h *ActiveFiltersHandler) handleNumericFilterDeletion(view *View, r rune) *tcell.EventKey {
	if num, err := strconv.Atoi(string(r)); err == nil && num > 0 && num <= len(view.state.data.filters) {
		view.deleteFilterByIndex(num - 1)
		return nil
	}
	return nil
}

// ResultsTableHandler handles events for the results table component
type ResultsTableHandler struct {
	*BaseHandler
	focus      *FocusOperations
	asyncOps   *AsyncOperations
	validation *ValidationOperations
}

// NewResultsTableHandler creates a new results table handler
func NewResultsTableHandler(view *View) *ResultsTableHandler {
	return &ResultsTableHandler{
		BaseHandler: NewBaseHandler(ComponentResultsTable, view),
		focus:       NewFocusOperations(view),
		asyncOps:    NewAsyncOperations(view),
		validation:  NewValidationOperations(view),
	}
}

// HandleEvent processes key events for the results table
func (h *ResultsTableHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if result := h.HandleCommonShortcuts(event); result == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyRune:
		return h.handleRuneEvents(view, event.Rune())
	case tcell.KeyEnter:
		return h.handleDocumentFetch(view)
	}
	return event
}

// handleRuneEvents processes rune-based key events
func (h *ResultsTableHandler) handleRuneEvents(view *View, r rune) *tcell.EventKey {
	switch r {
	case 'f':
		view.toggleFieldList()
		return nil
	case 'a':
		h.focus.SetFocusToComponent(ComponentFieldList)
		return nil
	case 's':
		h.focus.SetFocusToComponent(ComponentSelectedList)
		return nil
	}
	return nil
}

// handleDocumentFetch processes document fetching on Enter
func (h *ResultsTableHandler) handleDocumentFetch(view *View) *tcell.EventKey {
	row, _ := view.components.resultsTable.GetSelection()
	if row <= 0 {
		return nil
	}

	// Get document entry safely
	entry, err := h.getDocumentEntry(view, row)
	if err != nil {
		return nil
	}

	// Delegate to async operations
	h.asyncOps.FetchFullDocument(entry)
	return nil
}

// getDocumentEntry safely retrieves a document entry
func (h *ResultsTableHandler) getDocumentEntry(view *View, row int) (*DocEntry, error) {
	view.state.mu.RLock()
	defer view.state.mu.RUnlock()

	currentPage := view.state.pagination.currentPage
	pageSize := view.state.pagination.pageSize
	displayedResults := view.state.data.displayedResults

	actualIndex := (currentPage-1)*pageSize + (row - 1)
	if actualIndex >= len(displayedResults) {
		return nil, fmt.Errorf("invalid document index")
	}

	return displayedResults[actualIndex], nil
}

// FieldListHandler handles events for the field list component
type FieldListHandler struct {
	*BaseHandler
	listOps *ListOperations
	focus   *FocusOperations
}

// NewFieldListHandler creates a new field list handler
func NewFieldListHandler(view *View) *FieldListHandler {
	return &FieldListHandler{
		BaseHandler: NewBaseHandler(ComponentFieldList, view),
		listOps:     NewListOperations(view),
		focus:       NewFocusOperations(view),
	}
}

// HandleEvent processes key events for the field list
func (h *FieldListHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if result := h.HandleCommonShortcuts(event); result == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyRune:
		return h.handleRuneEvents(view, event.Rune())
	case tcell.KeyEnter:
		return h.handleFieldToggle(view)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return h.handleFilterClear(view)
	}
	return event
}

// handleRuneEvents processes rune-based events
func (h *FieldListHandler) handleRuneEvents(view *View, r rune) *tcell.EventKey {
	switch r {
	case 's':
		h.focus.SetFocusToComponent(ComponentSelectedList)
		return nil
	}
	return nil
}

// handleFieldToggle toggles field selection
func (h *FieldListHandler) handleFieldToggle(view *View) *tcell.EventKey {
	if _, text, valid := h.listOps.GetCurrentListItem(view.components.fieldList); valid {
		view.toggleField(text)
	}
	return nil
}

// handleFilterClear clears the field filter
func (h *FieldListHandler) handleFilterClear(view *View) *tcell.EventKey {
	view.state.ui.fieldListFilter = ""
	view.filterFieldList("")
	return nil
}

// SelectedListHandler handles events for the selected fields list
type SelectedListHandler struct {
	*BaseHandler
	listOps *ListOperations
	focus   *FocusOperations
}

// NewSelectedListHandler creates a new selected list handler
func NewSelectedListHandler(view *View) *SelectedListHandler {
	return &SelectedListHandler{
		BaseHandler: NewBaseHandler(ComponentSelectedList, view),
		listOps:     NewListOperations(view),
		focus:       NewFocusOperations(view),
	}
}

// HandleEvent processes key events for the selected list
func (h *SelectedListHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if result := h.HandleCommonShortcuts(event); result == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyRune:
		return h.handleRuneEvents(view, event.Rune())
	case tcell.KeyEnter:
		return h.handleFieldToggle(view)
	}
	return event
}

// handleRuneEvents processes rune-based events for field movement
func (h *SelectedListHandler) handleRuneEvents(view *View, r rune) *tcell.EventKey {
	switch r {
	case 'k': // Move up
		return h.handleFieldMove(view, true)
	case 'j': // Move down
		return h.handleFieldMove(view, false)
	case 'a':
		h.focus.SetFocusToComponent(ComponentFieldList)
		return nil
	}
	return nil
}

// handleFieldMove moves a field up or down in the order
func (h *SelectedListHandler) handleFieldMove(view *View, moveUp bool) *tcell.EventKey {
	if _, text, valid := h.listOps.GetCurrentListItem(view.components.selectedList); valid {
		view.moveFieldPosition(text, moveUp)
	}
	return nil
}

// handleFieldToggle toggles field selection
func (h *SelectedListHandler) handleFieldToggle(view *View) *tcell.EventKey {
	if _, text, valid := h.listOps.GetCurrentListItem(view.components.selectedList); valid {
		view.toggleField(text)
	}
	return nil
}

// IndexInputHandler handles events for the index input component
type IndexInputHandler struct {
	*BaseHandler
	validator *ValidationOperations
	asyncOps  *AsyncOperations
}

// NewIndexInputHandler creates a new index input handler
func NewIndexInputHandler(view *View) *IndexInputHandler {
	return &IndexInputHandler{
		BaseHandler: NewBaseHandler(ComponentIndexInput, view),
		validator:   NewValidationOperations(view),
		asyncOps:    NewAsyncOperations(view),
	}
}

// HandleEvent processes key events for the index input
func (h *IndexInputHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if result := h.HandleCommonShortcuts(event); result == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		view.components.indexInput.SetText(view.state.search.currentIndex)
		return nil
	case tcell.KeyEnter:
		return h.handleIndexChange(view)
	}
	return event
}

// handleIndexChange processes index changes
func (h *IndexInputHandler) handleIndexChange(view *View) *tcell.EventKey {
	newIndex := view.components.indexInput.GetText()
	if !h.validator.ValidateNonEmpty(newIndex) {
		return nil
	}

	view.state.mu.Lock()
	indexChanged := newIndex != view.state.search.currentIndex
	view.state.search.currentIndex = newIndex
	view.state.mu.Unlock()

	if indexChanged {
		h.asyncOps.ReloadFieldsForNewIndex()
	} else {
		view.refreshWithCurrentTimeframe()
	}
	return nil
}

// TimeframeInputHandler handles events for the timeframe input component
type TimeframeInputHandler struct {
	*BaseHandler
	validator *ValidationOperations
	error     *ErrorOperations
	focus     *FocusOperations
}

// NewTimeframeInputHandler creates a new timeframe input handler
func NewTimeframeInputHandler(view *View) *TimeframeInputHandler {
	return &TimeframeInputHandler{
		BaseHandler: NewBaseHandler(ComponentTimeframeInput, view),
		validator:   NewValidationOperations(view),
		error:       NewErrorOperations(view),
		focus:       NewFocusOperations(view),
	}
}

// HandleEvent processes key events for the timeframe input
func (h *TimeframeInputHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if result := h.HandleCommonShortcuts(event); result == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.focus.SetFocusToComponent(ComponentFilterInput)
		return nil
	case tcell.KeyEnter:
		return h.handleTimeframeChange(view)
	}
	return event
}

// handleTimeframeChange processes timeframe changes
func (h *TimeframeInputHandler) handleTimeframeChange(view *View) *tcell.EventKey {
	timeframe := view.components.timeframeInput.GetText()
	if err := h.validator.ValidateTimeframe(timeframe); err != nil {
		h.error.ShowError("Error: %v", err)
		return nil
	}

	view.state.search.timeframe = timeframe
	view.refreshWithCurrentTimeframe()
	return nil
}

// NumResultsInputHandler handles events for the number of results input
type NumResultsInputHandler struct {
	*BaseHandler
	focus *FocusOperations
}

// NewNumResultsInputHandler creates a new number results input handler
func NewNumResultsInputHandler(view *View) *NumResultsInputHandler {
	return &NumResultsInputHandler{
		BaseHandler: NewBaseHandler(ComponentNumResultsInput, view),
		focus:       NewFocusOperations(view),
	}
}

// HandleEvent processes key events for the number results input
func (h *NumResultsInputHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if result := h.HandleCommonShortcuts(event); result == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.focus.SetFocusToComponent(ComponentFilterInput)
		return nil
	}
	return event
}

// LocalFilterInputHandler handles events for the local filter input
type LocalFilterInputHandler struct {
	*BaseHandler
	focus *FocusOperations
}

// NewLocalFilterInputHandler creates a new local filter input handler
func NewLocalFilterInputHandler(view *View) *LocalFilterInputHandler {
	return &LocalFilterInputHandler{
		BaseHandler: NewBaseHandler(ComponentLocalFilterInput, view),
		focus:       NewFocusOperations(view),
	}
}

// HandleEvent processes key events for the local filter input
func (h *LocalFilterInputHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if result := h.HandleCommonShortcuts(event); result == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.focus.SetFocusToComponent(ComponentFilterInput)
		return nil
	}
	return event
}

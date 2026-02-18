package elastic

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ActionExecutor handles the execution of key actions
type ActionExecutor struct {
	view *View
}

// NewActionExecutor creates a new action executor
func NewActionExecutor(view *View) *ActionExecutor {
	return &ActionExecutor{view: view}
}

// ExecuteAction executes a key action
func (e *ActionExecutor) ExecuteAction(action *KeyAction) *tcell.EventKey {
	switch action.Type {
	case ActionFocus:
		return e.executeFocusAction(action.Data)
	case ActionToggle:
		return e.executeToggleAction(action.Data)
	case ActionNavigate:
		return e.executeNavigateAction(action.Data)
	case ActionFilter:
		return e.executeFilterAction(action.Data)
	case ActionDocument:
		return e.executeDocumentAction(action.Data)
	case ActionEdit:
		return e.executeEditAction(action.Data)
	case ActionClear:
		return e.executeClearAction(action.Data)
	case ActionDelete:
		return e.executeDeleteAction(action.Data)
	case ActionMove:
		return e.executeMoveAction(action.Data)
	case ActionReorder:
		return e.executeReorderAction(action.Data)
	default:
		// Unknown action type, let the event pass through
		return nil
	}
}

// executeFocusAction handles focus changes
func (e *ActionExecutor) executeFocusAction(data interface{}) *tcell.EventKey {
	component, ok := data.(ComponentType)
	if !ok {
		return nil
	}

	switch component {
	case ComponentFilterInput:
		e.view.manager.SetFocus(e.view.components.filterInput)
	case ComponentActiveFilters:
		e.view.manager.SetFocus(e.view.components.activeFilters)
	case ComponentIndexInput:
		e.view.manager.SetFocus(e.view.components.indexInput)
	case ComponentFieldList:
		e.view.manager.SetFocus(e.view.components.fieldList)
	case ComponentSelectedList:
		e.view.manager.SetFocus(e.view.components.selectedList)
	case ComponentResultsTable:
		e.view.manager.SetFocus(e.view.components.resultsTable)
	case ComponentTimeframeInput:
		e.view.manager.SetFocus(e.view.components.timeframeInput)
	case ComponentNumResultsInput:
		e.view.manager.SetFocus(e.view.components.numResultsInput)
	case ComponentLocalFilterInput:
		e.view.manager.SetFocus(e.view.components.localFilterInput)
	}
	return nil
}

// executeToggleAction handles toggle operations
func (e *ActionExecutor) executeToggleAction(data interface{}) *tcell.EventKey {
	action, ok := data.(string)
	if !ok {
		return nil
	}

	switch action {
	case "field_list":
		e.view.toggleFieldList()
	case "row_numbers":
		e.view.toggleRowNumbers()
	case "field_selection":
		// Handle field selection toggle
		currentFocus := e.view.manager.App().GetFocus()
		if currentFocus == e.view.components.fieldList {
			index := e.view.components.fieldList.GetCurrentItem()
			if index >= 0 && index < e.view.components.fieldList.GetItemCount() {
				mainText, _ := e.view.components.fieldList.GetItemText(index)
				e.view.toggleField(mainText)
			}
		} else if currentFocus == e.view.components.selectedList {
			index := e.view.components.selectedList.GetCurrentItem()
			if index >= 0 && index < e.view.components.selectedList.GetItemCount() {
				mainText, _ := e.view.components.selectedList.GetItemText(index)
				e.view.toggleField(mainText)
			}
		}
	}
	return nil
}

// executeNavigateAction handles navigation operations
func (e *ActionExecutor) executeNavigateAction(data interface{}) *tcell.EventKey {
	action, ok := data.(string)
	if !ok {
		return nil
	}

	switch action {
	case "forward":
		currentFocus := e.view.manager.App().GetFocus()
		return e.view.handleTabKey(currentFocus)
	case "backward":
		// Simple reverse tab - focus filter input as fallback
		e.view.manager.App().SetFocus(e.view.components.filterInput)
		return nil
	case "next_page":
		e.view.nextPage()
	case "previous_page":
		e.view.previousPage()
	case "up", "down", "left", "right":
		return e.executeVimNavigation(action)
	}
	return nil
}

// executeVimNavigation handles vim-style directional navigation
func (e *ActionExecutor) executeVimNavigation(direction string) *tcell.EventKey {
	currentFocus := e.view.manager.App().GetFocus()

	switch currentFocus {
	case e.view.components.resultsTable:
		return e.executeTableNavigation(direction)
	case e.view.components.fieldList:
		return e.executeListNavigation(e.view.components.fieldList, direction)
	case e.view.components.selectedList:
		return e.executeListNavigation(e.view.components.selectedList, direction)
	}
	return nil
}

// executeTableNavigation handles vim navigation for the results table
func (e *ActionExecutor) executeTableNavigation(direction string) *tcell.EventKey {
	table := e.view.components.resultsTable
	row, col := table.GetSelection()

	switch direction {
	case "up":
		if row > 0 {
			table.Select(row-1, col)
		}
	case "down":
		if row < table.GetRowCount()-1 {
			table.Select(row+1, col)
		}
	case "left":
		if col > 0 {
			table.Select(row, col-1)
		}
	case "right":
		if col < table.GetColumnCount()-1 {
			table.Select(row, col+1)
		}
	}
	return nil
}

// executeListNavigation handles vim navigation for lists (fieldList, selectedList)
func (e *ActionExecutor) executeListNavigation(list *tview.List, direction string) *tcell.EventKey {
	currentIndex := list.GetCurrentItem()
	itemCount := list.GetItemCount()

	switch direction {
	case "up":
		if currentIndex > 0 {
			list.SetCurrentItem(currentIndex - 1)
		}
	case "down":
		if currentIndex < itemCount-1 {
			list.SetCurrentItem(currentIndex + 1)
		}
	case "left", "right":
		// For lists, left/right don't have meaning, ignore
		return nil
	}
	return nil
}

// executeFilterAction handles filter operations
func (e *ActionExecutor) executeFilterAction(data interface{}) *tcell.EventKey {
	action, ok := data.(string)
	if !ok {
		return nil
	}

	switch action {
	case "add":
		return e.executeAddFilter()
	case "show_prompt":
		currentFocus := e.view.manager.App().GetFocus()
		switch currentFocus {
		case e.view.components.fieldList:
			e.view.showFilterPrompt(e.view.components.fieldList)
		case e.view.components.localFilterInput:
			e.view.showFilterPrompt(e.view.components.localFilterInput)
		case e.view.components.resultsTable:
			e.view.showFilterPrompt(e.view.components.resultsTable)
		}
	}
	return nil
}

// executeAddFilter handles adding a filter from the filter input
func (e *ActionExecutor) executeAddFilter() *tcell.EventKey {
	text := e.view.components.filterInput.GetText()
	if text == "" {
		return nil
	}

	e.view.state.mu.Lock()
	if _, err := ParseFilter(text, e.view.state.data.fieldCache); err != nil {
		e.view.state.mu.Unlock()
		e.view.manager.UpdateStatusBar(fmt.Sprintf("Invalid filter: %v", err))
		return nil
	}

	e.view.components.filterInput.SetText("")
	e.view.addFilter(text)
	e.view.state.mu.Unlock()

	e.view.refreshWithCurrentTimeframe()
	return nil
}

// executeDocumentAction handles document operations
func (e *ActionExecutor) executeDocumentAction(data interface{}) *tcell.EventKey {
	action, ok := data.(string)
	if !ok {
		return nil
	}

	switch action {
	case "fetch_full":
		return e.executeFetchFullDocument()
	}
	return nil
}

// executeFetchFullDocument handles fetching and displaying full document using centralized async operations
func (e *ActionExecutor) executeFetchFullDocument() *tcell.EventKey {
	row, _ := e.view.components.resultsTable.GetSelection()
	if row <= 0 {
		return nil
	}

	// Get document entry safely
	entry, err := e.getDocumentEntry(row)
	if err != nil {
		return nil
	}

	// Use centralized async operations for document fetching
	asyncOps := NewAsyncOperations(e.view)
	asyncOps.FetchFullDocument(entry)
	return nil
}

// getDocumentEntry safely retrieves a document entry (extracted from component handlers)
func (e *ActionExecutor) getDocumentEntry(row int) (*DocEntry, error) {
	e.view.state.mu.RLock()
	defer e.view.state.mu.RUnlock()

	currentPage := e.view.state.pagination.currentPage
	pageSize := e.view.state.pagination.pageSize
	displayedResults := e.view.state.data.displayedResults

	actualIndex := (currentPage-1)*pageSize + (row - 1)
	if actualIndex >= len(displayedResults) {
		return nil, fmt.Errorf("invalid document index")
	}

	return displayedResults[actualIndex], nil
}

// executeEditAction handles edit operations
func (e *ActionExecutor) executeEditAction(data interface{}) *tcell.EventKey {
	action, ok := data.(string)
	if !ok {
		return nil
	}

	switch action {
	case "change_index":
		return e.executeChangeIndex()
	case "apply_timeframe":
		return e.executeApplyTimeframe()
	}
	return nil
}

// executeChangeIndex handles changing the search index using centralized async operations
func (e *ActionExecutor) executeChangeIndex() *tcell.EventKey {
	newIndex := e.view.components.indexInput.GetText()
	if newIndex == "" {
		return nil
	}

	e.view.state.mu.Lock()
	indexChanged := newIndex != e.view.state.search.currentIndex
	e.view.state.search.currentIndex = newIndex
	e.view.state.mu.Unlock()

	if indexChanged {
		// Use centralized async operations for field reloading
		asyncOps := NewAsyncOperations(e.view)
		asyncOps.ReloadFieldsForNewIndex()
	} else {
		e.view.refreshWithCurrentTimeframe()
	}
	return nil
}

// executeApplyTimeframe handles applying a new timeframe
func (e *ActionExecutor) executeApplyTimeframe() *tcell.EventKey {
	timeframe := e.view.components.timeframeInput.GetText()
	if err := ValidateTimeframe(timeframe); err != nil {
		e.view.manager.UpdateStatusBar(fmt.Sprintf("Error: %v", err))
		return nil
	}
	e.view.state.search.timeframe = e.view.components.timeframeInput.GetText()
	e.view.refreshWithCurrentTimeframe()
	return nil
}

// executeClearAction handles clear operations
func (e *ActionExecutor) executeClearAction(data interface{}) *tcell.EventKey {
	action, ok := data.(string)
	if !ok {
		return nil
	}

	switch action {
	case "text":
		e.view.components.filterInput.SetText("")
	case "reset_to_current":
		e.view.components.indexInput.SetText(e.view.state.search.currentIndex)
	case "field_filter":
		e.view.state.ui.fieldListFilter = ""
		e.view.filterFieldList("")
	}
	return nil
}

// executeDeleteAction handles delete operations
func (e *ActionExecutor) executeDeleteAction(data interface{}) *tcell.EventKey {
	switch v := data.(type) {
	case string:
		if v == "selected_filter" {
			if len(e.view.state.data.filters) > 0 {
				e.view.deleteSelectedFilter()
			}
		}
	case int:
		// Delete filter by index (0-based)
		if v >= 0 && v < len(e.view.state.data.filters) {
			e.view.deleteFilterByIndex(v)
		}
	}
	return nil
}

// executeMoveAction handles move operations
func (e *ActionExecutor) executeMoveAction(data interface{}) *tcell.EventKey {
	direction, ok := data.(string)
	if !ok {
		return nil
	}

	index := e.view.components.selectedList.GetCurrentItem()
	if index >= 0 && index < e.view.components.selectedList.GetItemCount() {
		mainText, _ := e.view.components.selectedList.GetItemText(index)
		switch direction {
		case "up":
			e.view.moveFieldPosition(mainText, true)
		case "down":
			e.view.moveFieldPosition(mainText, false)
		}
	}
	return nil
}

// executeReorderAction handles reordering operations (uppercase J/K)
func (e *ActionExecutor) executeReorderAction(data interface{}) *tcell.EventKey {
	direction, ok := data.(string)
	if !ok {
		return nil
	}

	// Only works on the selected list
	currentFocus := e.view.manager.App().GetFocus()
	if currentFocus != e.view.components.selectedList {
		return nil
	}

	index := e.view.components.selectedList.GetCurrentItem()
	if index >= 0 && index < e.view.components.selectedList.GetItemCount() {
		mainText, _ := e.view.components.selectedList.GetItemText(index)
		switch direction {
		case "up":
			e.view.moveFieldPosition(mainText, true)
		case "down":
			e.view.moveFieldPosition(mainText, false)
		}
	}
	return nil
}

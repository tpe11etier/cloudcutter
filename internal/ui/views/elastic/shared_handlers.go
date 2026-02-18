package elastic

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Simplified handlers that work with the original view structure

// ElasticResultsTableHandler handles events for the results table
type ElasticResultsTableHandler struct {
	view *View
}

// NewElasticResultsTableHandler creates a new results table handler
func NewElasticResultsTableHandler(view *View) *ElasticResultsTableHandler {
	return &ElasticResultsTableHandler{
		view: view,
	}
}

// CanHandle checks if this handler can process the given event
func (h *ElasticResultsTableHandler) CanHandle(event *tcell.EventKey, component tview.Primitive) bool {
	switch event.Key() {
	case tcell.KeyEnter:
		return true
	case tcell.KeyRune:
		switch event.Rune() {
		case 'r', 'n', 'p':
			return true
		}
	}
	return false
}

// HandleEvent processes the event
func (h *ElasticResultsTableHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		// Show document modal
		h.view.manager.Logger().Debug("User requested document modal")

		// Get current selection
		row, _ := h.view.components.resultsTable.GetSelection()
		if row <= 0 {
			return nil
		}

		// Async operation to prevent UI lockups
		go func() {
			entry, err := h.getDocumentEntry(row)
			if err != nil {
				h.view.manager.App().QueueUpdateDraw(func() {
					h.view.manager.UpdateStatusBar(fmt.Sprintf("Error: %v", err))
				})
				return
			}

			// Queue UI update on main thread
			h.view.manager.App().QueueUpdateDraw(func() {
				h.view.showJSONModal(entry)
			})
		}()
		return nil

	case tcell.KeyRune:
		switch event.Rune() {
		case 'r':
			h.view.manager.Logger().Debug("User toggled row numbers")
			h.view.toggleRowNumbers()
			return nil
		case 'n':
			h.view.manager.Logger().Debug("User navigated to next page")
			h.view.nextPage()
			return nil
		case 'p':
			h.view.manager.Logger().Debug("User navigated to previous page")
			h.view.previousPage()
			return nil
		}
	}

	return event
}

// getDocumentEntry safely retrieves a document entry
func (h *ElasticResultsTableHandler) getDocumentEntry(row int) (*DocEntry, error) {
	h.view.state.mu.RLock()
	defer h.view.state.mu.RUnlock()

	currentPage := h.view.state.pagination.currentPage
	pageSize := h.view.state.pagination.pageSize
	displayedResults := h.view.state.data.displayedResults

	actualIndex := (currentPage-1)*pageSize + (row - 1)
	if actualIndex >= len(displayedResults) {
		return nil, fmt.Errorf("invalid document index")
	}

	return displayedResults[actualIndex], nil
}

// ElasticFieldListHandler handles events for the field list
type ElasticFieldListHandler struct {
	view *View
}

// NewElasticFieldListHandler creates a new field list handler
func NewElasticFieldListHandler(view *View) *ElasticFieldListHandler {
	return &ElasticFieldListHandler{
		view: view,
	}
}

// CanHandle checks if this handler can process the given event
func (h *ElasticFieldListHandler) CanHandle(event *tcell.EventKey, component tview.Primitive) bool {
	return event.Key() == tcell.KeyEnter
}

// HandleEvent processes the event
func (h *ElasticFieldListHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if event.Key() == tcell.KeyEnter {
		h.view.manager.Logger().Debug("User selected field from field list")

		// Get current field
		if _, text, valid := h.getCurrentListItem(h.view.components.fieldList); valid {
			h.view.toggleField(text)
		}
		return nil
	}
	return event
}

// getCurrentListItem gets the current list item - helper method
func (h *ElasticFieldListHandler) getCurrentListItem(list *tview.List) (int, string, bool) {
	if list.GetItemCount() == 0 {
		return -1, "", false
	}
	index := list.GetCurrentItem()
	if index < 0 || index >= list.GetItemCount() {
		return -1, "", false
	}
	text, _ := list.GetItemText(index)
	return index, text, true
}

// ElasticSelectedListHandler handles events for the selected fields list
type ElasticSelectedListHandler struct {
	view *View
}

// NewElasticSelectedListHandler creates a new selected list handler
func NewElasticSelectedListHandler(view *View) *ElasticSelectedListHandler {
	return &ElasticSelectedListHandler{
		view: view,
	}
}

// CanHandle checks if this handler can process the given event
func (h *ElasticSelectedListHandler) CanHandle(event *tcell.EventKey, component tview.Primitive) bool {
	return event.Key() == tcell.KeyEnter
}

// HandleEvent processes the event
func (h *ElasticSelectedListHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if event.Key() == tcell.KeyEnter {
		h.view.manager.Logger().Debug("User removed field from selected list")

		// Get current field and remove it
		if _, text, valid := h.getCurrentListItem(h.view.components.selectedList); valid {
			h.view.toggleField(text) // toggleField handles both add and remove
		}
		return nil
	}
	return event
}

// getCurrentListItem gets the current list item - helper method
func (h *ElasticSelectedListHandler) getCurrentListItem(list *tview.List) (int, string, bool) {
	if list.GetItemCount() == 0 {
		return -1, "", false
	}
	index := list.GetCurrentItem()
	if index < 0 || index >= list.GetItemCount() {
		return -1, "", false
	}
	text, _ := list.GetItemText(index)
	return index, text, true
}

// ElasticFilterInputHandler handles events for filter input components
type ElasticFilterInputHandler struct {
	view *View
}

// NewElasticFilterInputHandler creates a new filter input handler
func NewElasticFilterInputHandler(view *View) *ElasticFilterInputHandler {
	return &ElasticFilterInputHandler{
		view: view,
	}
}

// CanHandle checks if this handler can process the given event
func (h *ElasticFilterInputHandler) CanHandle(event *tcell.EventKey, component tview.Primitive) bool {
	return event.Key() == tcell.KeyEnter
}

// HandleEvent processes the event
func (h *ElasticFilterInputHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if event.Key() == tcell.KeyEnter {
		// Determine which input this is and handle accordingly
		currentFocus := h.view.manager.App().GetFocus()

		switch currentFocus {
		case h.view.components.filterInput:
			h.view.manager.Logger().Debug("User submitted main filter")
			h.view.refreshResults()

		case h.view.components.localFilterInput:
			h.view.manager.Logger().Debug("User submitted local filter")
			h.view.refreshResults()

		case h.view.components.indexInput:
			h.view.manager.Logger().Debug("User changed index")
			h.view.refreshResults()

		case h.view.components.timeframeInput:
			h.view.manager.Logger().Debug("User changed timeframe")
			h.view.refreshWithCurrentTimeframe()

		case h.view.components.numResultsInput:
			h.view.manager.Logger().Debug("User changed number of results")
			h.view.refreshResults()
		}

		return nil
	}
	return event
}
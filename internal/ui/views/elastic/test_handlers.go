package elastic

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
)

// Test-specific handlers that don't trigger async operations

type MockFilterInputHandler struct {
	view *View
}

func NewTestFilterInputHandler(view *View) *MockFilterInputHandler {
	return &MockFilterInputHandler{view: view}
}

func (h *MockFilterInputHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if shortcut := h.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.view.components.filterInput.SetText("")
		return nil
	case tcell.KeyEnter:
		text := h.view.components.filterInput.GetText()
		if text == "" {
			return nil
		}

		h.view.state.mu.Lock()
		if _, err := ParseFilter(text, h.view.state.data.fieldCache); err != nil {
			h.view.state.mu.Unlock()
			h.view.manager.UpdateStatusBar(fmt.Sprintf("Invalid filter: %v", err))
			return nil
		}

		h.view.components.filterInput.SetText("")
		h.view.addFilter(text)
		h.view.state.mu.Unlock()

		// Skip refreshWithCurrentTimeframe() in tests to avoid async operations
		return nil
	}
	return event
}

func (h *MockFilterInputHandler) handleCommonShortcuts(event *tcell.EventKey) *tcell.EventKey {
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

type MockFieldListHandler struct {
	view *View
}

func NewTestFieldListHandler(view *View) *MockFieldListHandler {
	return &MockFieldListHandler{view: view}
}

func (h *MockFieldListHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if shortcut := h.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 's':
			h.view.manager.SetFocus(h.view.components.selectedList)
		}
	case tcell.KeyEnter:
		index := h.view.components.fieldList.GetCurrentItem()
		if index >= 0 && index < h.view.components.fieldList.GetItemCount() {
			mainText, _ := h.view.components.fieldList.GetItemText(index)
			h.view.toggleField(mainText)
		}
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		h.view.state.ui.fieldListFilter = ""
		h.view.filterFieldList("")
		return nil
	}
	return event
}

func (h *MockFieldListHandler) handleCommonShortcuts(event *tcell.EventKey) *tcell.EventKey {
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

type MockSelectedListHandler struct {
	view *View
}

func NewTestSelectedListHandler(view *View) *MockSelectedListHandler {
	return &MockSelectedListHandler{view: view}
}

func (h *MockSelectedListHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if shortcut := h.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'k': // Move up
			index := h.view.components.selectedList.GetCurrentItem()
			if index >= 0 && index < h.view.components.selectedList.GetItemCount() {
				mainText, _ := h.view.components.selectedList.GetItemText(index)
				h.view.moveFieldPosition(mainText, true)
			}
			return nil
		case 'j': // Move down
			index := h.view.components.selectedList.GetCurrentItem()
			if index >= 0 && index < h.view.components.selectedList.GetItemCount() {
				mainText, _ := h.view.components.selectedList.GetItemText(index)
				h.view.moveFieldPosition(mainText, false)
			}
			return nil
		case 'a':
			h.view.manager.SetFocus(h.view.components.fieldList)
		}
	case tcell.KeyEnter:
		index := h.view.components.selectedList.GetCurrentItem()
		if index >= 0 && index < h.view.components.selectedList.GetItemCount() {
			mainText, _ := h.view.components.selectedList.GetItemText(index)
			h.view.toggleField(mainText)
		}
		return nil
	}
	return event
}

func (h *MockSelectedListHandler) handleCommonShortcuts(event *tcell.EventKey) *tcell.EventKey {
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

type MockTimeframeInputHandler struct {
	view *View
}

func NewTestTimeframeInputHandler(view *View) *MockTimeframeInputHandler {
	return &MockTimeframeInputHandler{view: view}
}

func (h *MockTimeframeInputHandler) HandleEvent(event *tcell.EventKey, view *View) *tcell.EventKey {
	if shortcut := h.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.view.manager.SetFocus(h.view.components.filterInput)
		return nil
	case tcell.KeyEnter:
		timeframe := h.view.components.timeframeInput.GetText()
		if err := ValidateTimeframe(timeframe); err != nil {
			h.view.manager.UpdateStatusBar(fmt.Sprintf("Error: %v", err))
			return nil
		}
		h.view.state.search.timeframe = h.view.components.timeframeInput.GetText()
		// Skip refreshWithCurrentTimeframe() in tests to avoid async operations
		return nil
	}
	return event
}

func (h *MockTimeframeInputHandler) handleCommonShortcuts(event *tcell.EventKey) *tcell.EventKey {
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
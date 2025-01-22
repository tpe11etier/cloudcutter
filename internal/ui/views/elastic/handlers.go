package elastic

import (
	"encoding/json"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"strconv"
)

func (v *View) handleTabKey(currentFocus tview.Primitive) *tcell.EventKey {
	switch currentFocus {
	case v.components.filterInput:
		v.manager.App().SetFocus(v.components.activeFilters)
	case v.components.activeFilters:
		v.manager.App().SetFocus(v.components.indexInput)
	case v.components.indexInput:
		v.manager.App().SetFocus(v.components.timeframeInput)
	case v.components.timeframeInput:
		v.manager.App().SetFocus(v.components.numResultsInput)
	case v.components.numResultsInput:
		v.manager.App().SetFocus(v.components.localFilterInput)

	case v.components.localFilterInput:
		v.manager.App().SetFocus(v.components.fieldList)
	case v.components.fieldList:
		v.manager.App().SetFocus(v.components.selectedList)
	case v.components.selectedList:
		v.manager.App().SetFocus(v.components.resultsTable)
	case v.components.resultsTable:
		v.manager.App().SetFocus(v.components.filterInput)
	default:
		v.manager.App().SetFocus(v.components.filterInput)
	}
	return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
}

func (v *View) handleFilterInput(event *tcell.EventKey) *tcell.EventKey {
	if shortcut := v.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		v.components.filterInput.SetText("")
		return nil
	case tcell.KeyEnter:
		text := v.components.filterInput.GetText()
		if text == "" {
			return nil
		}

		v.state.mu.Lock()
		if _, err := ParseFilter(text, v.state.data.fieldCache); err != nil {
			v.state.mu.Unlock()
			v.manager.UpdateStatusBar(fmt.Sprintf("Invalid filter: %v", err))
			return nil
		}

		v.components.filterInput.SetText("")
		v.addFilter(text)
		v.state.mu.Unlock()

		v.refreshWithCurrentTimeframe()
		return nil
	}
	return event
}

func (v *View) handleActiveFilters(event *tcell.EventKey) *tcell.EventKey {
	if shortcut := v.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		v.manager.SetFocus(v.components.filterInput)
		return nil
	case tcell.KeyDelete, tcell.KeyBackspace2, tcell.KeyBackspace:
		if len(v.state.data.filters) > 0 {
			v.deleteSelectedFilter()
		}
		return nil
	case tcell.KeyRune:
		if num, err := strconv.Atoi(string(event.Rune())); err == nil && num > 0 && num <= len(v.state.data.filters) {
			v.deleteFilterByIndex(num - 1)
			return nil
		}
	}
	return event
}

func (v *View) handleIndexInput(event *tcell.EventKey) *tcell.EventKey {
	if shortcut := v.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		v.components.indexInput.SetText(v.state.search.currentIndex)
		return nil
	case tcell.KeyEnter:
		newIndex := v.components.indexInput.GetText()
		if newIndex == "" {
			return nil
		}

		v.state.mu.Lock()
		indexChanged := newIndex != v.state.search.currentIndex
		v.state.search.currentIndex = newIndex
		v.state.mu.Unlock()

		if indexChanged {
			v.components.fieldList.Clear()
			v.components.selectedList.Clear()
			v.showLoading("Loading fields...")

			go func() {
				if err := v.loadFields(); err != nil {
					v.manager.Logger().Error("Failed to load fields for new index", "error", err)
					v.manager.App().QueueUpdateDraw(func() {
						v.manager.UpdateStatusBar(fmt.Sprintf("Error loading fields: %v", err))
					})
					return
				}

				v.manager.App().QueueUpdateDraw(func() {
					v.rebuildFieldList()
					v.manager.UpdateStatusBar("Fields loaded successfully")
				})

				v.refreshWithCurrentTimeframe()
			}()
		} else {
			v.refreshWithCurrentTimeframe()
		}
		return nil
	}
	return event
}

func (v *View) handleResultsTable(event *tcell.EventKey) *tcell.EventKey {
	if shortcut := v.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'f':
			v.toggleFieldList()
			return nil
		case 'a':
			v.manager.SetFocus(v.components.fieldList)
		case 's':
			v.manager.SetFocus(v.components.selectedList)
		}
	case tcell.KeyEnter:
		row, _ := v.components.resultsTable.GetSelection()
		if row > 0 && row <= len(v.state.data.displayedResults) {
			entry := v.state.data.displayedResults[row-1]

			v.showLoading("Fetching document...")

			go func() {
				defer v.hideLoading()

				res, err := v.service.Client.Get(
					entry.Index,
					entry.ID,
				)
				if err != nil {
					v.manager.App().QueueUpdateDraw(func() {
						v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching document: %v", err))
					})
					return
				}
				defer res.Body.Close()

				var fullDoc struct {
					Source map[string]any `json:"_source"`
				}
				if err := json.NewDecoder(res.Body).Decode(&fullDoc); err != nil {
					v.manager.App().QueueUpdateDraw(func() {
						v.manager.UpdateStatusBar(fmt.Sprintf("Error decoding document: %v", err))
					})
					return
				}

				// Display full doc
				entry.data = fullDoc.Source
				v.manager.App().QueueUpdateDraw(func() {
					v.showJSONModal(entry)
				})
			}()
		}
		return nil
	}
	return event
}

func (v *View) handleFieldList(event *tcell.EventKey) *tcell.EventKey {
	if shortcut := v.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 's':
			v.manager.SetFocus(v.components.selectedList)
		}

	case tcell.KeyEnter:
		// Field list only handles activation
		index := v.components.fieldList.GetCurrentItem()
		if index >= 0 && index < v.components.fieldList.GetItemCount() {
			mainText, _ := v.components.fieldList.GetItemText(index)
			v.toggleField(mainText)
		}
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		v.state.ui.fieldListFilter = ""
		v.filterFieldList("")
		return nil
	}
	return event
}

func (v *View) handleSelectedList(event *tcell.EventKey) *tcell.EventKey {
	if shortcut := v.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'k': // Move up
			index := v.components.selectedList.GetCurrentItem()
			if index >= 0 && index < v.components.selectedList.GetItemCount() {
				mainText, _ := v.components.selectedList.GetItemText(index)
				v.moveFieldPosition(mainText, true)
			}
			return nil
		case 'j': // Move down
			index := v.components.selectedList.GetCurrentItem()
			if index >= 0 && index < v.components.selectedList.GetItemCount() {
				mainText, _ := v.components.selectedList.GetItemText(index)
				v.moveFieldPosition(mainText, false)
			}
			return nil
		case 'a':
			v.manager.SetFocus(v.components.fieldList)
		}
	case tcell.KeyEnter:
		index := v.components.selectedList.GetCurrentItem()
		if index >= 0 && index < v.components.selectedList.GetItemCount() {
			mainText, _ := v.components.selectedList.GetItemText(index)
			v.toggleField(mainText)
		}
		return nil
	}
	return event
}

func (v *View) handleLocalFilterInput(event *tcell.EventKey) *tcell.EventKey {
	if shortcut := v.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		v.manager.SetFocus(v.components.filterInput)
		return nil
	}
	return event
}

func (v *View) handleTimeframeInput(event *tcell.EventKey) *tcell.EventKey {
	if shortcut := v.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		v.manager.SetFocus(v.components.filterInput)
		return nil
	case tcell.KeyEnter:
		v.state.search.timeframe = v.components.timeframeInput.GetText()
		v.refreshWithCurrentTimeframe()
		return nil
	}
	return event
}

func (v *View) handleNumResultsInput(event *tcell.EventKey) *tcell.EventKey {
	if shortcut := v.handleCommonShortcuts(event); shortcut == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc:
		v.manager.SetFocus(v.components.filterInput)
		return nil
	}
	return event
}

func (v *View) handleCommonShortcuts(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlA:
		v.manager.SetFocus(v.components.fieldList)
		return nil
	case tcell.KeyCtrlS:
		v.manager.SetFocus(v.components.selectedList)
		return nil
	case tcell.KeyCtrlR:
		v.manager.SetFocus(v.components.resultsTable)
		return nil
	}
	return event
}

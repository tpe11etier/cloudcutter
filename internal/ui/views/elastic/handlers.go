package elastic

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// filterInputACOpen reports whether the autocomplete dropdown is showing.
// Tab and Enter must be passed through to tview's InputField when it is,
// otherwise our global key routing consumes them before the dropdown can act.
func (v *View) filterInputACOpen() bool {
	return len(v.filterACCandidates(v.components.filterInput.GetText())) > 0
}


// filterIndices returns indices that contain text as a case-insensitive substring.
// Returns all indices when text is empty.
func (v *View) filterIndices(text string) []string {
	v.state.mu.RLock()
	indices := v.state.search.matchingIndices
	v.state.mu.RUnlock()
	if text == "" {
		return indices
	}
	lower := strings.ToLower(text)
	var matches []string
	for _, idx := range indices {
		if strings.Contains(strings.ToLower(idx), lower) {
			matches = append(matches, idx)
		}
	}
	return matches
}

// indexInputACOpen returns true when the index autocomplete dropdown is showing.
// Returns false for an exact match so Tab can cycle focus once the user has selected an index.
func (v *View) indexInputACOpen() bool {
	text := v.components.indexInput.GetText()
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	matches := v.filterIndices(text)
	for _, idx := range matches {
		if strings.ToLower(idx) == lower {
			return false // exact match — user is done, let Tab/Enter commit
		}
	}
	return len(matches) > 0
}

func (v *View) handleTabKey(currentFocus tview.Primitive) *tcell.EventKey {
	if currentFocus == v.components.filterInput && v.filterInputACOpen() {
		return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	if currentFocus == v.components.indexInput && v.indexInputACOpen() {
		return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	}
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
		if v.filterInputACOpen() {
			return event // let InputField handle autocomplete selection
		}
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

func (v *View) commitIndexInput() {
	newIndex := v.components.indexInput.GetText()
	if newIndex == "" {
		return
	}

	v.state.mu.Lock()
	indexChanged := newIndex != v.state.search.currentIndex
	v.state.search.currentIndex = newIndex
	if indexChanged {
		v.state.data.ResetFields()
	}
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
		return event // always pass through — tview handles AC selection or fires SetDoneFunc
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
		if row <= 0 {
			return nil
		}

		v.state.mu.RLock()
		currentPage := v.state.pagination.currentPage
		pageSize := v.state.pagination.pageSize
		displayedResults := v.state.data.displayedResults
		v.state.mu.RUnlock()

		// Calculate the actual index in displayedResults
		actualIndex := (currentPage-1)*pageSize + (row - 1)

		if actualIndex >= len(displayedResults) {
			return nil
		}

		entry := displayedResults[actualIndex]

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
		case 'j': // Navigate down
			currentIndex := v.components.fieldList.GetCurrentItem()
			maxIndex := v.components.fieldList.GetItemCount() - 1
			if currentIndex < maxIndex {
				v.components.fieldList.SetCurrentItem(currentIndex + 1)
			}
			return nil
		case 'k': // Navigate up
			currentIndex := v.components.fieldList.GetCurrentItem()
			if currentIndex > 0 {
				v.components.fieldList.SetCurrentItem(currentIndex - 1)
			}
			return nil
		case 's':
			v.manager.SetFocus(v.components.selectedList)
		}

	case tcell.KeyEnter:
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
		case 'j': // Navigate down
			currentIndex := v.components.selectedList.GetCurrentItem()
			maxIndex := v.components.selectedList.GetItemCount() - 1
			if currentIndex < maxIndex {
				v.components.selectedList.SetCurrentItem(currentIndex + 1)
			}
			return nil
		case 'k': // Navigate up
			currentIndex := v.components.selectedList.GetCurrentItem()
			if currentIndex > 0 {
				v.components.selectedList.SetCurrentItem(currentIndex - 1)
			}
			return nil
		case 'J': // Reorder down (move field down in order)
			index := v.components.selectedList.GetCurrentItem()
			if index >= 0 && index < v.components.selectedList.GetItemCount() {
				mainText, _ := v.components.selectedList.GetItemText(index)
				v.moveFieldPosition(mainText, false)
			}
			return nil
		case 'K': // Reorder up (move field up in order)
			index := v.components.selectedList.GetCurrentItem()
			if index >= 0 && index < v.components.selectedList.GetItemCount() {
				mainText, _ := v.components.selectedList.GetItemText(index)
				v.moveFieldPosition(mainText, true)
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
		timeframe := v.components.timeframeInput.GetText()
		if err := ValidateTimeframe(timeframe); err != nil {
			v.manager.UpdateStatusBar(fmt.Sprintf("Error: %v", err))
			return nil
		}
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

func (v *View) handleShiftTabKey(currentFocus tview.Primitive) *tcell.EventKey {
	switch currentFocus {
	case v.components.filterInput:
		v.manager.App().SetFocus(v.components.resultsTable)
	case v.components.resultsTable:
		v.manager.App().SetFocus(v.components.selectedList)
	case v.components.selectedList:
		v.manager.App().SetFocus(v.components.fieldList)
	case v.components.fieldList:
		v.manager.App().SetFocus(v.components.localFilterInput)
	case v.components.localFilterInput:
		v.manager.App().SetFocus(v.components.numResultsInput)
	case v.components.numResultsInput:
		v.manager.App().SetFocus(v.components.timeframeInput)
	case v.components.timeframeInput:
		v.manager.App().SetFocus(v.components.indexInput)
	case v.components.indexInput:
		v.manager.App().SetFocus(v.components.activeFilters)
	case v.components.activeFilters:
		v.manager.App().SetFocus(v.components.filterInput)
	default:
		v.manager.App().SetFocus(v.components.filterInput)
	}
	return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
}

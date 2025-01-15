package elastic

import (
	"context"
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
		v.manager.App().SetFocus(v.components.indexView)
	case v.components.indexView:
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
	switch event.Key() {
	case tcell.KeyEnter:
		filter := v.components.filterInput.GetText()
		if filter == "" {
			return nil
		}
		v.addFilter(filter)
		v.components.filterInput.SetText("")
		v.doRefreshWithCurrentTimeframe()
		return nil
	}
	return event
}
func (v *View) handleActiveFilters(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEsc:
		v.manager.SetFocus(v.components.filterInput)
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
	switch event.Key() {
	case tcell.KeyEnter:
		pattern := v.components.indexView.GetText()
		if pattern == "" {
			return nil
		}

		v.state.search.currentIndex = pattern

		v.resetFieldState()

		v.showLoading("Loading index stats...")

		go func() {
			// First update the stats
			if err := v.service.PreloadIndexStats(context.Background()); err != nil {
				v.manager.Logger().Error("Error refreshing index stats", err)
				v.manager.UpdateStatusBar(fmt.Sprintf("Error refreshing index stats: %v", err))
			}

			stats, err := v.service.GetIndexStats(context.Background(), pattern)
			if err != nil {
				v.manager.Logger().Error("Failed to get index stats", "error", err)
			}

			v.manager.App().QueueUpdateDraw(func() {
				v.state.search.indexStats = stats
				v.hideLoading()
				v.doRefreshWithCurrentTimeframe()
			})
		}()

		return nil
	}
	return event
}

func (v *View) resetFieldState() {
	v.state.data.originalFields = nil
	v.state.data.fieldOrder = nil
	v.state.data.activeFields = make(map[string]bool)
	v.components.fieldList.Clear()
	v.components.selectedList.Clear()
}

func (v *View) handleResultsTable(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'f':
			v.toggleFieldList()
			return nil
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
	switch event.Key() {
	case tcell.KeyEnter:
		// Field list only handles activation
		index := v.components.fieldList.GetCurrentItem()
		if index >= 0 {
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
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'k': // Move up
			index := v.components.selectedList.GetCurrentItem()
			if index >= 0 {
				mainText, _ := v.components.selectedList.GetItemText(index)
				v.moveFieldPosition(mainText, true)
			}
			return nil
		case 'j': // Move down
			index := v.components.selectedList.GetCurrentItem()
			if index >= 0 {
				mainText, _ := v.components.selectedList.GetItemText(index)
				v.moveFieldPosition(mainText, false)
			}
			return nil
		}
	case tcell.KeyEnter:
		// Selected list only handles deactivation
		index := v.components.selectedList.GetCurrentItem()
		if index >= 0 {
			mainText, _ := v.components.selectedList.GetItemText(index)
			v.toggleField(mainText) // Deactivate when Enter pressed in selected list
		}
		return nil
	}
	return event
}

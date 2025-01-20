package elastic

import (
	"fmt"
	"math"
	"strings"
)

func (v *View) addFilter(filter string) {
	if strings.TrimSpace(filter) == "" {
		return
	}

	// Check for duplicates
	for _, existing := range v.state.data.filters {
		if existing == filter {
			return
		}
	}

	v.state.data.filters = append(v.state.data.filters, filter)
	v.updateFiltersDisplay()
}

func (v *View) deleteFilterByIndex(index int) {
	if index >= 0 && index < len(v.state.data.filters) {
		v.state.data.filters = append(v.state.data.filters[:index], v.state.data.filters[index+1:]...)
		v.updateFiltersDisplay()
		v.refreshResults()

		v.updateHeader()
	}
}

func (v *View) deleteSelectedFilter() {
	row, _ := v.components.activeFilters.GetScrollOffset()
	if row < len(v.state.data.filters) {
		v.deleteFilterByIndex(row)
	}
}

func (v *View) updateFiltersDisplay() {
	if len(v.state.data.filters) == 0 {
		v.components.activeFilters.SetText("No active filters")
		return
	}

	var filters []string
	for i, filter := range v.state.data.filters {
		filters = append(filters, fmt.Sprintf("[#fabd2f]%d:[#70cae2]%s[-]", i+1, filter))
	}

	v.components.activeFilters.SetText(strings.Join(filters, " | "))
}

func (v *View) displayFilteredResults(filterText string) {
	v.state.mu.Lock()
	if v.state.data.currentFilter == filterText {
		v.state.mu.Unlock()
		return
	}
	v.state.data.currentFilter = filterText

	currentResults := make([]*DocEntry, len(v.state.data.filteredResults))
	copy(currentResults, v.state.data.filteredResults)
	v.state.mu.Unlock()

	var filtered []*DocEntry
	if filterText == "" {
		filtered = currentResults
	} else {
		filterText = strings.ToLower(filterText)
		filtered = make([]*DocEntry, 0, len(currentResults))
		for _, entry := range currentResults {
			if v.entryMatchesFilter(entry, filterText) {
				filtered = append(filtered, entry)
			}
		}
	}

	v.state.mu.Lock()
	v.state.data.displayedResults = filtered
	v.state.pagination.currentPage = 1
	v.state.pagination.totalPages = int(math.Ceil(float64(len(filtered)) / float64(v.state.pagination.pageSize)))
	v.state.mu.Unlock()

	v.displayCurrentPage()
}

func (v *View) entryMatchesFilter(entry *DocEntry, filterText string) bool {
	for _, header := range v.getActiveHeaders() {
		value := strings.ToLower(entry.GetFormattedValue(header))
		if strings.Contains(value, filterText) {
			return true
		}
	}
	return false
}

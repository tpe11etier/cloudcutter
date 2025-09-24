package elastic

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (v *View) nextPage() {
	if v.state.pagination.totalPages < 1 {
		v.state.pagination.totalPages = 1
	}

	if v.state.pagination.currentPage < v.state.pagination.totalPages {
		v.state.pagination.currentPage++
		v.displayCurrentPage()
	} else {
		v.manager.UpdateStatusBar("Already on the last page.")
	}
}

func (v *View) previousPage() {
	if v.state.pagination.totalPages < 1 {
		v.state.pagination.totalPages = 1
	}

	if v.state.pagination.currentPage > 1 {
		v.state.pagination.currentPage--
		v.displayCurrentPage()
	} else {
		v.manager.UpdateStatusBar("Already on the first page.")
	}
}

func (v *View) displayCurrentPage() {
	table := v.components.resultsTable
	oldRowOffset, oldColOffset := table.GetOffset()

	table.Clear()

	headers := v.getActiveHeaders()
	if len(headers) == 0 {
		v.manager.UpdateStatusBar("No fields selected. Select a field to see data.")
		return
	}

	displayHeaders := make([]string, 0, len(headers)+1)
	if v.state.ui.showRowNumbers {
		displayHeaders = append(displayHeaders, "#")
	}
	displayHeaders = append(displayHeaders, headers...)

	v.setupResultsTableHeaders(displayHeaders)

	v.state.mu.RLock()
	displayedResults := v.state.data.displayedResults
	currentPage := v.state.pagination.currentPage
	pageSize := v.state.pagination.pageSize
	v.state.mu.RUnlock()

	totalResults := len(displayedResults)
	if totalResults == 0 {
		v.manager.UpdateStatusBar("No results to display.")
		return
	}

	// Calculate page bounds
	start := (currentPage - 1) * pageSize
	if start >= totalResults {
		currentPage = (totalResults + pageSize - 1) / pageSize
		start = (currentPage - 1) * pageSize
	}
	end := start + pageSize
	if end > totalResults {
		end = totalResults
	}

	pageResults := displayedResults[start:end]
	numCols := len(displayHeaders)
	cells := make([][]*tview.TableCell, len(pageResults))

	for rowIdx := range pageResults {
		cells[rowIdx] = make([]*tview.TableCell, numCols)
		entry := pageResults[rowIdx]
		currentCol := 0

		if v.state.ui.showRowNumbers {
			cells[rowIdx][currentCol] = tview.NewTableCell(fmt.Sprintf("%d", start+rowIdx+1)).
				SetTextColor(tcell.ColorGray).
				SetAlign(tview.AlignRight)
			currentCol++
		}

		for _, header := range headers {
			cells[rowIdx][currentCol] = tview.NewTableCell(entry.GetFormattedValue(header)).
				SetTextColor(tcell.ColorBeige).
				SetAlign(tview.AlignLeft)
			currentCol++
		}
	}

	for rowIdx, row := range cells {
		for colIdx, cell := range row {
			table.SetCell(rowIdx+1, colIdx, cell)
		}
	}

	table.SetOffset(oldRowOffset, oldColOffset)

	v.updateStatusBar(len(pageResults))
	v.updateHeader()
}

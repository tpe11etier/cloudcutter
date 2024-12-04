package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type DataTable struct {
	*tview.Table
	headers     []string
	sortColumn  int
	sortAsc     bool
	selectable  bool
	onSelected  func(row, col int)
	headerStyle tcell.Style
	cellStyle   tcell.Style
	footerRow   int
}

func NewDataTable() *DataTable {
	dt := &DataTable{
		Table:      tview.NewTable(),
		sortColumn: -1,
		sortAsc:    true,
		selectable: true,
		headerStyle: tcell.StyleDefault.
			Background(tcell.ColorTeal).
			Foreground(tcell.ColorWhite).
			Bold(true),
		cellStyle: tcell.StyleDefault.
			Background(tcell.ColorDefault).
			Foreground(tcell.ColorWhite),
		footerRow: -1,
	}

	dt.SetBorders(false).
		SetSelectable(true, false).
		SetBorder(true).
		SetBorderColor(tcell.ColorBeige)

	// Initial message similar to view.DataTable
	dt.SetCell(0, 0, tview.NewTableCell("[turquoise::]Hurry up and select something...").
		SetTextColor(tcell.ColorDarkCyan).
		SetAlign(tview.AlignCenter).
		SetSelectable(false))

	// Set selected style to match view.DataTable
	dt.SetSelectedStyle(tcell.StyleDefault.
		Foreground(tcell.ColorBeige).
		Background(tcell.ColorBlack))

	// Add focus handling
	dt.SetFocusFunc(func() {
		dt.SetBorderColor(tcell.ColorMediumTurquoise)
	})

	dt.SetBlurFunc(func() {
		dt.SetBorderColor(tcell.ColorBeige)
	})

	// Set input capture to handle keyboard events
	//dt.SetInputCapture(dt.handleKeyboardEvents)

	return dt
}

func (dt *DataTable) SetHeaders(headers []string) {
	dt.headers = headers
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetStyle(dt.headerStyle).
			SetSelectable(false).
			SetExpansion(1) // Allow columns to expand
		dt.Table.SetCell(0, col, cell)
	}
	dt.Table.SetFixed(1, 0) // Fix header row
}

func (dt *DataTable) AddRow(values []string) {
	row := dt.GetRowCount()
	if dt.footerRow != -1 && row > dt.footerRow {
		row = dt.footerRow
		dt.footerRow++
	}

	for col, value := range values {
		cell := tview.NewTableCell(value).
			SetStyle(dt.cellStyle).
			SetExpansion(1) // Allow columns to expand
		dt.Table.SetCell(row, col, cell)
	}
}

func (dt *DataTable) Clear() {
	dt.Table.Clear()
	if len(dt.headers) > 0 {
		dt.SetHeaders(dt.headers)
	}
	dt.footerRow = -1
}

func (dt *DataTable) SetSelectedFunc(handler func(row, col int)) {
	dt.onSelected = handler
	dt.Table.SetSelectedFunc(func(row, col int) {
		if dt.onSelected != nil && row > 0 && (dt.footerRow == -1 || row < dt.footerRow) {
			dt.onSelected(row, col)
		}
	})
}

func (dt *DataTable) SetFooter(values []string) {
	if dt.footerRow == -1 {
		dt.footerRow = dt.GetRowCount()
	}

	for col, value := range values {
		cell := tview.NewTableCell(value).
			SetStyle(dt.headerStyle).
			SetSelectable(false).
			SetExpansion(1)
		dt.Table.SetCell(dt.footerRow, col, cell)
	}
}

func (dt *DataTable) SetCellAlignment(row, col int, alignment int) {
	if cell := dt.Table.GetCell(row, col); cell != nil {
		cell.SetAlign(alignment)
	}
}

func (dt *DataTable) SetColumnAlignment(col int, alignment int) {
	rowCount := dt.GetRowCount()
	for row := 0; row < rowCount; row++ {
		dt.SetCellAlignment(row, col, alignment)
	}
}

func (dt *DataTable) Setup(headers []string, alignments []int) {
	dt.SetHeaders(headers)

	// Set alignments if provided
	if len(alignments) > 0 {
		for col, alignment := range alignments {
			if col < len(headers) {
				dt.SetColumnAlignment(col, alignment)
			}
		}
	}
}

func (dt *DataTable) SetColumnExpansion(col int, expansion int) {
	rowCount := dt.GetRowCount()
	for row := 0; row < rowCount; row++ {
		if cell := dt.GetCell(row, col); cell != nil {
			cell.SetExpansion(expansion)
		}
	}
}

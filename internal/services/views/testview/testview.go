package testview

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/services/manager"
	"github.com/tpelletiersophos/cloudcutter/ui/components"
)

type View struct {
	name       string
	manager    *manager.Manager
	pages      *tview.Pages
	dataTables []*components.DataTable
	leftPanel  *components.LeftPanel
}

func NewView(manager *manager.Manager) *View {
	view := &View{
		name:    "test",
		manager: manager,
		pages:   tview.NewPages(),
	}
	view.setupLayouts()
	return view
}

func (v *View) setupLayouts() {
	// Create multiple layouts/pages
	singleTableLayout := v.createSingleTableLayout()
	dualTableLayout := v.createDualTableLayout()
	tripleTableLayout := v.createTripleTableLayout()

	v.pages.AddPage("single", singleTableLayout, true, true)
	v.pages.AddPage("dual", dualTableLayout, true, false)
	v.pages.AddPage("triple", tripleTableLayout, true, false)
}

func (v *View) createSingleTableLayout() tview.Primitive {
	table := components.NewDataTable()
	v.dataTables = append(v.dataTables, table)

	// Add sample data
	table.SetCell(0, 0, tview.NewTableCell("Header 1"))
	table.SetCell(0, 1, tview.NewTableCell("Header 2"))
	table.SetCell(1, 0, tview.NewTableCell("Data 1"))
	table.SetCell(1, 1, tview.NewTableCell("Data 2"))

	return table
}

func (v *View) createDualTableLayout() tview.Primitive {
	table1 := components.NewDataTable()
	table2 := components.NewDataTable()
	v.dataTables = append(v.dataTables, table1, table2)

	// Add sample data to table1
	table1.SetCell(0, 0, tview.NewTableCell("Table1 Header"))
	table1.SetCell(1, 0, tview.NewTableCell("Table1 Data"))

	// Add sample data to table2
	table2.SetCell(0, 0, tview.NewTableCell("Table2 Header"))
	table2.SetCell(1, 0, tview.NewTableCell("Table2 Data"))

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(table1, 0, 1, true).
		AddItem(table2, 0, 1, false)

	return flex
}

func (v *View) createTripleTableLayout() tview.Primitive {
	v.leftPanel = components.NewLeftPanel()
	table1 := components.NewDataTable()
	table2 := components.NewDataTable()
	v.dataTables = append(v.dataTables, table1, table2)

	// Add sample items to left panel
	v.leftPanel.AddItem("Item 1", "", 0, nil)
	v.leftPanel.AddItem("Item 2", "", 0, nil)

	// Add sample data to table1 and table2
	table1.SetCell(0, 0, tview.NewTableCell("Table1 Header"))
	table1.SetCell(1, 0, tview.NewTableCell("Table1 Data"))

	table2.SetCell(0, 0, tview.NewTableCell("Table2 Header"))
	table2.SetCell(1, 0, tview.NewTableCell("Table2 Data"))

	rightPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(table1, 0, 1, false).
		AddItem(table2, 0, 1, false)

	flex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(v.leftPanel, 30, 0, true).
		AddItem(rightPanel, 0, 1, false)

	return flex
}

func (v *View) Name() string {
	return v.name
}

func (v *View) GetContent() tview.Primitive {
	return v.pages
}

func (v *View) Show() {
	// Set initial focus
	v.manager.SetFocus(v.pages)
}

func (v *View) Hide() {}

func (v *View) InputHandler() func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case '1':
				v.pages.SwitchToPage("single")
				v.manager.SetFocus(v.dataTables[0]) // Focus on the table
			case '2':
				v.pages.SwitchToPage("dual")
				v.manager.SetFocus(v.dataTables[0]) // Focus on the first table
			case '3':
				v.pages.SwitchToPage("triple")
				v.manager.SetFocus(v.leftPanel) // Focus on the left panel
			}
		}
		return event
	}
}

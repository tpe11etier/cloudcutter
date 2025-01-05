package testview

//
//import (
//	"github.com/gdamore/tcell/v2"
//	"github.com/rivo/tview"
//	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
//)
//
//type View struct {
//	name      string
//	manager   *manager.Manager
//	pages     *tview.pages
//	tables    []*tview.Table
//	leftPanel *tview.List
//}
//
//func NewView(manager *manager.Manager) *View {
//	view := &View{
//		name:    "test",
//		manager: manager,
//		pages:   tview.NewPages(),
//		tables:  make([]*tview.Table, 0),
//	}
//	view.setupLayouts()
//	return view
//}
//
//func (v *View) setupLayouts() {
//	singleTableLayout := v.createSingleTableLayout()
//	dualTableLayout := v.createDualTableLayout()
//	tripleTableLayout := v.createTripleTableLayout()
//
//	v.pages.AddPage("single", singleTableLayout, true, true)
//	v.pages.AddPage("dual", dualTableLayout, true, false)
//	v.pages.AddPage("triple", tripleTableLayout, true, false)
//}
//
//func (v *View) createSingleTableLayout() tview.Primitive {
//	cfg := manager.LayoutConfig{
//		Direction: tview.FlexRow,
//		Components: []manager.Component{
//			{
//				ID:   "singleTable",
//				Type: manager.ComponentTable,
//				Style: manager.Style{
//					Border:      true,
//					BorderColor: tcell.ColorBeige,
//					Title:       " Single Table ",
//					TitleAlign:  tview.AlignCenter,
//					TitleColor:  style.GruvboxMaterial.Yellow,
//				},
//				Properties: map[string]any{
//					"selectedBackgroundColor": tcell.ColorDarkCyan,
//					"selectedTextColor":       tcell.ColorBeige,
//					"onFocus": func(table *tview.Table) {
//						table.SetBorderColor(tcell.ColorMediumTurquoise)
//					},
//					"onBlur": func(table *tview.Table) {
//						table.SetBorderColor(tcell.ColorBeige)
//					},
//				},
//			},
//		},
//	}
//
//	layout := v.manager.CreateLayout(cfg)
//	table := v.manager.GetPrimitiveByID("singleTable").(*tview.Table)
//	table.SetSelectable(true, true)
//	v.tables = append(v.tables, table)
//
//	// Add sample data
//	v.addTableCell(table, 0, 0, "Header 1")
//	v.addTableCell(table, 0, 1, "Header 2")
//	v.addTableCell(table, 1, 0, "Data 1")
//	v.addTableCell(table, 1, 1, "Data 2")
//
//	return layout
//}
//
//func (v *View) createDualTableLayout() tview.Primitive {
//	cfg := manager.LayoutConfig{
//		Direction: tview.FlexRow,
//		Components: []manager.Component{
//			{
//				ID:         "table1",
//				Type:       manager.ComponentTable,
//				Proportion: 1,
//				Style: manager.Style{
//					Border:      true,
//					BorderColor: tcell.ColorBeige,
//					Title:       " Table 1 ",
//					TitleAlign:  tview.AlignCenter,
//					TitleColor:  style.GruvboxMaterial.Yellow,
//				},
//				Properties: map[string]any{
//					"selectedBackgroundColor": tcell.ColorDarkCyan,
//					"selectedTextColor":       tcell.ColorBeige,
//					"onFocus": func(table *tview.Table) {
//						table.SetBorderColor(tcell.ColorMediumTurquoise)
//					},
//					"onBlur": func(table *tview.Table) {
//						table.SetBorderColor(tcell.ColorBeige)
//					},
//				},
//			},
//			{
//				ID:         "table2",
//				Type:       manager.ComponentTable,
//				Proportion: 1,
//				Style: manager.Style{
//					Border:      true,
//					BorderColor: tcell.ColorBeige,
//					Title:       " Table 2 ",
//					TitleAlign:  tview.AlignCenter,
//					TitleColor:  style.GruvboxMaterial.Yellow,
//				},
//				Properties: map[string]any{
//					"selectedBackgroundColor": tcell.ColorDarkCyan,
//					"selectedTextColor":       tcell.ColorBeige,
//					"onFocus": func(table *tview.Table) {
//						table.SetBorderColor(tcell.ColorMediumTurquoise)
//					},
//					"onBlur": func(table *tview.Table) {
//						table.SetBorderColor(tcell.ColorBeige)
//					},
//				},
//			},
//		},
//	}
//
//	layout := v.manager.CreateLayout(cfg)
//
//	// Setup tables and add sample data
//	for _, id := range []string{"table1", "table2"} {
//		table := v.manager.GetPrimitiveByID(id).(*tview.Table)
//		table.SetSelectable(true, true)
//		v.tables = append(v.tables, table)
//
//		v.addTableCell(table, 0, 0, id+" Header")
//		v.addTableCell(table, 1, 0, id+" Data")
//	}
//
//	return layout
//}
//
//func (v *View) createTripleTableLayout() tview.Primitive {
//	cfg := manager.LayoutConfig{
//		Direction: tview.FlexColumn,
//		Components: []manager.Component{
//			{
//				ID:        "leftPanel",
//				Type:      manager.ComponentList,
//				FixedSize: 30,
//				Style: manager.Style{
//					Border:      true,
//					BorderColor: tcell.ColorBeige,
//					Title:       " Items ",
//					TitleAlign:  tview.AlignCenter,
//					TitleColor:  style.GruvboxMaterial.Yellow,
//				},
//				Properties: map[string]any{
//					"items":                   []string{"Item 1", "Item 2"},
//					"selectedBackgroundColor": tcell.ColorDarkCyan,
//					"selectedTextColor":       tcell.ColorBeige,
//					"onFocus": func(list *tview.List) {
//						list.SetBorderColor(tcell.ColorMediumTurquoise)
//					},
//					"onBlur": func(list *tview.List) {
//						list.SetBorderColor(tcell.ColorBeige)
//					},
//				},
//			},
//			{
//				ID:         "rightPanel",
//				Type:       manager.ComponentFlex,
//				Proportion: 1,
//				Direction:  tview.FlexRow,
//				Children: []manager.Component{
//					{
//						ID:         "table3",
//						Type:       manager.ComponentTable,
//						Proportion: 1,
//						Style: manager.Style{
//							Border:      true,
//							BorderColor: tcell.ColorBeige,
//							Title:       " Table 3 ",
//							TitleAlign:  tview.AlignCenter,
//							TitleColor:  style.GruvboxMaterial.Yellow,
//						},
//						Properties: map[string]any{
//							"selectedBackgroundColor": tcell.ColorDarkCyan,
//							"selectedTextColor":       tcell.ColorBeige,
//							"onFocus": func(table *tview.Table) {
//								table.SetBorderColor(tcell.ColorMediumTurquoise)
//							},
//							"onBlur": func(table *tview.Table) {
//								table.SetBorderColor(tcell.ColorBeige)
//							},
//						},
//					},
//					{
//						ID:         "table4",
//						Type:       manager.ComponentTable,
//						Proportion: 1,
//						Style: manager.Style{
//							Border:      true,
//							BorderColor: tcell.ColorBeige,
//							Title:       " Table 4 ",
//							TitleAlign:  tview.AlignCenter,
//							TitleColor:  style.GruvboxMaterial.Yellow,
//						},
//						Properties: map[string]any{
//							"selectedBackgroundColor": tcell.ColorDarkCyan,
//							"selectedTextColor":       tcell.ColorBeige,
//							"onFocus": func(table *tview.Table) {
//								table.SetBorderColor(tcell.ColorMediumTurquoise)
//							},
//							"onBlur": func(table *tview.Table) {
//								table.SetBorderColor(tcell.ColorBeige)
//							},
//						},
//					},
//				},
//			},
//		},
//	}
//
//	layout := v.manager.CreateLayout(cfg)
//
//	// Store reference to left panel
//	v.leftPanel = v.manager.GetPrimitiveByID("leftPanel").(*tview.List)
//
//	// Setup tables and add sample data
//	for _, id := range []string{"table3", "table4"} {
//		table := v.manager.GetPrimitiveByID(id).(*tview.Table)
//		table.SetSelectable(true, true)
//		v.tables = append(v.tables, table)
//
//		v.addTableCell(table, 0, 0, id+" Header")
//		v.addTableCell(table, 1, 0, id+" Data")
//	}
//
//	return layout
//}
//
//// Helper function to add table cells with consistent styling
//func (v *View) addTableCell(table *tview.Table, row, col int, text string) {
//	cell := tview.NewTableCell(text).
//		SetTextColor(tcell.ColorBeige).
//		SetAlign(tview.AlignLeft).
//		SetSelectable(true)
//
//	// Make headers stand out
//	if row == 0 {
//		cell.SetTextColor(style.GruvboxMaterial.Yellow).
//			SetAlign(tview.AlignCenter).
//			SetAttributes(tcell.AttrBold)
//	}
//
//	table.SetCell(row, col, cell)
//}
//
//func (v *View) ActiveField() string {
//	currentFocus := v.manager.App.GetFocus()
//	switch currentFocus {
//	case v.leftPanel:
//		return "leftPanel"
//	default:
//		return ""
//	}
//}
//
//func (v *View) Name() string {
//	return v.name
//}
//
//func (v *View) Content() tview.Primitive {
//	return v.pages
//}
//
//func (v *View) Render() {
//	v.manager.SetFocus(v.pages)
//}
//
//func (v *View) Hide() {}
//
//func (v *View) InputHandler() func(event *tcell.EventKey) *tcell.EventKey {
//	return func(event *tcell.EventKey) *tcell.EventKey {
//		switch event.Key() {
//		case tcell.KeyRune:
//			switch event.Rune() {
//			case '1':
//				v.pages.SwitchToPage("single")
//				v.manager.SetFocus(v.tables[0])
//			case '2':
//				v.pages.SwitchToPage("dual")
//				v.manager.SetFocus(v.tables[1])
//			case '3':
//				v.pages.SwitchToPage("triple")
//				v.manager.SetFocus(v.leftPanel)
//			}
//		}
//		return event
//	}
//}

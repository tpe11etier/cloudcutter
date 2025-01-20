package elastic

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/types"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/help"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/style"
	"strconv"
)

func (v *View) setupLayout() {
	cfg := types.LayoutConfig{
		Direction: tview.FlexRow,
		Components: []types.Component{
			// Control Panel
			{
				Type:      types.ComponentFlex,
				Direction: tview.FlexRow,
				FixedSize: 12,
				Children: []types.Component{
					// Filter Input
					{
						ID:        "filterInput",
						Type:      types.ComponentInputField,
						FixedSize: 3,
						Style: types.InputFieldStyle{
							BaseStyle: types.BaseStyle{
								Border:      true,
								BorderColor: tcell.ColorBeige,
								TitleAlign:  tview.AlignLeft,
							},
							LabelColor:           tcell.ColorMediumTurquoise,
							FieldBackgroundColor: tcell.ColorBlack,
							FieldTextColor:       tcell.ColorBeige,
						},
						Properties: types.InputFieldProperties{
							Label:      " ES Filter >_ ",
							FieldWidth: 0,
							OnFocus: func(inputField *tview.InputField) {
								inputField.SetBorderColor(tcell.ColorMediumTurquoise)
							},
							OnBlur: func(inputField *tview.InputField) {
								inputField.SetBorderColor(tcell.ColorBeige)
							},
						},
					},
					// Active Filters
					{
						ID:        "activeFilters",
						Type:      types.ComponentTextView,
						FixedSize: 3,
						Style: types.TextViewStyle{
							BaseStyle: types.BaseStyle{
								Border:      true,
								BorderColor: tcell.ColorBeige,
								Title:       " Active Filters (Delete/Backspace to remove all, or press filter number) ",
								TitleColor:  style.GruvboxMaterial.Yellow,
								TextColor:   tcell.ColorBeige,
							},
						},
						Properties: types.TextViewProperties{
							Text:          "No active filters",
							DynamicColors: true,
							OnFocus: func(textView *tview.TextView) {
								textView.SetBorderColor(tcell.ColorMediumTurquoise)
							},
							OnBlur: func(textView *tview.TextView) {
								textView.SetBorderColor(tcell.ColorBeige)
							},
						},
					},
					// Index and Timeframe Row
					{
						Type:      types.ComponentFlex,
						Direction: tview.FlexColumn,
						FixedSize: 3,
						Children: []types.Component{
							// Index Input
							{
								ID:         "indexInput",
								Type:       types.ComponentInputField,
								Proportion: 1,
								Style: types.InputFieldStyle{
									BaseStyle: types.BaseStyle{
										Border:      true,
										BorderColor: tcell.ColorBeige,
										Title:       " Index ",
										TitleAlign:  tview.AlignCenter,
										TitleColor:  style.GruvboxMaterial.Yellow,
									},
									LabelColor:           tcell.ColorMediumTurquoise,
									FieldBackgroundColor: tcell.ColorBlack,
									FieldTextColor:       tcell.ColorBeige,
								},
								Properties: types.InputFieldProperties{
									Label:      ">_ ",
									FieldWidth: 0,
									Text:       v.state.search.currentIndex,
									OnFocus: func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorMediumTurquoise)
										if helpCategory := v.manager.Help().GetContextHelp(); helpCategory != nil {
											helpCategory.Commands = getIndices(v)
											v.manager.Help().SetContextHelp(helpCategory)
										}
									},
									OnBlur: func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorBeige)
									},
								},
								Help: getIndices(v),
							},
							// Timeframe Input
							{
								ID:         "timeframeInput",
								Type:       types.ComponentInputField,
								Proportion: 1,
								Style: types.InputFieldStyle{
									BaseStyle: types.BaseStyle{
										Border:      true,
										BorderColor: tcell.ColorBeige,
										Title:       " Timeframe ",
										TitleAlign:  tview.AlignCenter,
										TitleColor:  style.GruvboxMaterial.Yellow,
									},
									LabelColor:           tcell.ColorMediumTurquoise,
									FieldBackgroundColor: tcell.ColorBlack,
									FieldTextColor:       tcell.ColorBeige,
								},
								Properties: types.InputFieldProperties{
									Label:      ">_ ",
									FieldWidth: 0,
									Text:       "today",
									OnFocus: func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorMediumTurquoise)
									},
									OnBlur: func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorBeige)
									},
									DoneFunc: func(s string) {
										if s == "" {
											return
										}
										// Only do the reset if it's a truly new index:
										if s != v.state.search.currentIndex {
											v.state.search.currentIndex = s
											v.resetFieldState()
											v.doRefreshWithCurrentTimeframe()
										}
									},
								},
								Help: []help.Command{
									{Key: "12h", Description: "Last 12 hours"},
									{Key: "24h", Description: "Last 24 hours"},
									{Key: "7d", Description: "Last 7 days"},
									{Key: "30d", Description: "Last 30 days"},
									{Key: "today", Description: "Since start of day"},
									{Key: "week", Description: "Last week"},
									{Key: "month", Description: "Last month"},
									{Key: "quarter", Description: "Last quarter"},
									{Key: "year", Description: "Last year"},
									{Key: "Enter", Description: "Apply timeframe"},
								},
							},
							{
								ID:         "numResultsInput",
								Type:       types.ComponentInputField,
								Proportion: 1,
								Style: types.InputFieldStyle{
									BaseStyle: types.BaseStyle{
										Border:      true,
										BorderColor: tcell.ColorBeige,
										Title:       " # Results ",
										TitleAlign:  tview.AlignCenter,
										TitleColor:  style.GruvboxMaterial.Yellow,
									},
									LabelColor:           tcell.ColorMediumTurquoise,
									FieldBackgroundColor: tcell.ColorBlack,
									FieldTextColor:       tcell.ColorBeige,
								},
								Properties: types.InputFieldProperties{
									Label:      ">_ ",
									FieldWidth: 0,
									Text:       strconv.Itoa(v.state.search.numResults),
									OnFocus: func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorMediumTurquoise)
									},
									OnBlur: func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorBeige)
									},
									DoneFunc: func(s string) {
										if num, err := strconv.Atoi(s); err == nil && num > 0 {
											v.state.search.numResults = num
											v.refreshResults()
										}
									},
								},
								Help: []help.Command{
									{Key: "Enter", Description: "Apply number of results"},
								},
							},
						},
					},
					// Local Filter Input
					{
						ID:        "localFilterInput",
						Type:      types.ComponentInputField,
						FixedSize: 3,
						Style: types.InputFieldStyle{
							BaseStyle: types.BaseStyle{
								Border:      true,
								BorderColor: tcell.ColorBeige,
								TitleAlign:  tview.AlignLeft,
								Title:       " Filter Results ",
								TitleColor:  style.GruvboxMaterial.Yellow,
							},
							LabelColor:           tcell.ColorMediumTurquoise,
							FieldBackgroundColor: tcell.ColorBlack,
							FieldTextColor:       tcell.ColorBeige,
						},
						Properties: types.InputFieldProperties{
							Label:      ">_ ",
							FieldWidth: 0,
							OnFocus: func(inputField *tview.InputField) {
								inputField.SetBorderColor(tcell.ColorMediumTurquoise)
							},
							OnBlur: func(inputField *tview.InputField) {
								inputField.SetBorderColor(tcell.ColorBeige)
							},
						},
					},
				},
			},
			// Results Section
			{
				ID:         "resultsFlex",
				Type:       types.ComponentFlex,
				Direction:  tview.FlexColumn,
				Proportion: 3,
				Children: []types.Component{
					// Left side - Fields lists
					{
						ID:        "listsContainer",
						Type:      types.ComponentFlex,
						Direction: tview.FlexRow,
						FixedSize: 50,
						Children: []types.Component{
							{
								ID:         "fieldList",
								Type:       types.ComponentList,
								Proportion: 1,
								Style: types.ListStyle{
									BaseStyle: types.BaseStyle{
										Border:      true,
										BorderColor: tcell.ColorBeige,
										Title:       "(A)vailable Fields (Enter to select)",
										TitleColor:  style.GruvboxMaterial.Yellow,
										TextColor:   tcell.ColorBeige,
									},
									SelectedTextColor:       tcell.ColorBeige,
									SelectedBackgroundColor: tcell.ColorDarkCyan,
								},
								Properties: types.ListProperties{
									OnFocus: func(list *tview.List) {
										list.SetBorderColor(tcell.ColorMediumTurquoise)
									},
									OnBlur: func(list *tview.List) {
										list.SetBorderColor(tcell.ColorBeige)
									},
								},
							},
							{
								ID:         "selectedList",
								Type:       types.ComponentList,
								Proportion: 1,
								Style: types.ListStyle{
									BaseStyle: types.BaseStyle{
										Border:      true,
										BorderColor: tcell.ColorBeige,
										Title:       "(S)elected Fields (j↓/k↑ to order)",
										TitleColor:  style.GruvboxMaterial.Yellow,
										TextColor:   tcell.ColorBeige,
									},
									SelectedTextColor:       tcell.ColorBeige,
									SelectedBackgroundColor: tcell.ColorDarkCyan,
								},
								Properties: types.ListProperties{
									OnFocus: func(list *tview.List) {
										list.SetBorderColor(tcell.ColorMediumTurquoise)
									},
									OnBlur: func(list *tview.List) {
										list.SetBorderColor(tcell.ColorBeige)
									},
								},
							},
						},
					},
					// Right side - Results table
					{
						ID:         "resultsTable",
						Type:       types.ComponentTable,
						Proportion: 1,
						Style: types.TableStyle{
							BaseStyle: types.BaseStyle{
								Border:      true,
								BorderColor: tcell.ColorBeige,
							},
							SelectedTextColor:       tcell.ColorBeige,
							SelectedBackgroundColor: tcell.ColorDarkCyan,
						},
						Properties: types.TableProperties{
							OnFocus: func(table *tview.Table) {
								table.SetBorderColor(tcell.ColorMediumTurquoise)
							},
							OnBlur: func(table *tview.Table) {
								table.SetBorderColor(tcell.ColorBeige)
							},
						},
					},
				},
			},
		},
	}

	v.components.content = v.manager.CreateLayout(cfg).(*tview.Flex)
	pages := v.manager.Pages()
	pages.AddPage("elastic", v.components.content, true, true)

	v.components.filterInput = v.manager.GetPrimitiveByID("filterInput").(*tview.InputField)
	v.components.activeFilters = v.manager.GetPrimitiveByID("activeFilters").(*tview.TextView)
	v.components.indexInput = v.manager.GetPrimitiveByID("indexInput").(*tview.InputField)
	v.components.localFilterInput = v.manager.GetPrimitiveByID("localFilterInput").(*tview.InputField)
	v.components.timeframeInput = v.manager.GetPrimitiveByID("timeframeInput").(*tview.InputField)
	v.components.numResultsInput = v.manager.GetPrimitiveByID("numResultsInput").(*tview.InputField)
	v.components.resultsFlex = v.manager.GetPrimitiveByID("resultsFlex").(*tview.Flex)
	v.components.fieldList = v.manager.GetPrimitiveByID("fieldList").(*tview.List)
	v.components.selectedList = v.manager.GetPrimitiveByID("selectedList").(*tview.List)
	v.components.resultsTable = v.manager.GetPrimitiveByID("resultsTable").(*tview.Table)
	v.components.listsContainer = v.manager.GetPrimitiveByID("listsContainer").(*tview.Flex)

	v.components.localFilterInput.SetChangedFunc(func(text string) {
		v.displayFilteredResults(text)
	})

}

func (v *View) updateHeader() {
	summary := make([]types.SummaryItem, 0, 5)

	var indexInfo string
	if stats := v.state.search.indexStats; stats != nil {
		var healthColor = style.GruvboxMaterial.Red
		switch stats.Health {
		case "green":
			healthColor = style.GruvboxMaterial.Green
		case "yellow":
			healthColor = style.GruvboxMaterial.Yellow
		}

		indexInfo = fmt.Sprintf("%s ([%s]%s[-]) | %s docs | %s",
			v.state.search.currentIndex,
			healthColor,
			stats.Health,
			stats.DocsCount,
			stats.StoreSize,
		)
	} else {
		indexInfo = v.state.search.currentIndex
	}

	summary = append(summary,
		types.SummaryItem{Key: "Index", Value: indexInfo},
		types.SummaryItem{Key: "Filters", Value: fmt.Sprintf("%d", len(v.state.data.filters))},
		types.SummaryItem{Key: "Results", Value: fmt.Sprintf("%d", len(v.state.data.displayedResults))},
		types.SummaryItem{Key: "Page", Value: fmt.Sprintf("[%s::b]%d/%d[-]", style.GruvboxMaterial.Yellow, v.state.pagination.currentPage, v.state.pagination.totalPages)},
		types.SummaryItem{Key: "Timeframe", Value: v.components.timeframeInput.GetText()},
	)

	v.manager.UpdateHeader(summary)
}

func (v *View) updateResultsLayout() {
	resultsFlex := v.components.resultsFlex
	if resultsFlex == nil {
		return
	}

	resultsFlex.RemoveItem(v.components.listsContainer)
	resultsFlex.RemoveItem(v.components.resultsTable)

	if v.state.ui.fieldListVisible {
		resultsFlex.AddItem(v.components.listsContainer, 50, 0, false).
			AddItem(v.components.resultsTable, 0, 1, true)
	} else {
		resultsFlex.AddItem(v.components.resultsTable, 0, 1, true)
	}
}

func (v *View) updateStatusBar(currentPageSize int) {
	filterText := v.components.localFilterInput.GetText()
	statusMsg := fmt.Sprintf("Page %d/%d | Showing %d of %d logs",
		v.state.pagination.currentPage,
		v.state.pagination.totalPages,
		currentPageSize,
		len(v.state.data.displayedResults))

	if filterText != "" {
		statusMsg += fmt.Sprintf(" (filtered: %q)", filterText)
	}

	if v.state.ui.showRowNumbers {
		statusMsg += fmt.Sprintf(" | [%s]Row numbers: on (press 'r' to toggle)[-]",
			style.GruvboxMaterial.Yellow)
	}

	v.manager.UpdateStatusBar(statusMsg)
}

func (v *View) setupResultsTableHeaders(headers []string) {
	table := v.components.resultsTable
	table.Clear()
	table.SetSelectable(true, false)

	if v.state.ui.showRowNumbers {
		table.SetFixed(1, 1)
	} else {
		table.SetFixed(1, 0)
	}

	for col, header := range headers {
		table.SetCell(0, col,
			tview.NewTableCell(header).
				SetTextColor(style.GruvboxMaterial.Yellow).
				SetAlign(tview.AlignCenter).
				SetSelectable(false).
				SetAttributes(tcell.AttrBold))
	}
}

func (v *View) toggleRowNumbers() {
	v.state.ui.showRowNumbers = !v.state.ui.showRowNumbers
	v.displayCurrentPage() // No need for full refresh
}

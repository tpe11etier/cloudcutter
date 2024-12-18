package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
	components2 "github.com/tpelletiersophos/cloudcutter/internal/ui/components"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/help"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/types"
	"sort"
	"strconv"
	"strings"
)

type View struct {
	manager    *manager.Manager
	components viewComponents
	service    *elastic.Service
	state      viewState
	layout     tview.Primitive
}

type viewComponents struct {
	content          *tview.Flex
	filterInput      *tview.InputField
	activeFilters    *tview.TextView
	indexView        *tview.InputField
	selectedView     *tview.TextView
	fieldList        *tview.List
	resultsTable     *tview.Table
	localFilterInput *tview.InputField
	timeframeInput   *tview.InputField
}

type viewState struct {
	activeFields    map[string]bool
	filters         []string
	filterMode      bool
	currentIndex    string
	matchingIndices []string
	currentResults  []*LogEntry
	originalFields  []string
	fieldListFilter string
	currentFilter   string
	fieldMatches    []string
	fieldOrder      []string
}

func NewView(manager *manager.Manager, esClient *elastic.Service, defaultIndex string) (*View, error) {
	v := &View{
		manager: manager,
		service: esClient,
		state: viewState{
			activeFields:    make(map[string]bool),
			filters:         make([]string, 0),
			currentIndex:    defaultIndex,
			matchingIndices: make([]string, 0),
		},
	}

	//v.components.pages = tview.NewPages()
	v.setupLayout()
	manager.SetFocus(v.components.filterInput)
	return v, nil
}

func (v *View) setupLayout() {
	cfg := types.LayoutConfig{
		Direction: tview.FlexRow,
		Components: []types.Component{
			{
				// Control Panel section
				Type:      types.ComponentFlex,
				Direction: tview.FlexRow,
				FixedSize: 15,
				Children: []types.Component{
					{
						ID:        "filterInput",
						Type:      types.ComponentInputField,
						FixedSize: 3,
						Style: types.Style{
							Border:      true,
							BorderColor: tcell.ColorBeige,
							TitleAlign:  tview.AlignLeft,
						},
						Properties: map[string]any{
							"label":                " ES Filter >_ ",
							"labelColor":           tcell.ColorMediumTurquoise,
							"fieldBackgroundColor": tcell.ColorBlack,
							"fieldTextColor":       tcell.ColorBeige,
							"fieldWidth":           0,
							"onFocus": func(inputField *tview.InputField) {
								inputField.SetBorderColor(tcell.ColorMediumTurquoise)
							},
							"onBlur": func(inputField *tview.InputField) {
								inputField.SetBorderColor(tcell.ColorBeige)
							},
						},
					},
					{
						ID:        "localFilterInput",
						Type:      types.ComponentInputField,
						FixedSize: 3,
						Style: types.Style{
							Border:      true,
							BorderColor: tcell.ColorBeige,
							TitleAlign:  tview.AlignLeft,
							Title:       " Filter Results ",
							TitleColor:  tcell.ColorYellow,
						},
						Properties: map[string]any{
							"label":                ">_ ",
							"labelColor":           tcell.ColorMediumTurquoise,
							"fieldBackgroundColor": tcell.ColorBlack,
							"fieldTextColor":       tcell.ColorBeige,
							"fieldWidth":           0,
							"onFocus": func(inputField *tview.InputField) {
								inputField.SetBorderColor(tcell.ColorMediumTurquoise)
							},
							"onBlur": func(inputField *tview.InputField) {
								inputField.SetBorderColor(tcell.ColorBeige)
							},
						},
					},
					{
						ID:        "activeFilters",
						Type:      types.ComponentTextView,
						FixedSize: 3,
						Style: types.Style{
							Border:      true,
							BorderColor: tcell.ColorBeige,
							Title:       " Active Filters (Delete/Backspace to remove all, or press filter number) ",
							TitleColor:  tcell.ColorYellow,
						},
						Properties: map[string]any{
							"dynamicColors": true,
							"text":          "No active filters",
							"onFocus": func(textView *tview.TextView) {
								textView.SetBorderColor(tcell.ColorMediumTurquoise)
							},
							"onBlur": func(textView *tview.TextView) {
								textView.SetBorderColor(tcell.ColorBeige)
							},
						},
					},
					{
						Type:      types.ComponentFlex,
						Direction: tview.FlexColumn,
						FixedSize: 3,
						Children: []types.Component{
							{
								ID:         "indexView",
								Type:       types.ComponentInputField,
								Proportion: 2,
								Style: types.Style{
									Border:      true,
									BorderColor: tcell.ColorBeige,
									Title:       " Index ",
									TitleAlign:  tview.AlignCenter,
									TitleColor:  tcell.ColorYellow,
								},
								Properties: map[string]any{
									"label":                ">_ ",
									"labelColor":           tcell.ColorMediumTurquoise,
									"fieldBackgroundColor": tcell.ColorBlack,
									"fieldTextColor":       tcell.ColorBeige,
									"fieldWidth":           0,
									"text":                 v.state.currentIndex,
									"onFocus": func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorMediumTurquoise)
									},
									"onBlur": func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorBeige)
									},
								},
							},
							{
								ID:         "timeframeInput",
								Type:       types.ComponentInputField,
								Proportion: 1,
								Style: types.Style{
									Border:      true,
									BorderColor: tcell.ColorBeige,
									Title:       " Timeframe ",
									TitleAlign:  tview.AlignCenter,
									TitleColor:  tcell.ColorYellow,
								},
								Properties: map[string]any{
									"label":                ">_ ",
									"labelColor":           tcell.ColorMediumTurquoise,
									"fieldBackgroundColor": tcell.ColorBlack,
									"fieldTextColor":       tcell.ColorBeige,
									"fieldWidth":           0,
									"text":                 "12h",
									"onFocus": func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorMediumTurquoise)
									},
									"onBlur": func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorBeige)
									},
								},
								Help: []help.Command{
									{Key: "12h", Description: "Last 12 hours"},
									{Key: "24h", Description: "Last 24 hours"},
									{Key: "7d", Description: "Last 7 days"},
									{Key: "30d", Description: "Last 30 days"},
									{Key: "Enter", Description: "Apply timeframe"},
								},
							},
						},
					},
					{
						ID:        "selectedView",
						Type:      types.ComponentTextView,
						FixedSize: 3,
						Style: types.Style{
							Border:      true,
							BorderColor: tcell.ColorBeige,
							Title:       " Selected Fields ",
							TitleAlign:  tview.AlignCenter,
							TitleColor:  tcell.ColorYellow,
						},
						Properties: map[string]any{
							"items":         []string{},
							"dynamicColors": true,
							"onFocus": func(textView *tview.TextView) {
								textView.SetBorderColor(tcell.ColorMediumTurquoise)
							},
							"onBlur": func(textView *tview.TextView) {
								textView.SetBorderColor(tcell.ColorBeige)
							},
						},
					},
				},
			},
			{
				Type:       types.ComponentFlex,
				Direction:  tview.FlexColumn,
				Proportion: 1,
				Children: []types.Component{
					{
						ID:        "fieldList",
						Type:      types.ComponentList,
						FixedSize: 50,
						Style: types.Style{
							Border:      true,
							BorderColor: tcell.ColorBeige,
							Title:       " Available Fields ",
							TitleColor:  tcell.ColorYellow,
						},
						Properties: map[string]any{
							"selectedBackgroundColor": tcell.ColorDarkCyan,
							"selectedTextColor":       tcell.ColorBeige,
							"showSecondaryText":       false,
							"onFocus": func(list *tview.List) {
								list.SetBorderColor(tcell.ColorMediumTurquoise)
							},
							"onBlur": func(list *tview.List) {
								list.SetBorderColor(tcell.ColorBeige)
							},
						},
					},
					{
						ID:         "resultsTable",
						Type:       types.ComponentTable,
						Proportion: 1,
						Style: types.Style{
							Border:      true,
							BorderColor: tcell.ColorBeige,
						},
						Properties: map[string]any{
							"selectedBackgroundColor": tcell.ColorDarkCyan,
							"selectedTextColor":       tcell.ColorBeige,
							"onFocus": func(table *tview.Table) {
								table.SetBorderColor(tcell.ColorMediumTurquoise)
							},
							"onBlur": func(table *tview.Table) {
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
	v.components.indexView = v.manager.GetPrimitiveByID("indexView").(*tview.InputField)
	v.components.selectedView = v.manager.GetPrimitiveByID("selectedView").(*tview.TextView)
	v.components.fieldList = v.manager.GetPrimitiveByID("fieldList").(*tview.List)
	v.components.resultsTable = v.manager.GetPrimitiveByID("resultsTable").(*tview.Table)
	v.components.localFilterInput = v.manager.GetPrimitiveByID("localFilterInput").(*tview.InputField)
	v.components.timeframeInput = v.manager.GetPrimitiveByID("timeframeInput").(*tview.InputField)

	v.components.localFilterInput.SetChangedFunc(func(text string) {
		v.displayFilteredResults(text)
	})

	v.initFields()
	v.service.ListIndices(context.Background(), "*")
}

func (v *View) Name() string {
	return "elastic"
}

func (v *View) Content() tview.Primitive {
	return v.layout
}

func (v *View) Show() {
	v.refreshResults()
	v.manager.App().SetFocus(v.components.filterInput)
}

func (v *View) Hide() {}

func (v *View) ActiveField() string {
	currentFocus := v.manager.App().GetFocus()
	switch currentFocus {
	case v.components.filterInput:
		return "filterInput"
	case v.components.indexView:
		return "indexView"
	case v.components.localFilterInput:
		return "localFilterInput"
	case v.components.timeframeInput:
		return "timeframeInput"
	default:
		return ""
	}
}
func (v *View) InputHandler() func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		currentFocus := v.manager.App().GetFocus()

		switch event.Key() {
		case tcell.KeyTab:
			return v.handleTabKey(currentFocus)
		case tcell.KeyRune:
			if event.Rune() == '~' {
				v.manager.App().SetFocus(v.components.indexView)
				v.showIndexSelector()
				return nil
			}
		case tcell.KeyEsc:
			v.manager.App().SetFocus(v.components.filterInput)
			return nil
		}

		switch currentFocus {
		case v.components.filterInput:
			return v.handleFilterInput(event)
		case v.components.activeFilters:
			return v.handleActiveFilters(event)
		case v.components.indexView:
			return v.handleIndexInput(event)
		case v.components.fieldList:
			return v.handleFieldList(event)
		case v.components.localFilterInput:
			return event
		}

		return event
	}
}

func (v *View) handleTabKey(currentFocus tview.Primitive) *tcell.EventKey {
	switch currentFocus {
	case v.components.filterInput:
		v.manager.App().SetFocus(v.components.localFilterInput)
	case v.components.localFilterInput:
		v.manager.App().SetFocus(v.components.activeFilters)
	case v.components.activeFilters:
		v.manager.App().SetFocus(v.components.indexView)
	case v.components.indexView:
		v.manager.App().SetFocus(v.components.timeframeInput)
	case v.components.timeframeInput:
		v.manager.App().SetFocus(v.components.fieldList)
	case v.components.fieldList:
		v.manager.App().SetFocus(v.components.resultsTable)
	case v.components.resultsTable:
		v.manager.App().SetFocus(v.components.filterInput)
	default:
		v.manager.App().SetFocus(v.components.filterInput)
	}
	return nil
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
		v.refreshResults()
		return nil
	}
	return event
}

func (v *View) handleActiveFilters(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyDelete, tcell.KeyBackspace2, tcell.KeyBackspace:
		if len(v.state.filters) > 0 {
			v.deleteSelectedFilter()
		}
		return nil
	case tcell.KeyRune:
		if num, err := strconv.Atoi(string(event.Rune())); err == nil && num > 0 && num <= len(v.state.filters) {
			v.deleteFilterByIndex(num - 1)
			return nil
		}
	}
	return event
}

func (v *View) addFilter(filter string) {
	if strings.TrimSpace(filter) == "" {
		return
	}

	for _, existing := range v.state.filters {
		if existing == filter {
			return
		}
	}

	v.state.filters = append(v.state.filters, filter)
	v.updateFiltersDisplay()
}

func (v *View) deleteFilterByIndex(index int) {
	if index >= 0 && index < len(v.state.filters) {
		v.state.filters = append(v.state.filters[:index], v.state.filters[index+1:]...)
		v.updateFiltersDisplay()
		v.refreshResults()
	}
}

func (v *View) deleteSelectedFilter() {
	row, _ := v.components.activeFilters.GetScrollOffset()
	if row < len(v.state.filters) {
		v.deleteFilterByIndex(row)
	}
}

func (v *View) updateSelectedFieldsDisplay() {
	var selectedFields []string
	for field, active := range v.state.activeFields {
		if active {
			selectedFields = append(selectedFields, "[yellow]"+field+"[-]")
		}
	}

	if len(selectedFields) == 0 {
		v.components.selectedView.SetText("No fields selected")
	} else {
		v.components.selectedView.SetText(strings.Join(selectedFields, " | "))
	}
}

func (v *View) updateFiltersDisplay() {
	if len(v.state.filters) == 0 {
		v.components.activeFilters.SetText("No active filters")
		return
	}

	var filters []string
	for i, filter := range v.state.filters {
		filters = append(filters, fmt.Sprintf("[#fabd2f]%d:[#70cae2]%s[-]", i+1, filter))
	}

	v.components.activeFilters.SetText(strings.Join(filters, " | "))
}

func (v *View) buildQuery() map[string]any {
	timeframe := v.components.timeframeInput.GetText()
	if timeframe == "" {
		timeframe = "12h" // default
	}
	must := []any{
		map[string]any{
			"range": map[string]any{
				"unixTime": map[string]any{
					"gte": fmt.Sprintf("now-%s", timeframe),
					"lte": "now",
				},
			},
		},
	}

	for _, filter := range v.state.filters {
		var parts []string
		if strings.Contains(filter, "=") {
			parts = strings.SplitN(filter, "=", 2)
		} else if strings.Contains(filter, ":") {
			parts = strings.SplitN(filter, ":", 2)
		}

		if len(parts) == 2 {
			field := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			must = append(must, map[string]any{
				"term": map[string]any{
					field: value,
				},
			})
		}
	}

	return map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": must,
			},
		},
		"sort": []map[string]any{
			{
				"unixTime": map[string]any{
					"order": "desc",
				},
			},
		},
		"size": 100,
	}
}
func (v *View) showIndexSelector() {
	if len(v.state.matchingIndices) == 0 {
		v.service.ListIndices(context.Background(), "*")
	}

	list := tview.NewList()
	list.ShowSecondaryText(false).
		SetMainTextColor(tcell.ColorBeige).
		SetBorder(true).
		SetTitle(" Select Index ").
		SetTitleAlign(tview.AlignLeft)
	list.SetSelectedBackgroundColor(tcell.ColorDarkCyan).
		SetSelectedTextColor(tcell.ColorBeige)

	for _, index := range v.state.matchingIndices {
		indexName := index
		list.AddItem(indexName, "", 0, func() {
			v.state.currentIndex = indexName
			v.components.indexView.SetText(indexName)
			v.refreshResults()
			v.manager.App().SetRoot(v.components.content, true)
		})
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			v.manager.App().SetRoot(v.components.content, true)
			v.manager.App().SetFocus(v.components.indexView)
			return nil
		}
		return event
	})

	modalGrid := tview.NewGrid().
		SetRows(0, 10, 0).
		SetColumns(0, 50, 0).
		SetBorders(false).
		AddItem(list, 1, 1, 1, 1, 0, 0, true)

	v.manager.App().SetRoot(modalGrid, true)
	v.manager.App().SetFocus(list)
}

func (v *View) handleIndexInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		pattern := v.components.indexView.GetText()
		if pattern != "" {
			v.state.currentIndex = pattern
			v.refreshResults()
		}
		return nil
	case tcell.KeyCtrlI:
		v.showIndexSelector()
		return nil
	}
	return event
}

func (v *View) initFields() {
	query := map[string]any{
		"size": 10,
		"sort": []map[string]any{
			{
				"unixTime": map[string]any{
					"order": "desc",
				},
			},
		},
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error creating query: %v", err))
		return
	}

	res, err := v.service.Client.Search(
		v.service.Client.Search.WithIndex(v.state.currentIndex),
		v.service.Client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Search error: %v", err))
		return
	}
	defer res.Body.Close()

	var result struct {
		Hits struct {
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error decoding response: %v", err))
		return
	}

	fieldSet := make(map[string]bool)

	for _, hit := range result.Hits.Hits {
		entry, err := NewLogEntry(hit.Source)
		if err != nil {
			continue
		}

		fields := entry.GetAvailableFields()
		for _, field := range fields {
			fieldSet[field] = true
		}
	}

	var fields []string
	for field := range fieldSet {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	v.state.originalFields = fields
	v.state.fieldOrder = make([]string, len(v.state.originalFields))
	copy(v.state.fieldOrder, v.state.originalFields)

	// Now populate the UI list once using fieldOrder
	for _, field := range v.state.fieldOrder {
		fieldName := field
		v.components.fieldList.AddItem(fieldName, "", 0, func() {
			v.toggleField(fieldName)
		})
	}

	v.manager.UpdateStatusBar(fmt.Sprintf("Found %d available fields", len(fields)))
}

func (v *View) displayFilteredResults(filterText string) {
	v.components.resultsTable.Clear()

	// Get active headers
	var headers []string
	for _, field := range v.state.fieldOrder {
		if v.state.activeFields[field] {
			headers = append(headers, field)
		}
	}

	if len(headers) == 0 {
		v.manager.UpdateStatusBar("No fields selected. Select a field to see data.")
		return
	}

	v.components.resultsTable.SetFixed(1, 0)
	for col, header := range headers {
		v.components.resultsTable.SetCell(0, col,
			tview.NewTableCell(header).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignCenter).
				SetSelectable(false).
				SetAttributes(tcell.AttrBold))
	}

	filterText = strings.ToLower(filterText)
	row := 1
	matches := 0
	for _, entry := range v.state.currentResults {
		// Check if row matches local filter
		if filterText != "" {
			rowText := ""
			for _, header := range headers {
				rowText += " " + strings.ToLower(entry.GetFormattedValue(header))
			}
			if !strings.Contains(rowText, filterText) {
				continue
			}
		}

		for col, header := range headers {
			value := entry.GetFormattedValue(header)
			v.components.resultsTable.SetCell(row, col,
				tview.NewTableCell(value).
					SetTextColor(tcell.ColorBeige).
					SetAlign(tview.AlignLeft).
					SetSelectable(true))
		}
		row++
		matches++
	}

	v.manager.UpdateStatusBar(fmt.Sprintf("Showing %d of %d logs (local filter: %q)",
		matches, len(v.state.currentResults), filterText))
}

func (v *View) HandleFilter(prompt *components2.Prompt, previousFocus tview.Primitive) {
	var opts components2.PromptOptions

	switch previousFocus {
	case v.components.filterInput:
		opts = components2.PromptOptions{
			Title:      " Filter Query ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnDone: func(text string) {
				v.addFilter(text)
				v.components.filterInput.SetText("")
				v.refreshResults()
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.components.filterInput.SetText("")
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
		}

	case v.components.fieldList:
		opts = components2.PromptOptions{
			Title:      " Filter Fields ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnChanged: func(text string) {
				v.filterFieldList(text)
			},
			OnDone: func(text string) {
				v.state.fieldListFilter = text
				v.filterFieldList(text)
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.state.fieldListFilter = ""
				v.filterFieldList("")
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
		}

	case v.components.localFilterInput:
		opts = components2.PromptOptions{
			Title:      " Filter Results ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnChanged: func(text string) {
				v.displayFilteredResults(text)
			},
			OnDone: func(text string) {
				v.displayFilteredResults(text)
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.displayFilteredResults("")
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
		}
	}

	prompt.Configure(opts)
	v.manager.ShowFilterPrompt(prompt.Show())
	v.manager.App().SetFocus(prompt.InputField)
}

func (v *View) Reinitialize(cfg aws.Config) {
	if err := v.service.Reinitialize(cfg, v.manager.CurrentProfile()); err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error reinitializing Elasticsearch service: %v", err))
		return
	}

	// Clear old state and UI
	v.state.currentResults = nil
	v.state.fieldOrder = nil
	v.state.activeFields = make(map[string]bool)
	v.state.filters = nil
	v.components.fieldList.Clear()
	v.components.resultsTable.Clear()
	v.components.selectedView.SetText("No fields selected")

	// Re-run initialization
	v.initFields()
	v.refreshResults()
	v.Show()
}

func (v *View) sortFieldList() []string {
	// Active fields should appear in the order defined by v.state.fieldOrder
	var active []string
	for _, f := range v.state.fieldOrder {
		if v.state.activeFields[f] {
			active = append(active, f)
		}
	}

	// Gather inactive fields
	var inactive []string
	for _, field := range v.state.originalFields {
		if !v.state.activeFields[field] {
			inactive = append(inactive, field)
		}
	}

	// Sort only the inactive fields
	sort.Strings(inactive)

	// Return active fields in the user-defined order, followed by sorted inactive fields
	return append(active, inactive...)
}

func (v *View) toggleField(field string) {
	wasActive := v.state.activeFields[field]
	v.state.activeFields[field] = !wasActive

	v.moveFieldInOrder(field, v.state.activeFields[field])

	if v.state.currentFilter != "" {
		v.filterFieldList(v.state.currentFilter)
	} else {
		v.rebuildFieldList()
	}

	v.updateSelectedFieldsDisplay()
	v.refreshResults()
}

func (v *View) filterFieldList(filter string) {
	v.state.currentFilter = filter
	v.components.fieldList.Clear()

	if filter == "" {
		v.state.fieldMatches = nil
		v.rebuildFieldList()
		v.manager.UpdateStatusBar("Showing all fields")
		return
	}

	filter = strings.ToLower(filter)
	var matches []string

	for _, field := range v.state.originalFields {
		if strings.Contains(strings.ToLower(field), filter) {
			matches = append(matches, field)
		}
	}

	v.state.fieldMatches = matches

	for _, field := range matches {
		isActive := v.state.activeFields[field]
		displayText := field
		if isActive {
			displayText = "[yellow]" + field + "[-]"
		}
		fieldName := field
		v.components.fieldList.AddItem(displayText, "", 0, func() {
			v.toggleField(fieldName)
		})
	}

	v.manager.UpdateStatusBar(fmt.Sprintf("Filtered: showing fields matching '%s' (%d matches)", filter, len(matches)))
}

func (v *View) rebuildFieldList() {
	v.components.fieldList.Clear()
	for _, field := range v.state.fieldOrder {
		displayText := field
		if v.state.activeFields[field] {
			displayText = "[yellow]" + field + "[-]"
		}

		fieldName := field // capture for closure
		v.components.fieldList.AddItem(displayText, "", 0, func() {
			v.toggleField(fieldName)
		})
	}
}

func (v *View) moveFieldPosition(field string, moveUp bool) {
	if !v.state.activeFields[field] {
		return // Only move selected fields
	}

	if v.state.fieldOrder == nil {
		v.state.fieldOrder = []string{}
		for _, f := range v.state.originalFields {
			if v.state.activeFields[f] {
				v.state.fieldOrder = append(v.state.fieldOrder, f)
			}
		}
	}

	// Find current position of the field in fieldOrder
	currentPos := -1
	for i, f := range v.state.fieldOrder {
		if f == field {
			currentPos = i
			break
		}
	}

	// If field not in fieldOrder, add it
	if currentPos == -1 {
		v.state.fieldOrder = append(v.state.fieldOrder, field)
		currentPos = len(v.state.fieldOrder) - 1
	}

	// get new position
	newPos := currentPos
	if moveUp && currentPos > 0 {
		newPos = currentPos - 1
	} else if !moveUp && currentPos < len(v.state.fieldOrder)-1 {
		newPos = currentPos + 1
	}

	// If we can move it, swap and rebuild
	if newPos != currentPos {
		// Save currently selected field for after rebuild
		selectedIndex := v.components.fieldList.GetCurrentItem()
		selectedText, _ := v.components.fieldList.GetItemText(selectedIndex)
		selectedText = stripColorTags(selectedText)

		// Swap in fieldOrder
		v.state.fieldOrder[currentPos], v.state.fieldOrder[newPos] = v.state.fieldOrder[newPos], v.state.fieldOrder[currentPos]

		v.rebuildFieldList()

		// Find the new index of the previously selected field
		newIndex := -1
		for i := 0; i < v.components.fieldList.GetItemCount(); i++ {
			txt, _ := v.components.fieldList.GetItemText(i)
			txt = stripColorTags(txt)
			if txt == selectedText {
				newIndex = i
				break
			}
		}

		// Restore selection
		if newIndex != -1 {
			v.components.fieldList.SetCurrentItem(newIndex)
		}

		v.refreshResults()
	}
}

func (v *View) handleFieldList(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'k': // Move up
			index := v.components.fieldList.GetCurrentItem()
			if index >= 0 {
				mainText, _ := v.components.fieldList.GetItemText(index)
				field := stripColorTags(mainText)
				v.moveFieldPosition(field, true)
			}
			return nil
		case 'j': // Move down
			index := v.components.fieldList.GetCurrentItem()
			if index >= 0 {
				mainText, _ := v.components.fieldList.GetItemText(index)
				field := stripColorTags(mainText)
				v.moveFieldPosition(field, false)
			}
			return nil
		}
	case tcell.KeyEnter:
		index := v.components.fieldList.GetCurrentItem()
		if index >= 0 {
			mainText, _ := v.components.fieldList.GetItemText(index)
			field := stripColorTags(mainText)
			v.toggleField(field)
		}
		return nil
	}
	return event
}

func stripColorTags(text string) string {
	text = strings.TrimPrefix(text, "[yellow]")
	text = strings.TrimSuffix(text, "[-]")
	return strings.TrimSpace(text)
}

func (v *View) getActiveHeaders() []string {
	var headers []string
	for _, field := range v.state.fieldOrder {
		if v.state.activeFields[field] {
			headers = append(headers, field)
		}
	}
	return headers
}

func (v *View) refreshResults() {
	v.components.resultsTable.Clear()

	headers := v.getActiveHeaders()
	if len(headers) == 0 {
		v.manager.UpdateStatusBar("No fields selected. Select a field to see data.")
		return
	}

	v.setupResultsTableHeaders(headers)
	v.fetchAndStoreResults()
	v.displayFilteredResults(v.components.localFilterInput.GetText())
}

func (v *View) setupResultsTableHeaders(headers []string) {
	v.components.resultsTable.SetFixed(1, 0)
	for col, header := range headers {
		v.components.resultsTable.SetCell(0, col,
			tview.NewTableCell(header).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignCenter).
				SetSelectable(false).
				SetAttributes(tcell.AttrBold))
	}
}

func (v *View) fetchAndStoreResults() {
	query := v.buildQuery()
	queryJSON, err := json.Marshal(query)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error creating query: %v", err))
		return
	}

	res, err := v.service.Client.Search(
		v.service.Client.Search.WithIndex(v.state.currentIndex),
		v.service.Client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Search error: %v", err))
		return
	}
	defer res.Body.Close()

	var result struct {
		Hits struct {
			Total int `json:"total"`
			Hits  []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error decoding response: %v", err))
		return
	}

	v.state.currentResults = make([]*LogEntry, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		entry, err := NewLogEntry(hit.Source)
		if err != nil {
			continue
		}
		v.state.currentResults = append(v.state.currentResults, entry)
	}

	v.manager.UpdateStatusBar(fmt.Sprintf("Found %d logs", result.Hits.Total))
}

func (v *View) moveFieldInOrder(field string, isActive bool) {
	pos := -1
	for i, f := range v.state.fieldOrder {
		if f == field {
			pos = i
			break
		}
	}

	if pos == -1 {
		// If not found, just return
		return
	}

	// Remove from current position
	v.state.fieldOrder = append(v.state.fieldOrder[:pos], v.state.fieldOrder[pos+1:]...)

	if isActive {
		// Move active fields to the top
		v.state.fieldOrder = append([]string{field}, v.state.fieldOrder...)
	} else {
		// Move inactive fields to the bottom or a specific position.
		// If you want them alphabetical, you can insert them
		// For simplicity, just append at the bottom:
		v.state.fieldOrder = append(v.state.fieldOrder, field)
	}
}

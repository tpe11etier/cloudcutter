package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/types"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/help"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type spinnerTextView struct {
	*tview.TextView
	frames  []string
	current int
	done    chan bool
}

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
	fieldList        *tview.List
	resultsTable     *tview.Table
	localFilterInput *tview.InputField
	timeframeInput   *tview.InputField
}

type viewState struct {
	activeFields      map[string]bool
	filters           []string
	currentIndex      string
	matchingIndices   []string
	currentResults    []*DocEntry
	originalFields    []string
	fieldListFilter   string
	currentFilter     string
	fieldMatches      []string
	fieldOrder        []string
	currentPage       int
	pageSize          int
	totalPages        int
	filteredResults   []*DocEntry
	displayedResults  []*DocEntry
	showRowNumbers    bool
	visibleRows       int
	lastDisplayHeight int
	isLoading         bool
	spinnerDone       chan bool
	columnCache       map[string][]string
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
			currentPage:     1,
			pageSize:        50,
			totalPages:      1,
			showRowNumbers:  true,
			visibleRows:     0,
			columnCache:     make(map[string][]string),
			spinnerDone:     make(chan bool),
		},
	}

	v.setupLayout()
	err := v.initFieldsSync()
	if err != nil {
		return v, err
	}

	manager.SetFocus(v.components.filterInput)
	return v, nil
}

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
								TitleColor:  tcell.ColorYellow,
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
								ID:         "indexView",
								Type:       types.ComponentInputField,
								Proportion: 2,
								Style: types.InputFieldStyle{
									BaseStyle: types.BaseStyle{
										Border:      true,
										BorderColor: tcell.ColorBeige,
										Title:       " Index ",
										TitleAlign:  tview.AlignCenter,
										TitleColor:  tcell.ColorYellow,
									},
									LabelColor:           tcell.ColorMediumTurquoise,
									FieldBackgroundColor: tcell.ColorBlack,
									FieldTextColor:       tcell.ColorBeige,
								},
								Properties: types.InputFieldProperties{
									Label:      ">_ ",
									FieldWidth: 0,
									Text:       v.state.currentIndex,
									OnFocus: func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorMediumTurquoise)
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
										TitleColor:  tcell.ColorYellow,
									},
									LabelColor:           tcell.ColorMediumTurquoise,
									FieldBackgroundColor: tcell.ColorBlack,
									FieldTextColor:       tcell.ColorBeige,
								},
								Properties: types.InputFieldProperties{
									Label:      ">_ ",
									FieldWidth: 0,
									Text:       "12h",
									OnFocus: func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorMediumTurquoise)
									},
									OnBlur: func(inputField *tview.InputField) {
										inputField.SetBorderColor(tcell.ColorBeige)
									},
									DoneFunc: func(s string) {
										v.refreshResults()
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
								TitleColor:  tcell.ColorYellow,
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
				Type:       types.ComponentFlex,
				Direction:  tview.FlexColumn,
				Proportion: 1,
				Children: []types.Component{
					// Field List
					{
						ID:        "fieldList",
						Type:      types.ComponentList,
						FixedSize: 50,
						Style: types.ListStyle{
							BaseStyle: types.BaseStyle{
								Border:      true,
								BorderColor: tcell.ColorBeige,
								Title:       "Available Fields (j ↓ / k ↑ to sort)",
								TitleColor:  tcell.ColorYellow,
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
					// Results Table
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
	v.components.indexView = v.manager.GetPrimitiveByID("indexView").(*tview.InputField)
	v.components.fieldList = v.manager.GetPrimitiveByID("fieldList").(*tview.List)
	v.components.resultsTable = v.manager.GetPrimitiveByID("resultsTable").(*tview.Table)
	v.components.localFilterInput = v.manager.GetPrimitiveByID("localFilterInput").(*tview.InputField)
	v.components.timeframeInput = v.manager.GetPrimitiveByID("timeframeInput").(*tview.InputField)

	v.components.localFilterInput.SetChangedFunc(func(text string) {
		v.displayFilteredResults(text)
	})

	v.initFields()
}

func (v *View) Name() string {
	return "elastic"
}

func (v *View) Content() tview.Primitive {
	return v.components.content
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
			switch event.Rune() {
			case 'r':
				if currentFocus == v.components.resultsTable {
					v.toggleRowNumbers()
					return nil
				}
			}
		}

		if event.Rune() == 'n' || event.Rune() == 'p' {
			if currentFocus == v.components.resultsTable {
				if event.Rune() == 'n' {
					v.nextPage()
				} else if event.Rune() == 'p' {
					v.previousPage()
				}
				return nil
			}
		}

		switch event.Key() {
		case tcell.KeyEsc:
			switch currentFocus {
			case v.components.resultsTable:
				v.manager.SetFocus(v.components.fieldList)
			case v.components.fieldList:
				v.manager.SetFocus(v.components.filterInput)
			default:
				v.manager.HideAllModals()
				v.manager.App().SetFocus(v.components.resultsTable)
			}
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
		case v.components.timeframeInput:
			return event
		case v.components.resultsTable:
			return v.handleResultsTable(event)
		case v.components.localFilterInput:
			return event
		}

		return event
	}
}
func (v *View) nextPage() {
	if v.state.totalPages < 1 {
		v.state.totalPages = 1
	}

	if v.state.currentPage < v.state.totalPages {
		v.state.currentPage++
		v.displayCurrentPage()
	} else {
		v.manager.UpdateStatusBar("Already on the last page.")
	}
}

func (v *View) previousPage() {
	if v.state.totalPages < 1 {
		v.state.totalPages = 1
	}

	if v.state.currentPage > 1 {
		v.state.currentPage--
		v.displayCurrentPage()
	} else {
		v.manager.UpdateStatusBar("Already on the first page.")
	}
}

func (v *View) handleTabKey(currentFocus tview.Primitive) *tcell.EventKey {
	switch currentFocus {
	case v.components.filterInput:
		v.manager.App().SetFocus(v.components.activeFilters)
	case v.components.activeFilters:
		v.manager.App().SetFocus(v.components.indexView)
	case v.components.indexView:
		v.manager.App().SetFocus(v.components.timeframeInput)
	case v.components.timeframeInput:
		v.manager.App().SetFocus(v.components.localFilterInput)
	case v.components.localFilterInput:
		v.manager.App().SetFocus(v.components.fieldList)
	case v.components.fieldList:
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
	v.refreshResults()

	v.updateHeader()
}

func (v *View) deleteFilterByIndex(index int) {
	if index >= 0 && index < len(v.state.filters) {
		v.state.filters = append(v.state.filters[:index], v.state.filters[index+1:]...)
		v.updateFiltersDisplay()
		v.refreshResults()

		v.updateHeader()
	}
}

func (v *View) deleteSelectedFilter() {
	row, _ := v.components.activeFilters.GetScrollOffset()
	if row < len(v.state.filters) {
		v.deleteFilterByIndex(row)
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

func prettyPrintJSON(data interface{}) string {
	pretty, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(pretty)
}

func (v *View) logQuery(query string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}

	logDir := filepath.Join(homeDir, ".cloudcutter", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	timestamp := time.Now().Format("2006-01-02")
	logFile := filepath.Join(logDir, fmt.Sprintf("es_queries_%s.log", timestamp))

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer f.Close()

	entry := fmt.Sprintf("\n=== Query at %s ===\n%s\n====================\n",
		time.Now().Format("2006-01-02 15:04:05"), query)

	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write to log file: %v", err)
	}

	return nil
}

func (v *View) buildQuery() map[string]interface{} {
	timeframe := v.components.timeframeInput.GetText()
	query := make(map[string]interface{})

	if timeframe != "" {
		// Build time-based query for documents with unixTime field
		boolQuery := map[string]interface{}{
			"must": []interface{}{
				map[string]interface{}{
					"range": map[string]interface{}{
						"unixTime": map[string]interface{}{
							"gte": fmt.Sprintf("now-%s", timeframe),
							"lte": "now",
						},
					},
				},
			},
		}

		// Add filters if any
		for _, filter := range v.state.filters {
			parts := strings.SplitN(filter, ":", 2)
			if len(parts) != 2 {
				parts = strings.SplitN(filter, "=", 2)
			}
			if len(parts) == 2 {
				field := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				// Try to parse value as number
				if num, err := strconv.ParseFloat(value, 64); err == nil {
					boolQuery["must"] = append(boolQuery["must"].([]interface{}),
						map[string]interface{}{
							"term": map[string]interface{}{
								field: num,
							},
						})
				} else {
					boolQuery["must"] = append(boolQuery["must"].([]interface{}),
						map[string]interface{}{
							"term": map[string]interface{}{
								field: value,
							},
						})
				}
			}
		}

		query["query"] = map[string]interface{}{
			"bool": boolQuery,
		}
	} else {
		// Simple match_all query for basic documents
		query["query"] = map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	}

	// Add size and sort
	query["size"] = v.state.pageSize

	// Only add sort if unixTime exists
	if timeframe != "" {
		query["sort"] = []map[string]interface{}{
			{
				"unixTime": map[string]interface{}{
					"order": "desc",
				},
			},
		}
	}

	return query
}

//	func (v *View) buildQuery() map[string]any {
//		timeframe := v.components.timeframeInput.GetText()
//		if timeframe == "" {
//			timeframe = "12h"
//		}
//
//		boolQuery := map[string]any{
//			"must": []any{
//				map[string]any{
//					"range": map[string]any{
//						"unixTime": map[string]any{
//							"gte": fmt.Sprintf("now-%s", timeframe),
//							"lte": "now",
//						},
//					},
//				},
//			},
//		}
//
//		for _, filter := range v.state.filters {
//			var parts []string
//			if strings.Contains(filter, "=") {
//				parts = strings.SplitN(filter, "=", 2)
//			} else if strings.Contains(filter, ":") {
//				parts = strings.SplitN(filter, ":", 2)
//			}
//
//			if len(parts) == 2 {
//				field := strings.TrimSpace(parts[0])
//				value := strings.TrimSpace(parts[1])
//
//				if num, err := strconv.ParseFloat(value, 64); err == nil {
//					boolQuery["must"] = append(boolQuery["must"].([]any), map[string]any{
//						"term": map[string]any{
//							field: num,
//						},
//					})
//				} else {
//					boolQuery["must"] = append(boolQuery["must"].([]any), map[string]any{
//						"term": map[string]any{
//							field: value,
//						},
//					})
//				}
//			}
//		}
//
//		// Build the final query
//		finalQuery := map[string]any{
//			"query": map[string]any{
//				"bool": boolQuery,
//			},
//			"sort": []map[string]any{
//				{
//					"unixTime": map[string]any{
//						"order": "desc",
//					},
//				},
//			},
//			"size": 250,
//		}
//
//		prettyQuery := prettyPrintJSON(finalQuery)
//		if err := v.logQuery(prettyQuery); err != nil {
//			v.manager.UpdateStatusBar(fmt.Sprintf("Failed to log query: %v", err))
//		} else {
//			// Copy to clipboard for easy pasting into Kibana
//			if err := clipboard.WriteAll(prettyQuery); err != nil {
//				v.manager.UpdateStatusBar(fmt.Sprintf("Failed to copy query: %v", err))
//			} else {
//				v.manager.UpdateStatusBar("Query copied to clipboard and logged to ~/.cloudcutter/logs/")
//			}
//		}
//
//		return finalQuery
//	}
func (v *View) handleIndexInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		pattern := v.components.indexView.GetText()
		if pattern != "" {
			v.state.currentIndex = pattern
			v.refreshResults()
		}
		return nil
	}
	return event
}

func (v *View) initFields() {
	query := map[string]any{
		"size": v.state.pageSize,
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
			Total int `json:"total"`
			Hits  []struct {
				ID      string          `json:"_id"`
				Index   string          `json:"_index"`
				Type    string          `json:"_type"`
				Score   *float64        `json:"_score"`
				Version *int64          `json:"_version"`
				Source  json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error decoding response: %v", err))
		return
	}

	// Initialize field set
	fieldSet := make(map[string]bool)

	v.state.currentResults = make([]*DocEntry, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		entry, err := NewDocEntry(
			hit.Source,
			hit.ID,
			hit.Index,
			hit.Type,
			hit.Score,
			hit.Version,
		)
		if err != nil {
			continue
		}

		// Get all fields including metadata and document fields
		fields := entry.GetAvailableFields()
		for _, field := range fields {
			fieldSet[field] = true
		}

		v.state.currentResults = append(v.state.currentResults, entry)
	}

	// Convert field set to slice
	var fields []string
	for field := range fieldSet {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	v.state.originalFields = fields
	v.state.fieldOrder = make([]string, len(v.state.originalFields))
	copy(v.state.fieldOrder, v.state.originalFields)

	for _, field := range v.state.fieldOrder {
		fieldName := field
		v.components.fieldList.AddItem(fieldName, "", 0, func() {
			v.toggleField(fieldName)
		})
	}

	v.manager.UpdateStatusBar(fmt.Sprintf("Found %d available fields", len(fields)))
}

func (v *View) HandleFilter(prompt *components.Prompt, previousFocus tview.Primitive) {
	var opts components.PromptOptions

	switch previousFocus {
	case v.components.filterInput:
		opts = components.PromptOptions{
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
		opts = components.PromptOptions{
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
		opts = components.PromptOptions{
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
	promptLayout := prompt.Show()
	v.manager.ShowFilterPrompt(promptLayout)
	v.manager.App().SetFocus(prompt.InputField)
}

func (v *View) Reinitialize(cfg aws.Config) error {
	if err := v.service.Reinitialize(cfg, v.manager.CurrentProfile()); err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error reinitializing Elasticsearch service: %v", err))
		return nil
	}

	// Clear old state and UI
	v.state.currentResults = nil
	v.state.fieldOrder = nil
	v.state.activeFields = make(map[string]bool)
	v.state.filters = nil
	v.components.fieldList.Clear()
	v.components.resultsTable.Clear()

	// Re-run initialization
	if err := v.initFieldsSync(); err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error initializing fields: %v", err))
		return nil
	}

	// Trigger results refresh and redraw
	v.refreshResults()
	v.displayCurrentPage()
	return nil
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
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		v.state.fieldListFilter = ""
		v.filterFieldList("")
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

func (v *View) fetchAndStoreResults() error {
	// Build and execute query
	query := v.buildQuery()
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("error creating query: %v", err)
	}

	// Execute search
	res, err := v.service.Client.Search(
		v.service.Client.Search.WithIndex(v.state.currentIndex),
		v.service.Client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		return fmt.Errorf("search error: %v", err)
	}
	defer res.Body.Close()

	// Process results
	var result struct {
		Hits struct {
			Total int `json:"total"`
			Hits  []struct {
				ID      string          `json:"_id"`
				Index   string          `json:"_index"`
				Type    string          `json:"_type"`
				Score   *float64        `json:"_score"`
				Version *int64          `json:"_version"`
				Source  json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return fmt.Errorf("error decoding response: %v", err)
	}

	// Clear and repopulate filtered results
	v.state.filteredResults = make([]*DocEntry, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		entry, err := NewDocEntry(
			hit.Source,
			hit.ID,
			hit.Index,
			hit.Type,
			hit.Score,
			hit.Version,
		)
		if err != nil {
			continue
		}
		v.state.filteredResults = append(v.state.filteredResults, entry)
	}

	// **Set displayedResults to filteredResults initially**
	v.state.displayedResults = append([]*DocEntry(nil), v.state.filteredResults...)

	// **Calculate totalPages ensuring it's at least 1**
	totalResults := len(v.state.filteredResults)
	v.state.totalPages = int(math.Ceil(float64(totalResults) / float64(v.state.pageSize)))
	if v.state.totalPages < 1 {
		v.state.totalPages = 1
	}

	// Update fields and UI
	v.updateAvailableFields(v.state.filteredResults)
	v.updateHeader()

	v.manager.UpdateStatusBar(fmt.Sprintf("Found %d logs across %d pages", totalResults, v.state.totalPages))
	return nil
}

func (v *View) handleResultsTable(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {

	case tcell.KeyEnter:
		row, _ := v.components.resultsTable.GetSelection()
		if row > 0 && row <= len(v.state.displayedResults) {
			entry := v.state.displayedResults[row-1]
			if entry.data == nil {
				// Fetch the document if it hasn't been fetched yet
				query := map[string]any{
					"query": map[string]any{
						"ids": map[string]any{
							"values": []string{entry.ID},
						},
					},
				}
				queryJSON, err := json.Marshal(query)
				if err != nil {
					v.manager.UpdateStatusBar(fmt.Sprintf("Error creating query: %v", err))
					return nil
				}

				res, err := v.service.Client.Search(
					v.service.Client.Search.WithIndex(entry.Index),
					v.service.Client.Search.WithBody(strings.NewReader(string(queryJSON))),
				)
				if err != nil {
					v.manager.UpdateStatusBar(fmt.Sprintf("Search error: %v", err))
					return nil
				}
				defer res.Body.Close()

				var result struct {
					Hits struct {
						Total int `json:"total"`
						Hits  []struct {
							ID      string          `json:"_id"`
							Index   string          `json:"_index"`
							Type    string          `json:"_type"`
							Score   *float64        `json:"_score"`
							Version *int64          `json:"_version"`
							Source  json.RawMessage `json:"_source"`
						} `json:"hits"`
					} `json:"hits"`
				}

				if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
					v.manager.UpdateStatusBar(fmt.Sprintf("Error decoding response: %v", err))
					return nil
				}

				if len(result.Hits.Hits) > 0 {
					var sourceData map[string]any
					if err := json.Unmarshal(result.Hits.Hits[0].Source, &sourceData); err != nil {
						v.manager.UpdateStatusBar(fmt.Sprintf("Error unmarshaling source data: %v", err))
						return nil
					}
					entry.data = sourceData
				}
			}

			v.showJSONModal(entry)
		}
		return nil
	}
	return event
}
func (v *View) showJSONModal(entry *DocEntry) {
	data := map[string]any{
		"_id":    entry.ID,
		"_index": entry.Index,
		"_type":  entry.Type,
	}
	if entry.Score != nil {
		data["_score"] = *entry.Score
	}
	if entry.Version != nil {
		data["_version"] = *entry.Version
	}

	data["_source"] = entry.data

	prettyJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error formatting JSON: %v", err))
		return
	}

	jsonStr := string(prettyJSON)

	textView := tview.NewTextView()
	textView.SetTitle("'y' to copy").SetTitleColor(tcell.ColorYellow)
	textView.SetText(string(prettyJSON)).
		SetDynamicColors(true).
		SetRegions(true).
		SetScrollable(true).
		SetWrap(false)
	textView.SetBorder(true).SetBorderColor(tcell.ColorMediumTurquoise)

	frame := tview.NewFrame(textView).
		SetBorders(0, 0, 0, 0, 0, 0)

	grid := tview.NewGrid().
		SetColumns(0, 150, 0).
		SetRows(0, 40, 0)

	grid.AddItem(frame, 1, 1, 1, 1, 0, 0, true)

	grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			v.manager.HideAllModals()
			v.manager.App().SetRoot(v.components.content, true)
			v.manager.App().SetFocus(v.components.resultsTable)
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'y' {
				// Copy to clipboard
				if err := clipboard.WriteAll(jsonStr); err != nil {
					v.manager.UpdateStatusBar("Failed to copy JSON to clipboard")
				} else {
					v.manager.UpdateStatusBar("JSON copied to clipboard")
				}
				return nil
			}
		}
		return event
	})

	pages := v.manager.Pages()
	if pages.HasPage(manager.ModalJSON) {
		pages.RemovePage(manager.ModalJSON)
	}
	pages.AddPage(manager.ModalJSON, grid, true, true)

	v.manager.App().SetFocus(textView)
}

func (v *View) updateAvailableFields(results []*DocEntry) {
	// Create a map to track field presence
	fieldSet := make(map[string]bool)

	// Process all results to collect fields
	for _, entry := range results {
		fields := entry.GetAvailableFields()
		for _, field := range fields {
			fieldSet[field] = true
		}
	}

	// Convert to sorted slice
	var newFields []string
	for field := range fieldSet {
		newFields = append(newFields, field)
	}
	sort.Strings(newFields)

	// Track new fields
	var addedFields []string
	for _, field := range newFields {
		if !contains(v.state.originalFields, field) {
			addedFields = append(addedFields, field)
		}
	}

	// Only update if we found new fields
	if len(addedFields) > 0 {
		// Update original fields
		v.state.originalFields = newFields

		// Update field order while preserving existing order
		newFieldOrder := make([]string, 0, len(newFields))

		// First add existing ordered fields
		for _, field := range v.state.fieldOrder {
			if fieldSet[field] {
				newFieldOrder = append(newFieldOrder, field)
			}
		}

		// Then add any new fields
		for _, field := range newFields {
			if !contains(newFieldOrder, field) {
				newFieldOrder = append(newFieldOrder, field)
			}
		}

		v.state.fieldOrder = newFieldOrder

		// Update the UI
		v.rebuildFieldList()

		// Notify user
		v.manager.UpdateStatusBar(fmt.Sprintf("Found %d new fields: %s",
			len(addedFields), strings.Join(addedFields, ", ")))
	}
}

func (v *View) updateHeader() {
	summary := []types.SummaryItem{
		{Key: "Index", Value: v.state.currentIndex},
		{Key: "Filters", Value: fmt.Sprintf("%d", len(v.state.filters))},
		{Key: "Results", Value: fmt.Sprintf("%d", len(v.state.displayedResults))},
		{Key: "Page", Value: fmt.Sprintf("[yellow]%d/%d[-]", v.state.currentPage, v.state.totalPages)},
		{Key: "Timeframe", Value: v.components.timeframeInput.GetText()},
	}
	v.manager.UpdateHeader(summary)
}

func (v *View) displayFilteredResults(filterText string) {
	if v.state.currentFilter == filterText {
		return // Avoid refiltering if filter hasn't changed
	}
	v.state.currentFilter = filterText

	// Reset display state
	if filterText == "" {
		v.state.displayedResults = append([]*DocEntry(nil), v.state.filteredResults...)
	} else {
		filterText = strings.ToLower(filterText)
		filtered := make([]*DocEntry, 0, len(v.state.filteredResults))

		for _, entry := range v.state.filteredResults {
			if v.entryMatchesFilter(entry, filterText) {
				filtered = append(filtered, entry)
			}
		}
		v.state.displayedResults = filtered
	}

	// Update pagination
	v.state.currentPage = 1
	totalResults := len(v.state.displayedResults)
	v.state.totalPages = int(math.Ceil(float64(totalResults) / float64(v.state.pageSize)))

	// Update display
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

func (v *View) displayCurrentPage() {
	v.components.resultsTable.Clear()

	headers := v.getActiveHeaders()
	if len(headers) == 0 {
		v.manager.UpdateStatusBar("No fields selected. Select a field to see data.")
		return
	}

	// Calculate visible rows before displaying
	v.calculateVisibleRows()

	// Add row numbers column if enabled
	displayHeaders := headers
	if v.state.showRowNumbers {
		displayHeaders = append([]string{"#"}, headers...)
	}
	v.setupResultsTableHeaders(displayHeaders)

	totalResults := len(v.state.displayedResults)
	if totalResults == 0 {
		v.manager.UpdateStatusBar("No results to display.")
		return
	}

	// Ensure currentPage is within bounds
	if v.state.currentPage < 1 {
		v.state.currentPage = 1
	} else if v.state.currentPage > v.state.totalPages {
		v.state.currentPage = v.state.totalPages
	}

	// Calculate page boundaries
	start := (v.state.currentPage - 1) * v.state.pageSize
	end := start + v.state.pageSize
	if end > totalResults {
		end = totalResults
	}

	pageResults := v.state.displayedResults[start:end]

	// Set up table cells
	for rowIdx, entry := range pageResults {
		currentRow := rowIdx + 1
		currentCol := 0

		if v.state.showRowNumbers {
			v.components.resultsTable.SetCell(currentRow, currentCol,
				tview.NewTableCell(fmt.Sprintf("%d", start+rowIdx+1)).
					SetTextColor(tcell.ColorGray).
					SetAlign(tview.AlignRight).
					SetSelectable(false))
			currentCol++
		}

		for _, header := range headers {
			value := entry.GetFormattedValue(header)
			v.components.resultsTable.SetCell(currentRow, currentCol,
				tview.NewTableCell(value).
					SetTextColor(tcell.ColorBeige).
					SetAlign(tview.AlignLeft).
					SetSelectable(true))
			currentCol++
		}
	}

	// Update the status bar with current page information
	v.updateStatusBar(len(pageResults))
}

func (v *View) updateStatusBar(currentPageSize int) {
	filterText := v.components.localFilterInput.GetText()
	statusMsg := fmt.Sprintf("Page %d/%d | Showing %d of %d logs",
		v.state.currentPage,
		v.state.totalPages,
		currentPageSize,
		len(v.state.displayedResults))

	if filterText != "" {
		statusMsg += fmt.Sprintf(" (filtered: %q)", filterText)
	}

	if v.state.showRowNumbers {
		statusMsg += " | [yellow]Row numbers: on (press 'r' to toggle)[-]"
	}

	v.manager.UpdateStatusBar(statusMsg)
}

func (v *View) calculateVisibleRows() {
	// Get the current screen size
	_, _, _, height := v.components.resultsTable.GetInnerRect()

	if height == v.state.lastDisplayHeight {
		return // No need to recalculate if height hasn't changed
	}

	v.state.lastDisplayHeight = height

	// Subtract 1 for header row and 1 for border
	v.state.visibleRows = height - 2

	// Ensure minimum of 1 row
	if v.state.visibleRows < 1 {
		v.state.visibleRows = 1
	}

	// Update page size to match visible rows
	v.state.pageSize = v.state.visibleRows
}

func (v *View) toggleRowNumbers() {
	v.state.showRowNumbers = !v.state.showRowNumbers
	v.displayCurrentPage() // No need for full refresh
}

func (v *View) Resize() {
	oldRows := v.state.visibleRows
	v.calculateVisibleRows()

	// Only refresh if the number of visible rows has changed
	if oldRows != v.state.visibleRows {
		v.refreshResults()
	}
}

func (v *View) moveFieldPosition(field string, moveUp bool) {
	// Don't move unselected fields
	if !v.state.activeFields[field] {
		return
	}

	// Find current position with early exit
	currentPos := -1
	for i, f := range v.state.fieldOrder {
		if f == field {
			currentPos = i
			break
		}
	}
	if currentPos == -1 {
		return
	}

	// Calculate new position
	newPos := currentPos
	if moveUp && currentPos > 0 {
		newPos = currentPos - 1
	} else if !moveUp && currentPos < len(v.state.fieldOrder)-1 {
		newPos = currentPos + 1
	}

	// If no movement needed, return early
	if newPos == currentPos {
		return
	}

	// Preserve current selection but only get it once
	selectedIndex := v.components.fieldList.GetCurrentItem()
	selectedText, _ := v.components.fieldList.GetItemText(selectedIndex)
	selectedText = stripColorTags(selectedText)

	// Perform the swap
	v.state.fieldOrder[currentPos], v.state.fieldOrder[newPos] =
		v.state.fieldOrder[newPos], v.state.fieldOrder[currentPos]

	// Clear only the affected columns in cache
	delete(v.state.columnCache, field)
	delete(v.state.columnCache, v.state.fieldOrder[currentPos])

	// More efficient rebuild focused only on the changed items
	v.components.fieldList.Clear()
	for _, f := range v.state.fieldOrder {
		displayText := f
		if v.state.activeFields[f] {
			displayText = "[yellow]" + f + "[-]"
		}
		fieldName := f
		v.components.fieldList.AddItem(displayText, "", 0, func() {
			v.toggleField(fieldName)
		})
	}

	// Restore selection efficiently
	for i := 0; i < v.components.fieldList.GetItemCount(); i++ {
		txt, _ := v.components.fieldList.GetItemText(i)
		if stripColorTags(txt) == selectedText {
			v.components.fieldList.SetCurrentItem(i)
			break
		}
	}

	// Refresh only the results table, not everything
	v.displayCurrentPage()
}

func (v *View) moveFieldInOrder(field string, isActive bool) {
	// Early exit if fieldOrder not initialized
	if v.state.fieldOrder == nil || len(v.state.fieldOrder) == 0 {
		return
	}

	// Find position with early exit
	currentPos := -1
	for i, f := range v.state.fieldOrder {
		if f == field {
			currentPos = i
			break
		}
	}
	if currentPos == -1 {
		return
	}

	// Allocate new slice only once
	newOrder := make([]string, 0, len(v.state.fieldOrder))

	if isActive {
		newOrder = append(newOrder, field)
		newOrder = append(newOrder, v.state.fieldOrder[:currentPos]...)
		if currentPos+1 < len(v.state.fieldOrder) {
			newOrder = append(newOrder, v.state.fieldOrder[currentPos+1:]...)
		}
	} else {
		newOrder = append(newOrder, v.state.fieldOrder[:currentPos]...)
		if currentPos+1 < len(v.state.fieldOrder) {
			newOrder = append(newOrder, v.state.fieldOrder[currentPos+1:]...)
		}
		newOrder = append(newOrder, field)
	}

	// Clear only necessary cache entries
	delete(v.state.columnCache, field)

	v.state.fieldOrder = newOrder
}

func (v *View) toggleField(field string) {
	v.state.activeFields[field] = !v.state.activeFields[field]

	v.moveFieldInOrder(field, v.state.activeFields[field])

	if v.state.currentFilter != "" {
		v.filterFieldList(v.state.currentFilter)
	} else {
		v.rebuildFieldList()
	}

	v.refreshResults()
}

func (v *View) initFieldsSync() error {
	// Build the initial query
	query := map[string]any{
		"size": v.state.pageSize,
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
		return fmt.Errorf("error creating query: %v", err)
	}

	res, err := v.service.Client.Search(
		v.service.Client.Search.WithIndex(v.state.currentIndex),
		v.service.Client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		return fmt.Errorf("search error: %v", err)
	}
	defer res.Body.Close()

	var result struct {
		Hits struct {
			Total int `json:"total"`
			Hits  []struct {
				ID      string          `json:"_id"`
				Index   string          `json:"_index"`
				Type    string          `json:"_type"`
				Score   *float64        `json:"_score"`
				Version *int64          `json:"_version"`
				Source  json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return fmt.Errorf("error decoding response: %v", err)
	}

	// Initialize field set
	fieldSet := make(map[string]bool)
	v.state.currentResults = make([]*DocEntry, 0, len(result.Hits.Hits))

	for _, hit := range result.Hits.Hits {
		entry, err := NewDocEntry(
			hit.Source,
			hit.ID,
			hit.Index,
			hit.Type,
			hit.Score,
			hit.Version,
		)
		if err != nil {
			continue
		}

		fields := entry.GetAvailableFields()
		for _, field := range fields {
			fieldSet[field] = true
		}

		v.state.currentResults = append(v.state.currentResults, entry)
	}

	var fields []string
	for field := range fieldSet {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	v.state.originalFields = fields
	v.state.fieldOrder = make([]string, len(v.state.originalFields))
	copy(v.state.fieldOrder, v.state.originalFields)

	v.state.filteredResults = append([]*DocEntry(nil), v.state.currentResults...)
	v.state.displayedResults = append([]*DocEntry(nil), v.state.filteredResults...)

	totalResults := len(v.state.filteredResults)
	v.state.totalPages = int(math.Ceil(float64(totalResults) / float64(v.state.pageSize)))
	if v.state.totalPages < 1 {
		v.state.totalPages = 1
	}

	v.components.fieldList.Clear()
	for _, field := range v.state.fieldOrder {
		fieldName := field
		v.components.fieldList.AddItem(fieldName, "", 0, func() {
			v.toggleField(fieldName)
		})
	}

	v.displayCurrentPage()

	v.manager.UpdateStatusBar(fmt.Sprintf("Found %d available fields", len(fields)))
	return nil
}

func (v *View) newSpinnerTextView(text string) *spinnerTextView {
	spinner := &spinnerTextView{
		TextView: tview.NewTextView().
			SetTextAlign(tview.AlignCenter).
			SetTextColor(tcell.ColorBeige).
			SetDynamicColors(true),
		frames:  []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		current: 0,
		done:    make(chan bool),
	}

	go v.animate(spinner, text)
	return spinner
}

func (v *View) animate(spinner *spinnerTextView, text string) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-spinner.done:
			return
		case <-ticker.C:
			v.manager.App().QueueUpdateDraw(func() {
				spinner.SetText(fmt.Sprintf("\n[yellow]%s[white] %s", text, spinner.frames[spinner.current]))
			})
			spinner.current = (spinner.current + 1) % len(spinner.frames)
		}
	}
}

func (v *View) showLoading(message string) {
	if !v.state.isLoading {
		v.state.isLoading = true

		spinner := v.newSpinnerTextView(message)

		modal := tview.NewGrid().
			SetColumns(0, 40, 0).
			SetRows(0, 5, 0).
			SetBorders(false).
			AddItem(spinner, 1, 1, 1, 1, 0, 0, false)

		pages := v.manager.Pages()
		pages.AddPage("loading", modal, true, true)

		v.state.spinnerDone = spinner.done
	}
}

func (v *View) hideLoading() {
	if v.state.isLoading {
		v.state.isLoading = false
		select {
		case v.state.spinnerDone <- true:
		default:
		}

		pages := v.manager.Pages()
		pages.RemovePage("loading")
	}
}

func (v *View) Show() {
	v.refreshResults()
	v.manager.App().SetFocus(v.components.filterInput)
}

func (v *View) refreshResults() {
	if v.state.isLoading {
		return
	}

	currentFocus := v.manager.App().GetFocus()

	v.showLoading("Refreshing results")

	go func() {
		err := v.fetchAndStoreResults()
		if err != nil {
			// Update the status bar with the error
			v.manager.UpdateStatusBar(fmt.Sprintf("Error fetching results: %v", err))
		}

		v.manager.App().QueueUpdateDraw(func() {
			v.displayCurrentPage()
			v.updateHeader()

			v.hideLoading()

			if currentFocus != nil {
				v.manager.App().SetFocus(currentFocus)
			}
		})
	}()
}

func getIndices(v *View) []help.Command {
	indices, _ := v.service.ListIndices(context.Background(), "*")
	indexCommands := make([]help.Command, 0, len(indices))
	for _, idx := range indices {
		indexCommands = append(indexCommands, help.Command{
			Key: idx,
		})
	}
	indexCommands = append(indexCommands, help.Command{})
	return indexCommands
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

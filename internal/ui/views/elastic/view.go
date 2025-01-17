package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/spinner"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/types"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/help"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/style"

	"sync"

	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FieldMetadata struct {
	Type         string
	Searchable   bool
	Aggregatable bool
	Active       bool
}

type searchResult struct {
	entries   []*DocEntry
	totalHits int
}

type View struct {
	manager                     *manager.Manager
	components                  viewComponents
	service                     *elastic.Service
	state                       State
	layout                      tview.Primitive
	refreshWithCurrentTimeframe func()
}

type viewComponents struct {
	content          *tview.Flex
	filterInput      *tview.InputField
	activeFilters    *tview.TextView
	indexView        *tview.InputField
	fieldList        *tview.List
	selectedList     *tview.List
	resultsTable     *tview.Table
	localFilterInput *tview.InputField
	timeframeInput   *tview.InputField
	numResultsInput  *tview.InputField
	filterPrompt     *components.Prompt
	resultsFlex      *tview.Flex
	listsContainer   *tview.Flex
}

type State struct {
	pagination PaginationState
	ui         UIState
	data       DataState
	search     SearchState
	misc       MiscState
	mu         sync.RWMutex
}

type PaginationState struct {
	currentPage int
	totalPages  int
	pageSize    int
}

type UIState struct {
	showRowNumbers   bool
	isLoading        bool
	fieldListFilter  string
	fieldListVisible bool
}

type DataState struct {
	activeFields     map[string]bool
	filters          []string
	currentResults   []*DocEntry
	fieldOrder       []string
	originalFields   []string
	fieldMatches     []string
	filteredResults  []*DocEntry
	displayedResults []*DocEntry
	columnCache      map[string][]string
	currentFilter    string
	fieldCache       *FieldCache
}

type SearchState struct {
	currentIndex    string
	matchingIndices []string
	numResults      int
	timeframe       string
	indexStats      *elastic.IndexStats
	cancelCurrentOp context.CancelFunc
}

type MiscState struct {
	visibleRows       int
	lastDisplayHeight int
	spinner           *spinner.Spinner
	rateLimit         *RateLimiter
}

func NewView(manager *manager.Manager, esClient *elastic.Service, defaultIndex string) (*View, error) {
	v := &View{
		manager: manager,
		service: esClient,
		components: viewComponents{
			filterPrompt: components.NewPrompt(),
		},
		state: State{
			pagination: PaginationState{
				currentPage: 1,
				pageSize:    50,
				totalPages:  1,
			},
			ui: UIState{
				showRowNumbers:  true,
				isLoading:       false,
				fieldListFilter: "",
			},
			data: DataState{
				activeFields:     make(map[string]bool),
				filters:          []string{},
				currentResults:   []*DocEntry{},
				fieldOrder:       []string{},
				originalFields:   []string{},
				fieldMatches:     []string{},
				filteredResults:  []*DocEntry{},
				displayedResults: []*DocEntry{},
				columnCache:      make(map[string][]string),
				fieldCache:       NewFieldCache(),
			},
			search: SearchState{
				currentIndex:    defaultIndex,
				matchingIndices: []string{},
				numResults:      1000,
				timeframe:       "today",
			},
			misc: MiscState{
				visibleRows:       0,
				lastDisplayHeight: 0,
				spinner:           nil,
				rateLimit:         NewRateLimiter(),
			},
		},
	}

	v.manager.Logger().Info("Initializing Elastic View", "defaultIndex", defaultIndex)

	// Setup layout and initialize fields
	v.setupLayout()
	v.initTimeframeState()
	err := v.initFieldsSync()
	if err != nil {
		v.manager.Logger().Error("Failed to initialize fields", "error", err)
		return v, err
	}

	v.refreshWithCurrentTimeframe = v.doRefreshWithCurrentTimeframe
	manager.SetFocus(v.components.filterInput)
	v.manager.Logger().Info("Elastic View successfully initialized")
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
								ID:         "indexView",
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
									DoneFunc: func(s string) {
										if s != "" {
											v.state.search.currentIndex = s
											v.resetFieldState()
											v.doRefreshWithCurrentTimeframe()
										}
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
										s = strings.TrimSpace(s)
										if s != "" {
											if _, err := ParseTimeframe(s); err != nil {
												v.manager.UpdateStatusBar(fmt.Sprintf("Invalid timeframe: %v", err))
												return
											}
										}
										v.doRefreshWithCurrentTimeframe()
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
										Title:       "Available Fields (Enter to select)",
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
										Title:       "Selected Fields (j↓/k↑ to order)",
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
	v.components.indexView = v.manager.GetPrimitiveByID("indexView").(*tview.InputField)
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
		if event.Key() == tcell.KeyRune && event.Rune() == '/' {
			switch currentFocus {
			case v.components.fieldList:
				v.showFilterPrompt(v.components.fieldList)
				return nil
			case v.components.localFilterInput:
				v.showFilterPrompt(v.components.localFilterInput)
				return nil
			case v.components.resultsTable:
				v.showFilterPrompt(v.components.resultsTable)
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
				v.manager.App().SetFocus(v.components.filterInput)
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
		case v.components.selectedList:
			return v.handleSelectedList(event)
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
				v.state.ui.fieldListFilter = text
				v.filterFieldList(text)
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.state.ui.fieldListFilter = ""
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
	promptLayout := prompt.Layout()
	v.manager.Pages().AddPage(types.ModalFilter, promptLayout, true, true)
	v.manager.App().SetFocus(prompt.InputField)
}

func (v *View) Reinitialize(cfg aws.Config) error {
	if err := v.service.Reinitialize(cfg, v.manager.CurrentProfile()); err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error reinitializing Elasticsearch service: %v", err))
		return nil
	}

	// Clear all state and UI components
	v.state = State{
		pagination: PaginationState{
			currentPage: 1,
			pageSize:    50,
			totalPages:  1,
		},
		ui: UIState{
			showRowNumbers:  true,
			isLoading:       false,
			fieldListFilter: "",
		},
		data: DataState{
			activeFields:     make(map[string]bool),
			filters:          []string{},
			currentResults:   []*DocEntry{},
			fieldOrder:       []string{},
			originalFields:   []string{},
			fieldMatches:     []string{},
			filteredResults:  []*DocEntry{},
			displayedResults: []*DocEntry{},
			columnCache:      make(map[string][]string),
			fieldCache:       NewFieldCache(),
		},
		search: SearchState{
			currentIndex:    v.state.search.currentIndex,
			matchingIndices: []string{},
			numResults:      1000,
			timeframe:       "12h",
		},
		misc: MiscState{
			visibleRows:       0,
			lastDisplayHeight: 0,
			spinner:           nil,
			rateLimit:         NewRateLimiter(),
		},
	}

	// Clear UI components
	v.components.fieldList.Clear()
	v.components.selectedList.Clear()
	v.components.resultsTable.Clear()
	v.components.activeFilters.SetText("No active filters")
	v.components.localFilterInput.SetText("")
	v.components.filterInput.SetText("")

	// Re-run initialization
	if err := v.initFieldsSync(); err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error initializing fields: %v", err))
		return nil
	}

	v.refreshResults()
	v.displayCurrentPage()
	return nil
}

func (v *View) filterFieldList(filter string) {
	v.state.data.currentFilter = filter
	v.components.fieldList.Clear()

	if filter == "" {
		v.state.data.fieldMatches = nil
		v.rebuildFieldList()
		v.manager.UpdateStatusBar("Showing all fields")
		return
	}

	filter = strings.ToLower(filter)
	var matches []string

	// Only show fields that match the filter AND are not currently active
	for _, field := range v.state.data.originalFields {
		if strings.Contains(strings.ToLower(field), filter) && !v.state.data.activeFields[field] {
			matches = append(matches, field)
		}
	}

	v.state.data.fieldMatches = matches

	for _, field := range matches {
		displayText := field
		fieldName := field
		v.components.fieldList.AddItem(displayText, "", 0, func() {
			v.toggleField(fieldName)
		})
	}

	v.manager.UpdateStatusBar(fmt.Sprintf("Filtered: showing available fields matching '%s' (%d matches)", filter, len(matches)))
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

	// Colorize the JSON
	coloredJSON := colorizeJSON(string(prettyJSON))

	textView := tview.NewTextView()
	textView.SetTitle("'y' to copy | 'Esc' to close").
		SetTitleColor(style.GruvboxMaterial.Yellow)
	textView.SetText(coloredJSON).
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

	jsonStr := string(prettyJSON)

	grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			v.manager.HideAllModals()
			v.manager.App().SetRoot(v.components.content, true)
			v.manager.App().SetFocus(v.components.resultsTable)
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'y' {
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

func (v *View) displayFilteredResults(filterText string) {
	v.state.mu.Lock()
	if v.state.data.currentFilter == filterText {
		v.state.mu.Unlock()
		return
	}
	v.state.data.currentFilter = filterText

	// Work with copies while holding the lock
	currentResults := make([]*DocEntry, len(v.state.data.filteredResults))
	copy(currentResults, v.state.data.filteredResults)
	v.state.mu.Unlock()

	// Process without holding the lock
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

	// Update state with results
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

func (v *View) calculateVisibleRows() {
	// Get the current screen size
	_, _, _, height := v.components.resultsTable.GetInnerRect()

	if height == v.state.misc.lastDisplayHeight {
		return // No need to recalculate if height hasn't changed
	}

	v.state.misc.lastDisplayHeight = height

	// Subtract 1 for header row and 1 for border
	v.state.misc.visibleRows = height - 2

	// Ensure minimum of 1 row
	if v.state.misc.visibleRows < 1 {
		v.state.misc.visibleRows = 1
	}

	// Update page size to match visible rows
	v.state.pagination.pageSize = v.state.misc.visibleRows
}

func (v *View) toggleRowNumbers() {
	v.state.ui.showRowNumbers = !v.state.ui.showRowNumbers
	v.displayCurrentPage() // No need for full refresh
}

func (v *View) moveFieldPosition(field string, moveUp bool) {
	// Get current list of fields in selected list
	var selectedFields []string
	for i := 0; i < v.components.selectedList.GetItemCount(); i++ {
		text, _ := v.components.selectedList.GetItemText(i)
		selectedFields = append(selectedFields, text)
	}

	// Find current position
	currentPos := -1
	for i, f := range selectedFields {
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
	} else if !moveUp && currentPos < len(selectedFields)-1 {
		newPos = currentPos + 1
	}

	// If no movement needed, return early
	if newPos == currentPos {
		return
	}

	// Do the swap
	selectedFields[currentPos], selectedFields[newPos] = selectedFields[newPos], selectedFields[currentPos]

	// Rebuild just the selected list
	v.components.selectedList.Clear()
	for _, f := range selectedFields {
		field := f // Capture for closure
		v.components.selectedList.AddItem(field, "", 0, func() {
			v.toggleField(field)
		})
	}

	// Set focus back to the moved item
	v.components.selectedList.SetCurrentItem(newPos)

	// Refresh the results table since order changed
	v.displayCurrentPage()
}
func (v *View) moveFieldInOrder(field string, isActive bool) {
	// Early exit if fieldOrder not initialized
	if v.state.data.fieldOrder == nil || len(v.state.data.fieldOrder) == 0 {
		return
	}

	// Find position with early exit
	currentPos := -1
	for i, f := range v.state.data.fieldOrder {
		if f == field {
			currentPos = i
			break
		}
	}
	if currentPos == -1 {
		return
	}

	newOrder := make([]string, 0, len(v.state.data.fieldOrder))

	if isActive {
		newOrder = append(newOrder, field)
		newOrder = append(newOrder, v.state.data.fieldOrder[:currentPos]...)
		if currentPos+1 < len(v.state.data.fieldOrder) {
			newOrder = append(newOrder, v.state.data.fieldOrder[currentPos+1:]...)
		}
	} else {
		newOrder = append(newOrder, v.state.data.fieldOrder[:currentPos]...)
		if currentPos+1 < len(v.state.data.fieldOrder) {
			newOrder = append(newOrder, v.state.data.fieldOrder[currentPos+1:]...)
		}
		newOrder = append(newOrder, field)
	}

	// Clear only necessary cache entries
	delete(v.state.data.columnCache, field)

	v.state.data.fieldOrder = newOrder
}

func (v *View) showLoading(message string) {
	v.state.mu.Lock()
	defer v.state.mu.Unlock()

	if v.state.misc.spinner == nil {
		v.state.misc.spinner = spinner.NewSpinner(message)
		v.state.misc.spinner.SetOnComplete(func() {
			v.state.mu.Lock()
			defer v.state.mu.Unlock()
			pages := v.manager.Pages()
			pages.RemovePage("loading")
		})
	} else {
		v.state.misc.spinner.SetMessage(message)
	}

	if !v.state.misc.spinner.IsLoading() {
		modal := spinner.CreateSpinnerModal(v.state.misc.spinner)
		pages := v.manager.Pages()
		pages.AddPage("loading", modal, true, true)
		v.state.misc.spinner.Start(v.manager.App())
	}
}

func (v *View) hideLoading() {
	if v.state.misc.spinner != nil {
		v.state.misc.spinner.Stop()
	}
}

func (v *View) Show() {
	v.manager.SetFocus(v.components.filterInput)
	v.refreshResults()
}

func getIndices(v *View) []help.Command {
	v.state.mu.RLock()
	indices := v.state.search.matchingIndices
	v.state.mu.RUnlock()

	indexCommands := make([]help.Command, 0, len(indices))
	for _, idx := range indices {
		indexCommands = append(indexCommands, help.Command{
			Key: idx,
		})
	}
	indexCommands = append(indexCommands, help.Command{})
	return indexCommands
}

func (v *View) showFilterPrompt(source tview.Primitive) {
	previousFocus := source

	switch source {
	case v.components.fieldList:
		v.components.filterPrompt.Configure(components.PromptOptions{
			Title:      " Filter Fields ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnChanged: func(text string) {
				v.filterFieldList(text)
			},
			OnDone: func(text string) {
				v.state.ui.fieldListFilter = text
				v.filterFieldList(text)
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.state.ui.fieldListFilter = ""
				v.filterFieldList("")
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
		})

	case v.components.localFilterInput:
		v.components.filterPrompt.Configure(components.PromptOptions{
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
		})
	case v.components.resultsTable:
		v.components.filterPrompt.Configure(components.PromptOptions{
			Title:      " Filter Results ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnChanged: func(text string) {
				v.components.localFilterInput.SetText(v.components.filterPrompt.GetText())
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
		})
	}

	v.components.filterPrompt.SetText("")
	promptLayout := v.components.filterPrompt.Layout()
	v.manager.Pages().AddPage(types.ModalFilter, promptLayout, true, true)
	v.manager.App().SetFocus(v.components.filterPrompt.InputField)
}

func (v *View) processSearchResults(hits []elastic.ESSearchHit) ([]*DocEntry, error) {
	results := make([]*DocEntry, 0, len(hits))

	for _, hit := range hits {
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
		results = append(results, entry)
	}

	return results, nil
}

func (v *View) toggleFieldList() {
	v.state.ui.fieldListVisible = !v.state.ui.fieldListVisible
	v.updateResultsLayout()

	if !v.state.ui.fieldListVisible {
		v.manager.App().SetFocus(v.components.resultsTable)
	}
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

func (v *View) getActiveHeaders() []string {
	// Instead of using fieldOrder, get order from selected list
	var headers []string
	for i := 0; i < v.components.selectedList.GetItemCount(); i++ {
		text, _ := v.components.selectedList.GetItemText(i)
		headers = append(headers, text)
	}
	return headers
}

func (v *View) rebuildFieldList() {
	v.components.fieldList.Clear()
	for _, field := range v.state.data.fieldOrder {
		if !v.state.data.activeFields[field] { // Only show non-selected fields
			v.components.fieldList.AddItem(field, "", 0, func() {
				v.toggleField(field)
			})
		}
	}
}

func (v *View) toggleField(field string) {
	isActive := v.state.data.activeFields[field]
	v.state.data.activeFields[field] = !isActive

	if !isActive {
		v.components.selectedList.AddItem(field, "", 0, func() {
			v.toggleField(field)
		})

		for i := 0; i < v.components.fieldList.GetItemCount(); i++ {
			if text, _ := v.components.fieldList.GetItemText(i); text == field {
				v.components.fieldList.RemoveItem(i)
				break
			}
		}
	} else {
		for i := 0; i < v.components.selectedList.GetItemCount(); i++ {
			if text, _ := v.components.selectedList.GetItemText(i); text == field {
				v.components.selectedList.RemoveItem(i)
				break
			}
		}
		v.components.fieldList.AddItem(field, "", 0, func() {
			v.toggleField(field)
		})
	}

	// Refresh results to show new column order
	v.refreshResults()
}

func (v *View) executeSearch(query map[string]any) (*elastic.ESSearchResult, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Wait according to rate limit
		v.state.misc.rateLimit.Wait()

		queryJSON, err := json.Marshal(query)
		if err != nil {
			v.manager.Logger().Error("Error marshaling query", "error", err)
			return nil, fmt.Errorf("error creating query: %v", err)
		}

		v.manager.Logger().Debug("Executing search query", "index", v.state.search.currentIndex, "query", string(queryJSON))

		res, err := v.service.Client.Search(
			v.service.Client.Search.WithIndex(v.state.search.currentIndex),
			v.service.Client.Search.WithBody(bytes.NewReader(queryJSON)),
		)
		if err != nil {
			lastErr = err
			// Check if it's a rate limit error
			if strings.Contains(err.Error(), "429") {
				v.state.misc.rateLimit.HandleTooManyRequests()
				v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited, retrying in %v...", v.state.misc.rateLimit.GetRetryAfter()))
				continue
			}
			v.manager.Logger().Error("Search query failed", "error", err, "index", v.state.search.currentIndex)
			return nil, fmt.Errorf("search error: %v", err)
		}
		defer res.Body.Close()

		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			v.manager.Logger().Error("Failed to read search response", "error", err)
			return nil, fmt.Errorf("error reading response body: %v", err)
		}

		// Check if response indicates rate limiting
		if res.StatusCode == 429 {
			v.state.misc.rateLimit.HandleTooManyRequests()
			v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited, retrying in %v...", v.state.misc.rateLimit.GetRetryAfter()))
			continue
		}

		// Reset rate limit backoff on successful request
		v.state.misc.rateLimit.Reset()

		v.manager.Logger().Debug("Raw search response", "response", string(bodyBytes))

		var result elastic.ESSearchResult
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			v.manager.Logger().Error("Failed to unmarshal search response", "error", err, "response", string(bodyBytes))
			return nil, fmt.Errorf("error decoding response: %v", err)
		}

		v.manager.Logger().Info("Search executed successfully", "hits", result.Hits.Total.Value, "took", result.Took)
		return &result, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
}

func (v *View) buildQuery() map[string]any {
	v.state.mu.RLock()
	filters := make([]string, len(v.state.data.filters))
	copy(filters, v.state.data.filters)
	timeframe := v.state.search.timeframe
	numResults := v.state.search.numResults
	v.state.mu.RUnlock()

	query, err := BuildQuery(filters, numResults, timeframe)
	if err != nil {
		v.manager.Logger().Error("Error building query", "error", err)
		v.manager.UpdateStatusBar(fmt.Sprintf("Error building query: %v", err))
		return nil
	}
	return query
}

func (v *View) initFieldsSync() error {
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Concurrent index listing
	wg.Add(1)
	go func() {
		defer wg.Done()
		indices, err := v.service.ListIndices(context.Background(), "*")
		if err != nil {
			v.manager.Logger().Error("Failed to list indices", "error", err)
			errChan <- err
			return
		}
		v.state.mu.Lock()
		v.state.search.matchingIndices = indices
		v.state.mu.Unlock()
	}()

	// Load fields using field_caps API
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := v.loadFields(); err != nil {
			errChan <- err
			return
		}

		// Initialize UI components
		v.manager.App().QueueUpdateDraw(func() {
			v.rebuildFieldList()
		})
	}()

	// Wait for all goroutines and check for errors
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Return first error if any occurred
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *View) Close() error {
	if v.manager.Logger() != nil {
		return v.manager.Logger().Close()
	}
	return nil
}

func (v *View) updateIndexStats() {
	stats, err := v.service.GetIndexStats(context.Background(), v.state.search.currentIndex)
	if err != nil {
		stats, err = v.service.GetIndexStats(context.Background(), v.state.search.currentIndex)
		if err != nil {
			v.manager.Logger().Error("Failed to get index stats after refresh", "error", err)
			return
		}
	}
	v.state.search.indexStats = stats
}

func colorizeJSON(jsonStr string) string {
	lines := strings.Split(jsonStr, "\n")
	var coloredLines []string

	for _, line := range lines {
		// Find the first double quote and colon to separate key and value
		parts := strings.SplitN(line, ":", 2)

		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]

			// Color the key
			key = strings.ReplaceAll(key, `"`, fmt.Sprintf("[%s]\"[%s]", style.GruvboxMaterial.Blue, tcell.ColorReset))

			// Color the value based on type
			value = strings.TrimSpace(value)
			switch {
			case value == "null":
				value = fmt.Sprintf("[%s]null[%s]", style.GruvboxMaterial.Red, tcell.ColorReset)
			case value == "true" || value == "false":
				value = fmt.Sprintf("[%s]%s[%s]", style.GruvboxMaterial.Purple, value, tcell.ColorReset)
			case strings.HasPrefix(value, `"`):
				value = fmt.Sprintf("[%s]%s[%s]", style.GruvboxMaterial.Green, value, tcell.ColorReset)
			case strings.HasPrefix(value, "{") || strings.HasPrefix(value, "["):
				value = fmt.Sprintf("[%s]%s[%s]", style.GruvboxMaterial.Yellow, value, tcell.ColorReset)
			default: // Numbers
				value = fmt.Sprintf("[%s]%s[%s]", style.GruvboxMaterial.Orange, value, tcell.ColorReset)
			}

			coloredLines = append(coloredLines, fmt.Sprintf("%s:%s", key, value))
		} else {
			trimmed := strings.TrimSpace(line)
			if trimmed == "{" || trimmed == "}" || trimmed == "[" || trimmed == "]" || trimmed == "}," || trimmed == "]," {
				line = fmt.Sprintf("[%s]%s[%s]", style.GruvboxMaterial.Yellow, line, tcell.ColorReset)
			}
			coloredLines = append(coloredLines, line)
		}
	}

	return strings.Join(coloredLines, "\n")
}

func (v *View) initTimeframeState() {
	v.state.search.timeframe = ""

	v.components.timeframeInput.SetText("today")

	v.doRefreshWithCurrentTimeframe()
}

func (v *View) doRefreshWithCurrentTimeframe() {
	timeframe := strings.TrimSpace(v.components.timeframeInput.GetText())
	// If timeframe is "today" (the default) and hasn't been explicitly set, treat as empty
	if timeframe == "today" && v.state.search.timeframe == "" {
		v.state.search.timeframe = ""
	} else {
		v.state.search.timeframe = timeframe
	}
	v.refreshResults()
}

func (v *View) fetchResults() (*searchResult, error) {
	v.state.mu.RLock()
	numResults := v.state.search.numResults
	query := v.buildQuery()
	currentIndex := v.state.search.currentIndex
	v.state.mu.RUnlock()

	if numResults > 10000 {
		return v.fetchLargeResults(query, currentIndex)
	}
	return v.fetchRegularResults(query, numResults, currentIndex)
}

func (v *View) refreshResults() {
	v.state.mu.Lock()
	if v.state.ui.isLoading {
		v.state.mu.Unlock()
		return
	}
	v.state.ui.isLoading = true
	v.state.mu.Unlock()

	intended := v.components.filterInput
	v.showLoading("Refreshing results")

	go func() {
		defer func() {
			v.state.mu.Lock()
			v.state.ui.isLoading = false
			v.state.mu.Unlock()
			v.hideLoading()
			v.manager.App().QueueUpdateDraw(func() {
				v.manager.SetFocus(intended)
			})
		}()

		searchResult, err := v.fetchResults()
		if err != nil {
			v.manager.Logger().Error("Error fetching results", "error", err)
			v.manager.App().QueueUpdateDraw(func() {
				v.manager.UpdateStatusBar(fmt.Sprintf("Error: %v", err))
			})
			return
		}

		v.manager.Logger().Info("Results refreshed successfully",
			"totalResults", len(searchResult.entries))

		// Update fields based on results
		v.updateFieldsFromResults(searchResult.entries)

		// Update results state
		v.state.mu.Lock()
		v.state.data.filteredResults = searchResult.entries
		v.state.data.displayedResults = append([]*DocEntry(nil), searchResult.entries...)
		v.state.pagination.totalPages = int(math.Ceil(float64(len(searchResult.entries)) /
			float64(v.state.pagination.pageSize)))
		if v.state.pagination.totalPages < 1 {
			v.state.pagination.totalPages = 1
		}
		v.state.mu.Unlock()

		// Queue UI updates
		v.manager.App().QueueUpdateDraw(func() {
			//v.updateIndexStats()
			v.displayCurrentPage()
			v.updateHeader()
			v.manager.UpdateStatusBar(fmt.Sprintf("Found %d results total (displaying %d)",
				searchResult.totalHits, len(searchResult.entries)))
		})
	}()
}

func (v *View) resetFieldState() {
	v.state.mu.Lock()
	v.state.data.originalFields = nil
	v.state.data.fieldOrder = nil
	v.state.data.activeFields = make(map[string]bool)
	v.state.data.columnCache = make(map[string][]string)
	v.state.mu.Unlock()

	v.components.fieldList.Clear()
	v.components.selectedList.Clear()

	go func() {
		if err := v.loadFields(); err != nil {
			v.manager.Logger().Error("Failed to load fields for new index", "error", err)
			v.manager.App().QueueUpdateDraw(func() {
				v.manager.UpdateStatusBar(fmt.Sprintf("Error loading fields: %v", err))
			})
		}
	}()
}

func (v *View) loadFields() error {
	query := map[string]interface{}{
		"size": 10,
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
	}

	result, err := v.executeSearch(query)
	if err != nil {
		return err
	}

	entries, err := v.processSearchResults(result.Hits.Hits)
	if err != nil {
		return err
	}

	activeFields := make(map[string]struct{})
	for _, entry := range entries {
		fields := entry.GetAvailableFields()
		for _, field := range fields {
			activeFields[field] = struct{}{}
		}
	}

	// Only fetch field metadata if it isn't cached
	needMetadata := false
	for field := range activeFields {
		if _, ok := v.state.data.fieldCache.Get(field); !ok {
			needMetadata = true
			break
		}
	}

	if needMetadata {
		// Fetch metadata for all fields
		res, err := v.service.Client.FieldCaps(
			v.service.Client.FieldCaps.WithIndex(v.state.search.currentIndex),
			v.service.Client.FieldCaps.WithBody(strings.NewReader(`{"fields": "*"}`)),
		)
		if err != nil {
			return fmt.Errorf("field caps error: %v", err)
		}
		defer res.Body.Close()

		var fieldCaps struct {
			Fields map[string]map[string]struct {
				Type         string `json:"type"`
				Searchable   bool   `json:"searchable"`
				Aggregatable bool   `json:"aggregatable"`
			} `json:"fields"`
		}

		if err := json.NewDecoder(res.Body).Decode(&fieldCaps); err != nil {
			return fmt.Errorf("error decoding field caps: %v", err)
		}

		// Update cache with new metadata
		for field, types := range fieldCaps.Fields {
			for typeName, meta := range types {
				v.state.data.fieldCache.Set(field, &FieldMetadata{
					Type:         typeName,
					Searchable:   meta.Searchable,
					Aggregatable: meta.Aggregatable,
					Active:       false,
				})
				break
			}
		}
	}

	// Process fields with locking
	v.state.mu.Lock()
	defer v.state.mu.Unlock()

	// Create sorted list of active fields
	fields := make([]string, 0, len(activeFields))
	for field := range activeFields {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	// Update state
	v.state.data.originalFields = fields
	v.state.data.fieldOrder = make([]string, len(fields))
	copy(v.state.data.fieldOrder, fields)

	// Update field metadata active status
	for field := range activeFields {
		if meta, ok := v.state.data.fieldCache.Get(field); ok {
			meta.Active = true
		}
	}

	// Preserve existing active field selections that are still valid
	newActiveFields := make(map[string]bool)
	for field := range v.state.data.activeFields {
		if _, ok := activeFields[field]; ok {
			newActiveFields[field] = true
		}
	}
	v.state.data.activeFields = newActiveFields

	return nil
}

func (v *View) updateFieldsFromResults(results []*DocEntry) {
	if len(results) == 0 {
		return
	}

	newFields := make(map[string]struct{})
	for _, entry := range results {
		fields := entry.GetAvailableFields()
		for _, field := range fields {
			newFields[field] = struct{}{}
		}
	}

	v.state.mu.Lock()
	defer v.state.mu.Unlock()

	if len(newFields) == len(v.state.data.originalFields) {
		allMatch := true
		for _, field := range v.state.data.originalFields {
			if _, ok := newFields[field]; !ok {
				allMatch = false
				break
			}
		}
		if allMatch {
			return // No changes needed
		}
	}

	fields := make([]string, 0, len(newFields))
	for field := range newFields {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	// Update state atomically
	v.state.data.originalFields = fields
	v.state.data.fieldOrder = make([]string, len(fields))
	copy(v.state.data.fieldOrder, fields)

	// Update field metadata
	for field := range newFields {
		if meta, ok := v.state.data.fieldCache.Get(field); ok {
			meta.Active = true
		}
	}

	// Clean up field selections
	newActiveFields := make(map[string]bool)
	for field := range v.state.data.activeFields {
		if _, ok := newFields[field]; ok {
			newActiveFields[field] = true
		}
	}
	v.state.data.activeFields = newActiveFields

	// Queue UI update
	v.manager.App().QueueUpdateDraw(func() {
		v.rebuildFieldList()
	})
}

func (v *View) fetchRegularResults(query map[string]any, numResults int, index string) (*searchResult, error) {
	query["size"] = numResults

	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Wait according to rate limit
		v.state.misc.rateLimit.Wait()

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(query); err != nil {
			return nil, fmt.Errorf("error encoding query: %v", err)
		}

		res, err := v.service.Client.Search(
			v.service.Client.Search.WithIndex(index),
			v.service.Client.Search.WithBody(&buf),
			v.service.Client.Search.WithScroll(5*time.Minute),
		)

		if err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "429") {
				v.state.misc.rateLimit.HandleTooManyRequests()
				v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited (attempt %d/%d), retrying in %v...",
					attempt+1, maxRetries, v.state.misc.rateLimit.GetRetryAfter()))
				continue
			}
			return nil, fmt.Errorf("search error: %v", err)
		}
		defer res.Body.Close()

		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response: %v", err)
		}

		// Check if response indicates rate limiting
		if res.StatusCode == 429 {
			v.state.misc.rateLimit.HandleTooManyRequests()
			v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited (attempt %d/%d), retrying in %v...",
				attempt+1, maxRetries, v.state.misc.rateLimit.GetRetryAfter()))
			continue
		}

		v.state.misc.rateLimit.Reset()

		var result elastic.ESSearchResult
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			return nil, fmt.Errorf("error decoding response: %v", err)
		}

		entries, err := v.processSearchResults(result.Hits.Hits)
		if err != nil {
			return nil, fmt.Errorf("error processing results: %v", err)
		}

		return &searchResult{
			entries:   entries,
			totalHits: result.Hits.GetTotalHits(),
		}, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
}

func (v *View) fetchLargeResults(query map[string]any, index string) (*searchResult, error) {
	query["size"] = 1000
	maxRetries := 3
	var lastErr error

	// Initial scroll request with retries
	var scrollID string
	var allResults []*DocEntry

	for attempt := 0; attempt < maxRetries; attempt++ {
		v.state.misc.rateLimit.Wait()

		queryJSON, err := json.Marshal(query)
		if err != nil {
			return nil, fmt.Errorf("error creating query: %v", err)
		}

		res, err := v.service.Client.Search(
			v.service.Client.Search.WithIndex(index),
			v.service.Client.Search.WithBody(strings.NewReader(string(queryJSON))),
			v.service.Client.Search.WithScroll(time.Duration(5)*time.Minute),
		)
		if err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "429") {
				v.state.misc.rateLimit.HandleTooManyRequests()
				v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited (attempt %d/%d), retrying in %v...",
					attempt+1, maxRetries, v.state.misc.rateLimit.GetRetryAfter()))
				continue
			}
			return nil, fmt.Errorf("initial scroll error: %v", err)
		}

		bodyBytes, err := io.ReadAll(res.Body)
		res.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("error reading response: %v", err)
		}

		if res.StatusCode == 429 {
			v.state.misc.rateLimit.HandleTooManyRequests()
			v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited (attempt %d/%d), retrying in %v...",
				attempt+1, maxRetries, v.state.misc.rateLimit.GetRetryAfter()))
			continue
		}

		var result elastic.ESSearchResult
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			return nil, fmt.Errorf("error decoding response: %v", err)
		}

		entries, err := v.processSearchResults(result.Hits.Hits)
		if err != nil {
			return nil, fmt.Errorf("error processing batch: %v", err)
		}
		allResults = append(allResults, entries...)
		scrollID = result.ScrollID

		// Successfully got first batch
		v.state.misc.rateLimit.Reset()
		break
	}

	if scrollID == "" {
		return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
	}

	// Continue scrolling with retries
	for {
		var scrollResult elastic.ESSearchResult

		// Try to get next batch with retries
		for attempt := 0; attempt < maxRetries; attempt++ {
			v.state.misc.rateLimit.Wait()

			scrollRes, err := v.service.Client.Scroll(
				v.service.Client.Scroll.WithScrollID(scrollID),
				v.service.Client.Scroll.WithScroll(time.Duration(5)*time.Minute),
			)
			if err != nil {
				if strings.Contains(err.Error(), "429") {
					v.state.misc.rateLimit.HandleTooManyRequests()
					v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited (attempt %d/%d), retrying in %v...",
						attempt+1, maxRetries, v.state.misc.rateLimit.GetRetryAfter()))
					continue
				}
				return nil, fmt.Errorf("scroll error: %v", err)
			}

			if scrollRes.StatusCode == 429 {
				scrollRes.Body.Close()
				v.state.misc.rateLimit.HandleTooManyRequests()
				v.manager.UpdateStatusBar(fmt.Sprintf("Rate limited (attempt %d/%d), retrying in %v...",
					attempt+1, maxRetries, v.state.misc.rateLimit.GetRetryAfter()))
				continue
			}

			bodyBytes, err := io.ReadAll(scrollRes.Body)
			scrollRes.Body.Close()

			if err != nil {
				return nil, fmt.Errorf("error reading scroll response: %v", err)
			}

			if err := json.Unmarshal(bodyBytes, &scrollResult); err != nil {
				return nil, fmt.Errorf("error decoding scroll response: %v", err)
			}

			v.state.misc.rateLimit.Reset()
			break
		}

		if len(scrollResult.Hits.Hits) == 0 {
			// Clean up scroll
			_, err := v.service.Client.ClearScroll(
				v.service.Client.ClearScroll.WithScrollID(scrollID),
			)
			if err != nil {
				v.manager.Logger().Error("Failed to clear scroll", "error", err)
			}
			break
		}

		entries, err := v.processSearchResults(scrollResult.Hits.Hits)
		if err != nil {
			return nil, fmt.Errorf("error processing batch: %v", err)
		}
		allResults = append(allResults, entries...)
	}

	return &searchResult{
		entries:   allResults,
		totalHits: len(allResults),
	}, nil
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

	// Build summary items in a single pass
	summary = append(summary,
		types.SummaryItem{Key: "Index", Value: indexInfo},
		types.SummaryItem{Key: "Filters", Value: fmt.Sprintf("%d", len(v.state.data.filters))},
		types.SummaryItem{Key: "Results", Value: fmt.Sprintf("%d", len(v.state.data.displayedResults))},
		types.SummaryItem{Key: "Page", Value: fmt.Sprintf("[%s::b]%d/%d[-]", style.GruvboxMaterial.Yellow, v.state.pagination.currentPage, v.state.pagination.totalPages)},
		types.SummaryItem{Key: "Timeframe", Value: v.components.timeframeInput.GetText()},
	)

	v.manager.UpdateHeader(summary)
}

func (v *View) executeBatchSearch(query map[string]any, index string) (*searchResult, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		v.state.misc.rateLimit.Wait()

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(query); err != nil {
			return nil, fmt.Errorf("error encoding query: %v", err)
		}

		res, err := v.service.Client.Search(
			v.service.Client.Search.WithIndex(index),
			v.service.Client.Search.WithBody(&buf),
		)

		if err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "429") {
				v.state.misc.rateLimit.HandleTooManyRequests()
				continue
			}
			return nil, fmt.Errorf("search error: %v", err)
		}

		bodyBytes, err := io.ReadAll(res.Body)
		res.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("error reading response: %v", err)
		}

		if res.StatusCode == 429 {
			v.state.misc.rateLimit.HandleTooManyRequests()
			continue
		}

		v.state.misc.rateLimit.Reset()

		var result elastic.ESSearchResult
		if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&result); err != nil {
			return nil, fmt.Errorf("error decoding response: %v", err)
		}

		entries, err := v.processSearchResults(result.Hits.Hits)
		if err != nil {
			return nil, fmt.Errorf("error processing results: %v", err)
		}

		return &searchResult{
			entries:   entries,
			totalHits: result.Hits.GetTotalHits(),
		}, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
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

	// Pre-calculate display headers with capacity hint
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

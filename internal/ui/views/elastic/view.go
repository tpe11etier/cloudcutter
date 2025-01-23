package elastic

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/services/elastic"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	"strings"
)

type View struct {
	manager    *manager.Manager
	components viewComponents
	service    *elastic.Service
	state      State
	layout     tview.Primitive
}

type viewComponents struct {
	content          *tview.Flex
	filterInput      *tview.InputField
	activeFilters    *tview.TextView
	indexInput       *tview.InputField
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

	v.setupLayout()
	v.initTimeframeState()
	err := v.initFieldsSync()
	if err != nil {
		v.manager.Logger().Error("Failed to initialize fields", "error", err)
		return v, err
	}

	v.components.timeframeInput.SetText("today")
	v.refreshWithCurrentTimeframe()

	manager.SetFocus(v.components.filterInput)
	v.manager.Logger().Info("Elastic View successfully initialized")
	return v, nil
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
	case v.components.indexInput:
		return "indexInput"
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
		case tcell.KeyBacktab:
			return v.handleShiftTabKey(currentFocus)

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
				v.manager.SetFocus(v.components.filterInput)
			}
			return nil
		}

		switch currentFocus {
		case v.components.filterInput:
			return v.handleFilterInput(event)
		case v.components.activeFilters:
			return v.handleActiveFilters(event)
		case v.components.indexInput:
			return v.handleIndexInput(event)
		case v.components.fieldList:
			return v.handleFieldList(event)
		case v.components.selectedList:
			return v.handleSelectedList(event)
		case v.components.timeframeInput:
			return v.handleTimeframeInput(event)
		case v.components.numResultsInput:
			return v.handleNumResultsInput(event)

		case v.components.resultsTable:
			return v.handleResultsTable(event)
		case v.components.localFilterInput:
			return v.handleLocalFilterInput(event)
		}

		return event
	}
}

func (v *View) Reinitialize(cfg aws.Config) error {
	if err := v.service.Reinitialize(cfg, v.manager.CurrentProfile()); err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error reinitializing ES service: %v", err))
		return err
	}

	if cfg.Region == "local" {
		v.components.timeframeInput.SetText("")
		v.state.search.timeframe = ""
	}

	v.state.mu.Lock()
	v.state.data.fieldCache = NewFieldCache()
	v.state.data.originalFields = nil
	v.state.data.fieldOrder = nil
	v.state.data.activeFields = make(map[string]bool)
	v.state.mu.Unlock()

	v.manager.App().QueueUpdateDraw(func() {
		v.components.fieldList.Clear()
		v.components.selectedList.Clear()
		v.manager.SetFocus(v.components.filterInput)
	})

	// Load fields and rebuild UI
	if err := v.loadFields(); err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error loading fields: %v", err))
		return err
	}

	v.manager.App().QueueUpdateDraw(func() {
		v.rebuildFieldList()
	})

	v.refreshResults()
	return nil
}

func (v *View) Show() {
	v.manager.SetFocus(v.components.filterInput)
	v.refreshResults()
}

func (v *View) Close() error {
	if v.manager.Logger() != nil {
		return v.manager.Logger().Close()
	}
	return nil
}

func (v *View) initTimeframeState() {
	defaultTimeframe := "today"

	v.state.search.timeframe = defaultTimeframe
	v.components.timeframeInput.SetText(defaultTimeframe)

	v.refreshWithCurrentTimeframe()
}

func (v *View) refreshWithCurrentTimeframe() {
	timeframe := strings.TrimSpace(v.components.timeframeInput.GetText())
	v.state.search.timeframe = timeframe
	v.refreshResults()
}

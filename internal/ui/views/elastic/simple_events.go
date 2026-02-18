package elastic

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type SimpleComponentType int

const (
	UnknownSimpleComponent SimpleComponentType = iota
	ResultsTableSimpleComponent
	FieldListSimpleComponent
	SelectedListSimpleComponent
	FilterInputSimpleComponent
	IndexInputSimpleComponent
	LocalFilterSimpleComponent
	TimeframeInputSimpleComponent
	NumResultsInputSimpleComponent
)

type SimpleHandlerContext struct {
	View         *View
	ErrorHandler *ElasticErrorHandler
	Logger       interface{ Debug(string, ...interface{}) }
	RateLimiter  *RateLimiter
}

type SimpleComponentHandler interface {
	CanHandle(event *tcell.EventKey, component tview.Primitive) bool
	HandleEvent(event *tcell.EventKey, ctx *SimpleHandlerContext) *tcell.EventKey
}

type SimpleEventBus struct {
	handlers     map[SimpleComponentType]SimpleComponentHandler
	errorHandler *ElasticErrorHandler
	logger       interface{ Debug(string, ...interface{}) }
	rateLimiter  *RateLimiter
	view         *View
}

func NewSimpleEventBus(view *View, errorHandler *ElasticErrorHandler, logger interface{ Debug(string, ...interface{}) }, rateLimiter *RateLimiter) *SimpleEventBus {
	bus := &SimpleEventBus{
		handlers:     make(map[SimpleComponentType]SimpleComponentHandler),
		errorHandler: errorHandler,
		logger:       logger,
		rateLimiter:  rateLimiter,
		view:         view,
	}

	bus.RegisterHandler(ResultsTableSimpleComponent, &SimpleResultsTableHandler{})
	bus.RegisterHandler(FieldListSimpleComponent, &SimpleFieldListHandler{})
	bus.RegisterHandler(SelectedListSimpleComponent, &SimpleSelectedListHandler{})
	bus.RegisterHandler(FilterInputSimpleComponent, &SimpleFilterInputHandler{})

	return bus
}

func (eb *SimpleEventBus) RegisterHandler(componentType SimpleComponentType, handler SimpleComponentHandler) {
	eb.handlers[componentType] = handler
}

func (eb *SimpleEventBus) ProcessEvent(event *tcell.EventKey, currentFocus tview.Primitive) *tcell.EventKey {
	componentType := eb.getComponentType(currentFocus)
	if handler, exists := eb.handlers[componentType]; exists {
		if handler.CanHandle(event, currentFocus) {
			ctx := &SimpleHandlerContext{
				View:         eb.view,
				ErrorHandler: eb.errorHandler,
				Logger:       eb.logger,
				RateLimiter:  eb.rateLimiter,
			}
			return handler.HandleEvent(event, ctx)
		}
	}

	return eb.handleGlobalShortcuts(event, currentFocus)
}

func (eb *SimpleEventBus) getComponentType(primitive tview.Primitive) SimpleComponentType {
	componentMap := map[tview.Primitive]SimpleComponentType{
		eb.view.components.resultsTable:     ResultsTableSimpleComponent,
		eb.view.components.fieldList:        FieldListSimpleComponent,
		eb.view.components.selectedList:     SelectedListSimpleComponent,
		eb.view.components.filterInput:      FilterInputSimpleComponent,
		eb.view.components.indexInput:       IndexInputSimpleComponent,
		eb.view.components.localFilterInput: LocalFilterSimpleComponent,
		eb.view.components.timeframeInput:   TimeframeInputSimpleComponent,
		eb.view.components.numResultsInput:  NumResultsInputSimpleComponent,
	}

	if componentType, exists := componentMap[primitive]; exists {
		return componentType
	}
	return UnknownSimpleComponent
}

func (eb *SimpleEventBus) handleGlobalShortcuts(event *tcell.EventKey, currentFocus tview.Primitive) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEsc:
		if currentFocus != eb.view.components.resultsTable && currentFocus != eb.view.components.fieldList {
			eb.view.manager.HideAllModals()
			eb.view.manager.SetFocus(eb.view.components.filterInput)
			return nil
		}
	case tcell.KeyTab:
		eb.handleTabNavigation(currentFocus)
		return nil
	}

	return event
}

func (eb *SimpleEventBus) handleTabNavigation(currentFocus tview.Primitive) {
	switch currentFocus {
	case eb.view.components.filterInput:
		eb.view.manager.SetFocus(eb.view.components.indexInput)
	case eb.view.components.indexInput:
		eb.view.manager.SetFocus(eb.view.components.timeframeInput)
	case eb.view.components.timeframeInput:
		eb.view.manager.SetFocus(eb.view.components.numResultsInput)
	case eb.view.components.numResultsInput:
		eb.view.manager.SetFocus(eb.view.components.localFilterInput)
	case eb.view.components.localFilterInput:
		eb.view.manager.SetFocus(eb.view.components.fieldList)
	case eb.view.components.fieldList:
		eb.view.manager.SetFocus(eb.view.components.selectedList)
	case eb.view.components.selectedList:
		eb.view.manager.SetFocus(eb.view.components.resultsTable)
	case eb.view.components.resultsTable:
		eb.view.manager.SetFocus(eb.view.components.filterInput)
	default:
		eb.view.manager.SetFocus(eb.view.components.filterInput)
	}
}

type SimpleResultsTableHandler struct{}

func (h *SimpleResultsTableHandler) CanHandle(event *tcell.EventKey, component tview.Primitive) bool {
	switch event.Key() {
	case tcell.KeyEnter:
		return true
	case tcell.KeyRune:
		switch event.Rune() {
		case 'r', 'n', 'p':
			return true
		}
	}
	return false
}

func (h *SimpleResultsTableHandler) HandleEvent(event *tcell.EventKey, ctx *SimpleHandlerContext) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		row, _ := ctx.View.components.resultsTable.GetSelection()
		if row <= 0 {
			return nil
		}

		go func() {
			entry, err := h.getDocumentEntry(ctx.View, row)
			if err != nil {
				ctx.View.manager.App().QueueUpdateDraw(func() {
					ctx.View.manager.UpdateStatusBar(fmt.Sprintf("Error: %v", err))
				})
				return
			}

			ctx.View.manager.App().QueueUpdateDraw(func() {
				ctx.View.showJSONModal(entry)
			})
		}()
		return nil

	case tcell.KeyRune:
		switch event.Rune() {
		case 'r':
			ctx.View.toggleRowNumbers()
			return nil
		case 'n':
			ctx.View.nextPage()
			return nil
		case 'p':
			ctx.View.previousPage()
			return nil
		}
	}

	return event
}

func (h *SimpleResultsTableHandler) getDocumentEntry(view *View, row int) (*DocEntry, error) {
	view.state.mu.RLock()
	defer view.state.mu.RUnlock()

	currentPage := view.state.pagination.currentPage
	pageSize := view.state.pagination.pageSize
	displayedResults := view.state.data.displayedResults

	actualIndex := (currentPage-1)*pageSize + (row - 1)
	if actualIndex >= len(displayedResults) {
		return nil, fmt.Errorf("invalid document index")
	}

	return displayedResults[actualIndex], nil
}

type SimpleFieldListHandler struct{}

func (h *SimpleFieldListHandler) CanHandle(event *tcell.EventKey, component tview.Primitive) bool {
	return event.Key() == tcell.KeyEnter
}

func (h *SimpleFieldListHandler) HandleEvent(event *tcell.EventKey, ctx *SimpleHandlerContext) *tcell.EventKey {
	if event.Key() == tcell.KeyEnter {
		if _, text, valid := h.getCurrentListItem(ctx.View.components.fieldList); valid {
			ctx.View.toggleField(text)
		}
		return nil
	}
	return event
}

func (h *SimpleFieldListHandler) getCurrentListItem(list *tview.List) (int, string, bool) {
	if list.GetItemCount() == 0 {
		return -1, "", false
	}
	index := list.GetCurrentItem()
	if index < 0 || index >= list.GetItemCount() {
		return -1, "", false
	}
	text, _ := list.GetItemText(index)
	return index, text, true
}

type SimpleSelectedListHandler struct{}

func (h *SimpleSelectedListHandler) CanHandle(event *tcell.EventKey, component tview.Primitive) bool {
	return event.Key() == tcell.KeyEnter
}

func (h *SimpleSelectedListHandler) HandleEvent(event *tcell.EventKey, ctx *SimpleHandlerContext) *tcell.EventKey {
	if event.Key() == tcell.KeyEnter {
		if _, text, valid := h.getCurrentListItem(ctx.View.components.selectedList); valid {
			ctx.View.toggleField(text)
		}
		return nil
	}
	return event
}

func (h *SimpleSelectedListHandler) getCurrentListItem(list *tview.List) (int, string, bool) {
	if list.GetItemCount() == 0 {
		return -1, "", false
	}
	index := list.GetCurrentItem()
	if index < 0 || index >= list.GetItemCount() {
		return -1, "", false
	}
	text, _ := list.GetItemText(index)
	return index, text, true
}

type SimpleFilterInputHandler struct{}

func (h *SimpleFilterInputHandler) CanHandle(event *tcell.EventKey, component tview.Primitive) bool {
	return event.Key() == tcell.KeyEnter
}

func (h *SimpleFilterInputHandler) HandleEvent(event *tcell.EventKey, ctx *SimpleHandlerContext) *tcell.EventKey {
	if event.Key() == tcell.KeyEnter {
		currentFocus := ctx.View.manager.App().GetFocus()

		switch currentFocus {
		case ctx.View.components.filterInput,
			ctx.View.components.localFilterInput,
			ctx.View.components.indexInput,
			ctx.View.components.numResultsInput:
			ctx.View.refreshResults()

		case ctx.View.components.timeframeInput:
			ctx.View.manager.UpdateStatusBar("Timeframe processing temporarily disabled for debugging")
			return nil
		}

		return nil
	}
	return event
}

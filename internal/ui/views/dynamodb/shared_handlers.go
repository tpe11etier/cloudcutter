package dynamodb

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/tpelletiersophos/cloudcutter/internal/ui/common"
)

type DynamoDBDataTableHandler struct {
	*common.BaseHandler
}

func NewDynamoDBDataTableHandler(view common.ViewInterface) *DynamoDBDataTableHandler {
	return &DynamoDBDataTableHandler{
		BaseHandler: common.NewBaseHandler(DataTableComponent, view),
	}
}

func (h *DynamoDBDataTableHandler) CanHandle(event *tcell.EventKey, component tview.Primitive) bool {
	switch event.Key() {
	case tcell.KeyEnter, tcell.KeyEsc:
		return true
	case tcell.KeyRune:
		switch event.Rune() {
		case 'r', 'n', 'p', '/':
			return true
		}
	}
	return false
}

func (h *DynamoDBDataTableHandler) HandleEvent(event *tcell.EventKey, ctx *common.HandlerContext) *tcell.EventKey {
	view := ctx.View.(*View)

	switch event.Key() {
	case tcell.KeyEsc:
		view.manager.SetFocus(view.leftPanel)
		return nil

	case tcell.KeyEnter:
		row, _ := view.dataTable.GetSelection()
		if row > 0 {
			start := (view.state.currentPage - 1) * view.state.pageSize
			itemIndex := start + row - 1
			if itemIndex < len(view.state.filteredItems) {
				view.showItemDetails(view.state.filteredItems[itemIndex])
			}
		}
		return nil

	case tcell.KeyRune:
		switch event.Rune() {
		case 'r':
			view.toggleRowNumbers()
			return nil
		case 'n':
			view.nextPage()
			return nil
		case 'p':
			view.previousPage()
			return nil
		case '/':
			view.showFilterPrompt(view.dataTable)
			return nil
		}
	}

	return event
}

type DynamoDBLeftPanelHandler struct {
	*common.BaseHandler
}

func NewDynamoDBLeftPanelHandler(view common.ViewInterface) *DynamoDBLeftPanelHandler {
	return &DynamoDBLeftPanelHandler{
		BaseHandler: common.NewBaseHandler(LeftPanelComponent, view),
	}
}

func (h *DynamoDBLeftPanelHandler) CanHandle(event *tcell.EventKey, component tview.Primitive) bool {
	switch event.Key() {
	case tcell.KeyEnter:
		return true
	case tcell.KeyRune:
		if event.Rune() == '/' {
			return true
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return true
	}
	return false
}

func (h *DynamoDBLeftPanelHandler) HandleEvent(event *tcell.EventKey, ctx *common.HandlerContext) *tcell.EventKey {
	view := ctx.View.(*View)

	switch event.Key() {
	case tcell.KeyEnter:
		index := view.leftPanel.GetCurrentItem()
		if index >= 0 && index < view.leftPanel.GetItemCount() {
			tableName, _ := view.leftPanel.GetItemText(index)
			view.showTableItems(tableName)
		}
		return nil

	case tcell.KeyRune:
		if event.Rune() == '/' {
			view.showFilterPrompt(view.leftPanel)
			return nil
		}

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		view.state.leftPanelFilter = ""
		view.filterLeftPanel("")
		return nil
	}

	return event
}

type DynamoDBFilterPromptHandler struct {
	*common.BaseHandler
}

func NewDynamoDBFilterPromptHandler(view common.ViewInterface) *DynamoDBFilterPromptHandler {
	return &DynamoDBFilterPromptHandler{
		BaseHandler: common.NewBaseHandler(FilterPromptComponent, view),
	}
}

func (h *DynamoDBFilterPromptHandler) CanHandle(event *tcell.EventKey, component tview.Primitive) bool {
	return event.Key() == tcell.KeyEnter
}

func (h *DynamoDBFilterPromptHandler) HandleEvent(event *tcell.EventKey, ctx *common.HandlerContext) *tcell.EventKey {
	if event.Key() == tcell.KeyEnter {
		return nil
	}
	return event
}
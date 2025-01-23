package help

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/style"
)

const (
	ModalHelp = "modalHelp"
)

type Command struct {
	Key         string
	Description string
}

type HelpCategory struct {
	Title    string
	Commands []Command
}

type HelpProperties struct {
	Commands []Command
}

type Help struct {
	*tview.Flex
	globalCategories []HelpCategory
	contextCategory  *HelpCategory
	table            *tview.Table
	selectedCommand  *Command
	isVisible        bool
	Selectable       bool
	Props            *HelpProperties
}

func (h *Help) getCommandForRow(row int) *Command {
	// Map table row back to Command
	if h.contextCategory != nil {
		if row > 0 && row <= len(h.contextCategory.Commands) {
			return &h.contextCategory.Commands[row-1]
		}
	}
	return nil
}
func NewHelp() *Help {
	help := &Help{
		Flex:       tview.NewFlex(),
		table:      tview.NewTable(),
		Props:      &HelpProperties{},
		Selectable: false,
		globalCategories: []HelpCategory{
			{
				Title: "General Commands",
				Commands: []Command{
					{Key: "Enter", Description: "Execute command"},
					{Key: "Esc", Description: "Close prompt/Go back"},
					{Key: "?", Description: "Show help"},
					{Key: "/", Description: "Filter"},
					{Key: ":", Description: "Command prompt"},
					{Key: "Tab", Description: "Cycle through fields"},
					{Key: "Shift+Tab", Description: "Cycle through fields (reverse)"},
					{Key: ":region", Description: "Change AWS region"},
					{Key: ":profile", Description: "Change Profile"},
				},
			},
			{
				Title: "Views",
				Commands: []Command{
					{Key: ":dynamodb", Description: "Switch to DynamoDB view"},
					{Key: ":elastic", Description: "Switch to Elastic view"},
				},
			},
		},
	}

	help.table.SetBorders(false)
	help.setupLayout()

	return help
}

func (h *Help) setupLayout() {
	h.Clear()
	h.SetDirection(tview.FlexColumn)
	h.AddItem(h.table, 0, 1, true)

	h.table.Clear()
	row := 0

	if h.contextCategory != nil {
		h.addCategoryToTable(h.contextCategory, &row)
	} else {
		for _, category := range h.globalCategories {
			h.addCategoryToTable(&category, &row)
		}
	}
	h.table.SetSelectable(h.Selectable, false)
}

func (h *Help) addCategoryToTable(category *HelpCategory, row *int) {
	h.table.SetCell(*row, 0,
		tview.NewTableCell(fmt.Sprintf("[::b]%s", category.Title)).
			SetTextColor(style.GruvboxMaterial.Yellow).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))
	*row++

	for _, cmd := range category.Commands {
		keyCell := tview.NewTableCell(fmt.Sprintf("  [mediumturquoise]%s", cmd.Key)).
			SetTextColor(tcell.ColorMediumTurquoise).
			SetAlign(tview.AlignLeft)

		descCell := tview.NewTableCell(fmt.Sprintf("  [beige]%s", cmd.Description)).
			SetTextColor(tcell.ColorBeige).
			SetAlign(tview.AlignLeft)

		h.table.SetCell(*row, 0, keyCell)
		h.table.SetCell(*row, 1, descCell)
		*row++
	}

	h.table.SetCell(*row, 0, tview.NewTableCell(""))
	*row++
}

func (h *Help) Show(pages *tview.Pages, onDone func()) {
	pages.RemovePage(ModalHelp)

	h.isVisible = true
	h.setupLayout()

	h.SetBorder(true).
		SetTitle(" Help ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(tcell.ColorMediumTurquoise).
		SetBorderColor(tcell.ColorMediumTurquoise)

	modal := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().
				AddItem(nil, 0, 1, false).
				AddItem(h, 100, 1, true).
				AddItem(nil, 0, 1, false),
			25, 1, true).
		AddItem(nil, 0, 1, false)

	pages.AddPage(ModalHelp, modal, true, true)
}

func (h *Help) Hide(pages *tview.Pages) {
	h.isVisible = false
	h.contextCategory = nil
	h.setupLayout()
	pages.RemovePage(ModalHelp)
}

func (h *Help) SetContextHelp(category *HelpCategory) {
	h.contextCategory = category
	h.setupLayout()
}

func (h *Help) ClearContextHelp() {
	h.contextCategory = nil
	h.setupLayout()
}

func (h *Help) IsVisible() bool {
	return h.isVisible
}

func (h *Help) SetCommands(commands []Command) {
	h.contextCategory = &HelpCategory{
		Title:    "Component Help",
		Commands: commands,
	}
	h.setupLayout()
}

func (h *Help) SetFocusFunc(handler func(*Help)) {
	h.table.SetFocusFunc(func() {
		handler(h)
	})
}

func (h *Help) SetBlurFunc(handler func(*Help)) {
	h.table.SetBlurFunc(func() {
		handler(h)
	})
}

func (h *Help) SetProperties(props HelpProperties) {
	if h.contextCategory == nil {
		h.contextCategory = &HelpCategory{
			Title:    "Component Help",
			Commands: props.Commands,
		}
	}
	h.setupLayout()
}

func (h *Help) SetSelectable(b bool) {
	h.Selectable = b
	h.setupLayout()
}

func (h *Help) GetContextHelp() *HelpCategory {
	return h.contextCategory
}

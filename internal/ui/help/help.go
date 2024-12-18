package help

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	ModalHelp = "helpModal"
)

type Command struct {
	Key         string
	Description string
}

type HelpCategory struct {
	Title    string
	Commands []Command
}

type Help struct {
	*tview.Flex
	globalCategories []HelpCategory
	contextCategory  *HelpCategory // Current context-specific commands
	table            *tview.Table
	isVisible        bool
}

func NewHelp() *Help {
	help := &Help{
		Flex:  tview.NewFlex(),
		table: tview.NewTable(),
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
				},
			},
			{
				Title: "Views",
				Commands: []Command{
					{Key: ":dynamodb", Description: "Switch to DynamoDB view"},
					{Key: ":s3", Description: "Switch to S3 view"},
					{Key: ":ec2", Description: "Switch to EC2 view"},
					{Key: ":ecr", Description: "Switch to ECR view"},
					{Key: ":elastic", Description: "Switch to Elastic view"},
				},
			},
			{
				Title: "AWS",
				Commands: []Command{
					{Key: ":region", Description: "Change AWS region"},
					{Key: ":profile", Description: "Change Opal profile"},
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

	// If we have context-specific help, ONLY show that
	if h.contextCategory != nil {
		h.addCategoryToTable(h.contextCategory, &row)
	} else {
		for _, category := range h.globalCategories {
			h.addCategoryToTable(&category, &row)
		}
	}
}

func (h *Help) addCategoryToTable(category *HelpCategory, row *int) {
	// Add category title
	h.table.SetCell(*row, 0,
		tview.NewTableCell(fmt.Sprintf("[::b]%s", category.Title)).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1))
	*row++

	// Add commands
	for _, cmd := range category.Commands {
		keyCell := tview.NewTableCell(fmt.Sprintf("  [mediumturquoise]%s", cmd.Key)).
			SetTextColor(tcell.ColorMediumTurquoise).
			SetAlign(tview.AlignLeft).
			SetSelectable(false)

		descCell := tview.NewTableCell(fmt.Sprintf("  [beige]%s", cmd.Description)).
			SetTextColor(tcell.ColorBeige).
			SetAlign(tview.AlignLeft).
			SetSelectable(false)

		h.table.SetCell(*row, 0, keyCell)
		h.table.SetCell(*row, 1, descCell)
		*row++
	}

	// Add spacing after category
	h.table.SetCell(*row, 0, tview.NewTableCell("").SetSelectable(false))
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

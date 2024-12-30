package header

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/types"
)

type Action struct {
	Shortcut    string
	Description string
}

type EnvVar struct {
	Key   string
	Value string
}

type Header struct {
	*tview.Flex
	leftTable    *tview.Table
	rightTable   *tview.Table
	envVars      map[string]string
	actions      []Action
	summaryView  *tview.TextView
	envVarRowMap map[string]int // Maps EnvVar.Key to table row
}

type SummaryItem struct {
	Key   string
	Value string
}

func NewHeader() *Header {
	header := &Header{
		Flex:         tview.NewFlex(),
		leftTable:    tview.NewTable(),
		rightTable:   tview.NewTable(),
		envVars:      make(map[string]string),
		envVarRowMap: make(map[string]int),
		actions: []Action{
			{Shortcut: "<:>", Description: "Command Mode"},
			{Shortcut: "<?>", Description: "Help"},
		},
	}

	header.SetBorder(true).SetBorderColor(tcell.ColorMediumTurquoise)
	header.SetDirection(tview.FlexColumn).
		AddItem(header.leftTable, 0, 1, false).
		AddItem(header.rightTable, 0, 1, false)

	// Set default env vars
	header.UpdateEnvVar("Profile", "default")
	header.UpdateEnvVar("Region", "us-west-2")

	header.setupLeftTable()

	return header
}

func (h *Header) setupLeftTable() {
	// Set up headers
	headers := []string{"  Environment Variables", "", "  Actions"}
	for col, headerText := range headers {
		cell := tview.NewTableCell(fmt.Sprintf("[yellow::b]%s", headerText)).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false)
		if headerText == "" {
			cell.SetText("  ")
		}
		h.leftTable.SetCell(0, col, cell)
	}

	for i, action := range h.actions {
		actionText := fmt.Sprintf("[mediumturquoise]%-10s [beige]%s", action.Shortcut, action.Description)
		actionCell := tview.NewTableCell(actionText).
			SetTextColor(tcell.ColorBeige).
			SetAlign(tview.AlignLeft).
			SetSelectable(false)
		h.leftTable.SetCell(i+1, 2, actionCell)
	}
}

func (h *Header) UpdateEnvVar(key, value string) {
	h.envVars[key] = value
	row, exists := h.envVarRowMap[key]
	if !exists {
		row = len(h.envVarRowMap) + 1
		h.envVarRowMap[key] = row
	}

	envText := fmt.Sprintf("[mediumturquoise]%-10s: [beige]%s", key, value)
	envCell := tview.NewTableCell(envText).
		SetTextColor(tcell.ColorBeige).
		SetAlign(tview.AlignLeft).
		SetSelectable(false)
	h.leftTable.SetCell(row, 0, envCell)
	h.leftTable.SetCell(row, 1, tview.NewTableCell("  ")) // Spacer
}

func (h *Header) UpdateSummary(items []types.SummaryItem) {
	h.rightTable.Clear()
	h.rightTable.SetTitle("Summary").SetTitleAlign(tview.AlignRight)

	h.rightTable.SetCell(0, 0, tview.NewTableCell("   Summary").
		SetTextColor(tcell.ColorYellow).
		SetAlign(tview.AlignCenter).
		SetSelectable(false))

	if len(items) == 0 {
		h.rightTable.SetCell(0, 0, tview.NewTableCell("No Summary Available").
			SetTextColor(tcell.ColorBeige).
			SetAlign(tview.AlignLeft).
			SetSelectable(false))
		return
	}

	for i, item := range items {
		keyCell := tview.NewTableCell(fmt.Sprintf("[mediumturquoise::b]%s: ", item.Key)).
			SetTextColor(tcell.ColorMediumTurquoise).
			SetAlign(tview.AlignLeft).
			SetSelectable(false)
		valueCell := tview.NewTableCell(item.Value).
			SetTextColor(tcell.ColorBeige).
			SetAlign(tview.AlignLeft).
			SetSelectable(false)
		h.rightTable.SetCell(i+1, 0, keyCell)
		h.rightTable.SetCell(i+1, 1, valueCell)
	}
}

func (h *Header) ClearSummary() {
	h.rightTable.Clear()
}

func (h *Header) GetHeight() int {
	return 5
}

package header

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/types"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/style"
)

type Action struct {
	Shortcut    string
	Description string
}

type EnvVar struct {
	Key   string
	Value string
}

type ViewCommands struct {
	View        string
	Description string
}

type Header struct {
	*tview.Flex
	leftTable     *tview.Table
	leftMidTable  *tview.Table
	rightMidTable *tview.Table
	rightTable    *tview.Table
	envVars       map[string]string
	actions       []Action
	envVarRowMap  map[string]int
	viewCommands  []ViewCommands
}

func NewHeader() *Header {
	header := &Header{
		Flex:          tview.NewFlex(),
		leftTable:     tview.NewTable(),
		leftMidTable:  tview.NewTable(),
		rightMidTable: tview.NewTable(),
		rightTable:    tview.NewTable(),
		envVars:       make(map[string]string),
		envVarRowMap:  make(map[string]int),
		actions: []Action{
			{Shortcut: "<:>", Description: "Command Mode"},
			{Shortcut: "<?>", Description: "Help"},
		},
	}

	header.SetTitle("[::b] Cloud Cutter ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(style.GruvboxMaterial.Yellow)
	header.SetBorder(true).SetBorderColor(tcell.ColorMediumTurquoise)
	header.SetDirection(tview.FlexColumn).
		AddItem(header.leftTable, 0, 1, false).
		AddItem(header.leftMidTable, 0, 1, false).
		AddItem(header.rightMidTable, 0, 1, false).
		AddItem(header.rightTable, 0, 1, false)

	header.UpdateEnvVar("Profile", "default")
	header.UpdateEnvVar("Region", "us-west-2")
	header.setupLeftTable()
	header.setupLeftMidTable()

	return header
}

type SummaryItem struct {
	Key   string
	Value string
}

func (h *Header) setupLeftTable() {
	// Set up headers
	headers := []string{"  Environment Variables", "", "  Actions"}
	for col, headerText := range headers {
		cell := tview.NewTableCell(headerText).
			SetTextColor(style.GruvboxMaterial.Yellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold)
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
	h.rightMidTable.Clear()
	h.rightMidTable.SetTitle("Summary").SetTitleAlign(tview.AlignRight)

	h.rightMidTable.SetCell(0, 0, tview.NewTableCell("   Summary").
		SetTextColor(style.GruvboxMaterial.Yellow).
		SetAlign(tview.AlignCenter).
		SetSelectable(false).
		SetAttributes(tcell.AttrBold))

	if len(items) == 0 {
		h.rightMidTable.SetCell(0, 0, tview.NewTableCell("No Summary Available").
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
		h.rightMidTable.SetCell(i+1, 0, keyCell)
		h.rightMidTable.SetCell(i+1, 1, valueCell)
	}
}

func (h *Header) ClearSummary() {
	h.rightTable.Clear()
}

func (h *Header) GetHeight() int {
	return 5
}

func (h *Header) setupLeftMidTable() {
	h.leftMidTable.SetCell(0, 0, tview.NewTableCell("  View Commands").
		SetTextColor(style.GruvboxMaterial.Yellow).
		SetAlign(tview.AlignLeft).
		SetSelectable(false).
		SetAttributes(tcell.AttrBold))
}

func (h *Header) SetViewCommands(commands []ViewCommands) {
	h.leftMidTable.Clear()
	h.setupLeftMidTable()

	for i, cmd := range commands {
		cmdText := fmt.Sprintf("[mediumturquoise]%-10s [beige]%s", cmd.View, cmd.Description)
		cmdCell := tview.NewTableCell(cmdText).
			SetTextColor(tcell.ColorBeige).
			SetAlign(tview.AlignLeft).
			SetSelectable(false)
		h.leftMidTable.SetCell(i+1, 0, cmdCell)
	}
}

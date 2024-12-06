package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type StatusBar struct {
	*tview.TextView
	messages    []string
	maxMessages int
}

func NewStatusBar() *StatusBar {
	sb := &StatusBar{
		TextView:    tview.NewTextView(),
		maxMessages: 10,
		messages:    make([]string, 0, 10),
	}

	sb.SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetTextColor(tcell.ColorWhite)

	return sb
}

func (sb *StatusBar) SetText(message string) {
	// Add new message to history
	sb.messages = append(sb.messages, message)
	if len(sb.messages) > sb.maxMessages {
		sb.messages = sb.messages[1:]
	}

	// Update display
	sb.TextView.SetText(message)
}

func (sb *StatusBar) ShowError(err error) {
	if err != nil {
		sb.SetText("[red]Error: " + err.Error())
	}
}

func (sb *StatusBar) ShowSuccess(message string) {
	sb.SetText("[green]" + message)
}

func (sb *StatusBar) ShowWarning(message string) {
	sb.SetText("[yellow]" + message)
}

func (sb *StatusBar) Clear() {
	sb.SetText("")
	sb.messages = make([]string, 0, sb.maxMessages)
}

func (sb *StatusBar) GetHistory() []string {
	return append([]string{}, sb.messages...)
}

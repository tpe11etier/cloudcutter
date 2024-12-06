package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Prompt struct {
	*tview.InputField
	onDone   func(text string)
	onCancel func()
}

func NewPrompt() *Prompt {
	inputField := tview.NewInputField().
		SetLabel("❯ ").
		SetFieldWidth(50).
		SetLabelColor(tcell.ColorMediumTurquoise).
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
		SetPlaceholderTextColor(tcell.ColorDarkGray)

	prompt := &Prompt{
		InputField: inputField,
	}

	inputField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			if prompt.onDone != nil {
				prompt.onDone(inputField.GetText())
			}
		case tcell.KeyEsc:
			if prompt.onCancel != nil {
				prompt.onCancel()
			}
		}
	})

	return prompt
}

func (p *Prompt) SetDoneFunc(handler func(text string)) {
	p.onDone = handler
}

func (p *Prompt) SetCancelFunc(handler func()) {
	p.onCancel = handler
}

func (p *Prompt) SetText(text string) {
	p.InputField.SetText(text)
}

func (p *Prompt) GetText() string {
	return p.InputField.GetText()
}

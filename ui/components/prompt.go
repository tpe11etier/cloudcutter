package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Prompt struct {
	*tview.Flex
	InputField *tview.InputField
	onDone     func(text string)
	onCancel   func()
}

func NewPrompt(label string) *Prompt {
	p := &Prompt{
		Flex:       tview.NewFlex(),
		InputField: tview.NewInputField(),
	}

	p.InputField.
		SetLabel(label).
		SetLabelColor(tcell.ColorTeal).
		SetFieldBackgroundColor(tcell.ColorDefault)

	p.AddItem(p.InputField, 0, 1, true)

	p.InputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			if p.onDone != nil {
				p.onDone(p.InputField.GetText())
			}
			return nil
		case tcell.KeyEsc:
			if p.onCancel != nil {
				p.onCancel()
			}
			return nil
		}
		return event
	})

	return p
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

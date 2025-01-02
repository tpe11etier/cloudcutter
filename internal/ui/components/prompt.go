package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type PromptOptions struct {
	Title      string
	Label      string
	LabelColor tcell.Color
	Width      int
	Height     int
	OnDone     func(text string)
	OnCancel   func()
	OnChanged  func(text string)
}

type Prompt struct {
	*tview.InputField
	options PromptOptions
}

func NewPrompt() *Prompt {
	p := &Prompt{
		InputField: tview.NewInputField(),
	}

	p.InputField.SetFieldBackgroundColor(tcell.ColorBlack)
	p.InputField.SetFieldTextColor(tcell.ColorBeige)

	return p
}

func (p *Prompt) Configure(opts PromptOptions) *Prompt {
	p.options = opts

	p.SetTitle(opts.Title)
	p.SetBorder(true)
	p.SetTitleAlign(tview.AlignLeft)
	p.SetBorderColor(tcell.ColorMediumTurquoise)

	p.InputField.SetLabel(opts.Label)
	p.InputField.SetLabelColor(opts.LabelColor)
	p.InputField.SetFieldWidth(0)

	if opts.OnDone != nil {
		p.InputField.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				opts.OnDone(p.GetText())
			}
		})
	}

	if opts.OnCancel != nil {
		p.InputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEsc {
				opts.OnCancel()
				return nil
			}
			return event
		})
	}

	if opts.OnChanged != nil {
		p.InputField.SetChangedFunc(opts.OnChanged)
	}

	return p
}

func (p *Prompt) SetDoneFunc(fn func(string)) {
	p.options.OnDone = fn
	if fn != nil {
		p.InputField.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				fn(p.GetText())
			}
		})
	} else {
		p.InputField.SetDoneFunc(nil)
	}
}

func (p *Prompt) Layout() *tview.Flex {
	width := p.options.Width
	if width == 0 {
		width = 50
	}

	height := p.options.Height
	if height == 0 {
		height = 3
	}

	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().
				AddItem(nil, 0, 1, false).
				AddItem(p, width, 0, true).
				AddItem(nil, 0, 1, false),
			height, 0, true).
		AddItem(nil, 0, 1, false)
}

func (p *Prompt) GetDoneFunc() func(string) {
	return p.options.OnDone
}

func (p *Prompt) SetCancelFunc(fn func()) {
	p.options.OnCancel = fn
}

func (p *Prompt) SetOnChangeFunc(fn func(string)) {
	p.options.OnChanged = fn
}

func (p *Prompt) SetText(text string) {
	p.InputField.SetText(text)
}

func (p *Prompt) GetText() string {
	return p.InputField.GetText()
}

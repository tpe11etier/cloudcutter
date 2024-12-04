package types

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ServiceView interface {
	Name() string
	GetContent() tview.Primitive
	Show()
	Hide()
	InputHandler() func(event *tcell.EventKey) *tcell.EventKey
}

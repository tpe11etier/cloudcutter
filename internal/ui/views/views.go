package views

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type View interface {
	Name() string
	Content() tview.Primitive
	Show()
	Hide()
	InputHandler() func(event *tcell.EventKey) *tcell.EventKey
	ActiveField() string
}

type Reinitializer interface {
	Reinitialize(cfg aws.Config) error
}

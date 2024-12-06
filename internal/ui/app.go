package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type App struct {
	*tview.Application
	inputCaptures []func(event *tcell.EventKey) *tcell.EventKey
	root          *tview.Pages
}

func NewApp() *App {
	app := &App{
		Application:   tview.NewApplication(),
		inputCaptures: make([]func(event *tcell.EventKey) *tcell.EventKey, 0),
		root:          tview.NewPages(),
	}

	app.Application.SetRoot(app.root, true)

	return app
}

func (app *App) QueueUpdateDraw(f func()) {
	app.QueueUpdate(func() {
		f()
		app.Draw()
	})
}

func (app *App) Suspend(f func()) {
	app.Suspend(f)
}

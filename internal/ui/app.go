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

// (Custom QueueUpdateDraw removed — it called tview.Application.Draw() inside
// a queued update, which itself re-enters QueueUpdate and blocks the main loop
// waiting for a "done" signal that only the main loop can send. Classic
// self-deadlock. Embedded tview.Application.QueueUpdateDraw uses the private
// draw() directly and works correctly, so we just fall through to that.)

func (app *App) Suspend(f func()) {
	app.Suspend(f)
}

package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sync"
)

type App struct {
	*tview.Application
	FocusStack    *Stack
	focusHistory  []tview.Primitive
	focusMutex    sync.Mutex
	inputCaptures []func(event *tcell.EventKey) *tcell.EventKey
	root          *tview.Pages // Store the root pages component
}

// NewApp creates a new App instance with initialized components
func NewApp() *App {
	app := &App{
		Application:   tview.NewApplication(),
		FocusStack:    NewStack(),
		focusHistory:  make([]tview.Primitive, 0),
		inputCaptures: make([]func(event *tcell.EventKey) *tcell.EventKey, 0),
		root:          tview.NewPages(),
	}

	// Set up global input capture
	app.Application.SetInputCapture(app.handleGlobalInput)

	// Set the root pages component
	app.Application.SetRoot(app.root, true)

	return app
}

// GetRoot returns the root pages component
func (app *App) GetRoot() *tview.Pages {
	return app.root
}

// ShowModal updates to use the stored root pages
func (app *App) ShowModal(modal tview.Primitive, width, height int) {
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(modal, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Store current focus
	app.SetFocusWithMemory(modal)

	// Add to root pages directly
	app.root.AddPage("modal", flex, true, true)
}

// HideModal updates to use the stored root pages
func (app *App) HideModal() {
	app.root.RemovePage("modal")
	app.RestorePreviousFocus()
}

// AddPage is a convenience method to add a page to the root
func (app *App) AddPage(name string, item tview.Primitive, resize, visible bool) {
	app.root.AddPage(name, item, resize, visible)
}

// RemovePage is a convenience method to remove a page from the root
func (app *App) RemovePage(name string) {
	app.root.RemovePage(name)
}

// SwitchToPage is a convenience method to switch pages
func (app *App) SwitchToPage(name string) {
	app.root.SwitchToPage(name)
}

// Stack manages view navigation
type Stack struct {
	items []tview.Primitive
	mu    sync.Mutex
}

func NewStack() *Stack {
	return &Stack{
		items: make([]tview.Primitive, 0),
	}
}

func (s *Stack) Push(item tview.Primitive) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
}

func (s *Stack) Pop() tview.Primitive {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) == 0 {
		return nil
	}
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return item
}

func (s *Stack) Peek() tview.Primitive {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) == 0 {
		return nil
	}
	return s.items[len(s.items)-1]
}

// AddInputCapture adds a new input capture handler
func (app *App) AddInputCapture(handler func(event *tcell.EventKey) *tcell.EventKey) {
	app.focusMutex.Lock()
	defer app.focusMutex.Unlock()
	app.inputCaptures = append(app.inputCaptures, handler)
}

// SetFocusWithMemory sets focus to a component and remembers the previous focus
func (app *App) SetFocusWithMemory(p tview.Primitive) {
	app.focusMutex.Lock()
	if current := app.GetFocus(); current != nil {
		app.focusHistory = append(app.focusHistory, current)
	}
	app.focusMutex.Unlock()
	app.SetFocus(p)
}

// RestorePreviousFocus restores focus to the previous component
func (app *App) RestorePreviousFocus() {
	app.focusMutex.Lock()
	defer app.focusMutex.Unlock()

	if len(app.focusHistory) == 0 {
		return
	}

	previous := app.focusHistory[len(app.focusHistory)-1]
	app.focusHistory = app.focusHistory[:len(app.focusHistory)-1]
	app.SetFocus(previous)
}

// handleGlobalInput processes global keyboard events
func (app *App) handleGlobalInput(event *tcell.EventKey) *tcell.EventKey {
	// Process all input captures in reverse order
	for i := len(app.inputCaptures) - 1; i >= 0; i-- {
		if event = app.inputCaptures[i](event); event == nil {
			return nil
		}
	}

	// Handle global keyboard shortcuts
	switch event.Key() {
	case tcell.KeyEsc:
		// Check if we should restore previous focus
		if app.FocusStack.Peek() != nil {
			app.RestorePreviousFocus()
			return nil
		}
	case tcell.KeyTab:
		// Implement custom tab behavior if needed
		return event
	}

	return event
}

// RunApplication starts the application with error handling
func (app *App) RunApplication() error {
	return app.Run()
}

// QueueUpdateDraw safely queues an update to the UI
func (app *App) QueueUpdateDraw(f func()) {
	app.QueueUpdate(func() {
		f()
		app.Draw()
	})
}

// IsFocused checks if a primitive currently has focus
func (app *App) IsFocused(p tview.Primitive) bool {
	return app.GetFocus() == p
}

// ClearFocusStack clears the focus history
func (app *App) ClearFocusStack() {
	app.focusMutex.Lock()
	defer app.focusMutex.Unlock()
	app.focusHistory = make([]tview.Primitive, 0)
}

// Suspend temporarily suspends the application
func (app *App) Suspend(f func()) {
	app.Suspend(f)
}

// Stop stops the application
func (app *App) Stop() {
	app.Stop()
}

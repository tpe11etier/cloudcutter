package spinner

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Spinner struct {
	*tview.TextView
	frames     []string
	current    int
	done       chan bool
	isLoading  bool
	message    string
	onComplete func()
}

// NewSpinner creates a new spinner component with the specified message
func NewSpinner(message string) *Spinner {
	spinner := &Spinner{
		TextView: tview.NewTextView().
			SetTextAlign(tview.AlignCenter).
			SetTextColor(tcell.ColorBeige).
			SetDynamicColors(true),
		frames:  []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		current: 0,
		done:    make(chan bool),
		message: message,
	}

	return spinner
}

// Start begins the spinner animation
func (s *Spinner) Start(app *tview.Application) {
	if !s.isLoading {
		s.isLoading = true
		go s.animate(app)
	}
}

// Stop halts the spinner animation and triggers the onComplete callback if set
func (s *Spinner) Stop() {
	if s.isLoading {
		s.isLoading = false
		select {
		case s.done <- true:
		default:
		}
		if s.onComplete != nil {
			s.onComplete()
		}
	}
}

// SetMessage updates the spinner's message
func (s *Spinner) SetMessage(message string) {
	s.message = message
}

// SetOnComplete sets a callback function to be called when the spinner stops
func (s *Spinner) SetOnComplete(fn func()) {
	s.onComplete = fn
}

// IsLoading returns whether the spinner is currently active
func (s *Spinner) IsLoading() bool {
	return s.isLoading
}

func (s *Spinner) animate(app *tview.Application) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			app.QueueUpdateDraw(func() {
				s.SetText(fmt.Sprintf("\n[yellow]%s[white] %s", s.message, s.frames[s.current]))
			})
			s.current = (s.current + 1) % len(s.frames)
		}
	}
}

// CreateSpinnerModal creates a modal containing the spinner
func CreateSpinnerModal(spinner *Spinner) *tview.Grid {
	return tview.NewGrid().
		SetColumns(0, 40, 0).
		SetRows(0, 5, 0).
		SetBorders(false).
		AddItem(spinner, 1, 1, 1, 1, 0, 0, false)
}

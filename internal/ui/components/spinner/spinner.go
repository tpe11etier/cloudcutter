package spinner

import (
	"context"
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

func (s *Spinner) Start(app *tview.Application) {
	if !s.isLoading {
		s.isLoading = true
		go s.animate(app)
	}
}

func (s *Spinner) StartWithContext(ctx context.Context, app *tview.Application) {
	if !s.isLoading {
		s.isLoading = true
		go s.animateWithContext(ctx, app)
	}
}

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

func (s *Spinner) SetMessage(message string) {
	s.message = message
}

func (s *Spinner) SetOnComplete(fn func()) {
	s.onComplete = fn
}

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

func (s *Spinner) animateWithContext(ctx context.Context, app *tview.Application) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.Stop()
			return
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

func CreateSpinnerModal(spinner *Spinner) tview.Primitive {
	// Create a simple frame for the spinner with no background
	frame := tview.NewFrame(spinner).
		SetBorders(0, 0, 0, 0, 0, 0)

	// Use a Grid to center the spinner
	grid := tview.NewGrid().
		SetRows(0, 3, 0).     // 3 rows height for spinner
		SetColumns(0, 50, 0). // 50 columns width for spinner
		AddItem(frame, 1, 1, 1, 1, 0, 0, true)

	// Input capture at the grid level
	grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			return event
		}
		return nil
	})

	return grid
}

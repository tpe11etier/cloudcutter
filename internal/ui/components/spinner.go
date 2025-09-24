package components

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Spinner struct {
	*tview.TextView
	frames  []string
	current int
	stop    chan struct{}
}

func NewSpinner() *Spinner {
	s := &Spinner{
		TextView: tview.NewTextView().
			SetTextAlign(tview.AlignCenter).
			SetDynamicColors(true),
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		stop:   make(chan struct{}),
	}

	s.SetBorder(true).
		SetBorderColor(tcell.ColorTeal).
		SetTitle(" Loading ").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorTeal)

	return s
}

func (s *Spinner) SetMessage(msg string) {
	go s.animate(msg)
}

func (s *Spinner) animate(msg string) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.SetText(fmt.Sprintf("%s %s", s.frames[s.current], msg))
			s.current = (s.current + 1) % len(s.frames)
		}
	}
}

func (s *Spinner) Stop() {
	close(s.stop)
}

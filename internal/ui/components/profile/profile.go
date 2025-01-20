package profile

import (
	"context"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/statusbar"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Manager interface {
	Pages() *tview.Pages
}

type Selector struct {
	*tview.List
	onSelect  func(profile string)
	onCancel  func()
	ph        *Handler
	statusBar *statusbar.StatusBar
	manager   Manager
}

func NewSelector(ph *Handler, onSelect func(profile string), onCancel func(), statusBar *statusbar.StatusBar, manager Manager) *Selector {
	selector := &Selector{
		List:      tview.NewList().ShowSecondaryText(false),
		onSelect:  onSelect,
		onCancel:  onCancel,
		ph:        ph,
		statusBar: statusBar,
		manager:   manager,
	}

	selector.
		SetSelectedStyle(tcell.StyleDefault.
			Foreground(tcell.ColorLightYellow).
			Background(tcell.ColorDarkCyan)).
		SetBorder(true).
		SetTitle(" Select Environment ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorMediumTurquoise)

	// Add items without direct functions
	selector.AddItem("Development", "", 0, nil)
	selector.AddItem("Production", "", 0, nil)
	selector.AddItem("Local", "", 0, nil)

	// Set selection handler
	selector.SetSelectedFunc(func(index int, name string, secondName string, shortcut rune) {
		switch index {
		case 0:
			selector.switchProfile("opal_dev")
		case 1:
			selector.switchProfile("opal_prod")
		case 2:
			selector.switchProfile("local")
		}
	})

	selector.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			if selector.onCancel != nil {
				selector.onCancel()
			}
			return nil
		}
		return event
	})

	selector.SetCurrentItem(0)
	return selector
}

func (ps *Selector) switchProfile(profile string) {
	ps.statusBar.SetText("Switching profile...")
	ps.ph.SwitchProfile(context.Background(), profile, func(cfg aws.Config, err error) {
		if err != nil {
			ps.statusBar.SetText(err.Error())
			return
		}
		ps.statusBar.SetText("Profile switched successfully")
		ps.onSelect(profile)
	})
}

func (ps *Selector) ShowSelector() (tview.Primitive, error) {
	numEntries := ps.GetItemCount() + 2
	modal := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(ps, 30, 0, true).
			AddItem(nil, 0, 1, false),
			numEntries, 1, true).
		AddItem(nil, 0, 1, false)

	ps.manager.Pages().AddPage("profileSelector", modal, true, true)
	return ps, nil
}

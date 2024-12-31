package profile

import (
	"context"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/statusbar"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Selector struct {
	*tview.List
	onSelect  func(profile string)
	onCancel  func()
	ph        *Handler
	statusBar *statusbar.StatusBar
}

func NewSelector(ph *Handler, onSelect func(profile string), onCancel func(), statusBar *statusbar.StatusBar) *Selector {
	selector := &Selector{
		List:      tview.NewList().ShowSecondaryText(false),
		onSelect:  onSelect,
		onCancel:  onCancel,
		ph:        ph,
		statusBar: statusBar,
	}

	selector.
		SetSelectedStyle(tcell.StyleDefault.
			Foreground(tcell.ColorLightYellow).
			Background(tcell.ColorDarkCyan)).
		SetBorder(true).
		SetTitle(" Select Environment ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorMediumTurquoise)

	selector.AddItem("Development", "", 0, func() {
		selector.switchProfile("opal_dev")
	})
	selector.AddItem("Production", "", 0, func() {
		selector.switchProfile("opal_prod")
	})
	selector.AddItem("Local", "", 0, func() {
		selector.switchProfile("local")
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

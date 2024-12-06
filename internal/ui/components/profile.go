package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ProfileSelector struct {
	*tview.List
	onSelect func(profile string)
	OnCancel func()
}

func NewProfileSelector(onSelect func(profile string), onCancel func()) *ProfileSelector {
	selector := &ProfileSelector{
		List:     tview.NewList().ShowSecondaryText(false),
		onSelect: onSelect,
		OnCancel: onCancel,
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
		onSelect("opal_dev")
	})
	selector.AddItem("Production", "", 0, func() {
		onSelect("opal_prod")
	})

	selector.SetCurrentItem(0)

	return selector
}

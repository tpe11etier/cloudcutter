package profile

import (
	"sort"

	"github.com/tpe11etier/cloudcutter/internal/environments"
	"github.com/tpe11etier/cloudcutter/internal/ui/components/statusbar"
	"github.com/tpe11etier/cloudcutter/internal/ui/style"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Manager interface {
	Pages() *tview.Pages
	Resolver() *environments.Resolver
}

type Selector struct {
	*tview.List
	onSelect  func(profile string)
	onCancel  func()
	ph        *Handler
	statusBar *statusbar.StatusBar
	profiles  []string
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
		SetMainTextColor(style.GruvboxMaterial.Foreground).
		SetSelectedStyle(tcell.StyleDefault.
			Foreground(tcell.ColorLightYellow).
			Background(tcell.ColorDarkCyan)).
		SetBorder(true).
		SetTitle(" Select Environment ").
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(style.GruvboxMaterial.Foreground).
		SetBorderColor(tcell.ColorMediumTurquoise)

	if r := manager.Resolver(); r != nil {
		selector.profiles = r.List()
		sort.Strings(selector.profiles)
	}

	for _, profile := range selector.profiles {
		selector.AddItem(profile, "", 0, nil)
	}

	selector.SetSelectedFunc(func(index int, name string, secondName string, shortcut rune) {
		// All dispatch goes through the manager's onSelect callback.
		// Special-casing per profile name is gone in phase 4.
		selector.onSelect(name)
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

func (ps *Selector) ShowSelector() (tview.Primitive, error) {
	numEntries := ps.GetItemCount() + 2
	modal := tview.NewGrid().
		SetRows(0, numEntries, 0).
		SetColumns(0, 30, 0).
		AddItem(ps, 1, 1, 1, 1, 0, 0, true)
	ps.manager.Pages().AddPage("profileSelector", modal, true, true)
	return ps, nil
}

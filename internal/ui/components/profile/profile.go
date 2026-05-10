package profile

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tpe11etier/cloudcutter/internal/auth"
	"github.com/tpe11etier/cloudcutter/internal/environments"
	"github.com/tpe11etier/cloudcutter/internal/ui/components/statusbar"
	"github.com/tpe11etier/cloudcutter/internal/ui/style"
	"gopkg.in/ini.v1"

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

func (ps *Selector) discoverProfiles() []string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	profileMap := make(map[string]struct{})

	// Load profiles from credentials file
	credFile := filepath.Join(homedir, ".aws", "credentials")
	if readCfg, err := ini.Load(credFile); err == nil {
		for _, section := range readCfg.Sections() {
			name := section.Name()
			if name != ini.DefaultSection {
				name = strings.TrimPrefix(name, "profile")
				profileMap[name] = struct{}{}
			}
		}
	}

	// add local profile to connect to local Docker instance
	profileMap["local"] = struct{}{}
	profileMap[auth.DragosProfile] = struct{}{}
	// Convert map to sorted slice
	profiles := make([]string, 0, len(profileMap))
	for profile := range profileMap {
		profiles = append(profiles, profile)
	}
	sort.Strings(profiles)

	return profiles
}

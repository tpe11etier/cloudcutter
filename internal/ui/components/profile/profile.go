package profile

import (
	"context"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/statusbar"
	"gopkg.in/ini.v1"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
		SetSelectedStyle(tcell.StyleDefault.
			Foreground(tcell.ColorLightYellow).
			Background(tcell.ColorDarkCyan)).
		SetBorder(true).
		SetTitle(" Select Environment ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorMediumTurquoise)

	// Discover available profiles
	selector.profiles = selector.discoverProfiles()

	// Add all discovered profiles
	for _, profile := range selector.profiles {
		selector.AddItem(profile, "", 0, nil)
	}

	// Set selection handler to use profile name directly
	selector.SetSelectedFunc(func(index int, name string, secondName string, shortcut rune) {
		selector.switchProfile(name)
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

	// Load profiles from config file
	configFile := filepath.Join(homedir, ".aws", "config")
	if cfgFile, err := ini.Load(configFile); err == nil {
		for _, section := range cfgFile.Sections() {
			name := section.Name()
			// Config file uses "profile prefix" except for default
			if name != ini.DefaultSection {
				name = strings.TrimPrefix(name, "profile ")
				profileMap[name] = struct{}{}
			}
		}
	}

	// Add default profile if either file exists
	if _, err := os.Stat(credFile); err == nil {
		profileMap["default"] = struct{}{}
	}
	if _, err := os.Stat(configFile); err == nil {
		profileMap["default"] = struct{}{}
	}

	// add local profile to connect to local Docker instance
	profileMap["local"] = struct{}{}
	// Convert map to sorted slice
	profiles := make([]string, 0, len(profileMap))
	for profile := range profileMap {
		profiles = append(profiles, profile)
	}
	sort.Strings(profiles)

	return profiles
}

package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type RegionSelector struct {
	*tview.List
	onSelect func(string)
	onCancel func()
}

func NewRegionSelector(onSelect func(string), onCancel func()) *RegionSelector {
	regions := []string{
		"us-east-1",      // US East (N. Virginia)
		"us-east-2",      // US East (Ohio)
		"us-west-1",      // US West (N. California)
		"us-west-2",      // US West (Oregon)
		"eu-west-1",      // Europe (Ireland)
		"eu-central-1",   // Europe (Frankfurt)
		"ap-northeast-1", // Asia Pacific (Tokyo)
		"ap-southeast-1", // Asia Pacific (Singapore)
		"ap-southeast-2", // Asia Pacific (Sydney)
	}

	selector := &RegionSelector{
		List:     tview.NewList().ShowSecondaryText(false),
		onSelect: onSelect,
		onCancel: onCancel,
	}

	selector.SetBorder(true)
	selector.SetTitle(" Select Region ")
	selector.SetTitleAlign(tview.AlignLeft)
	selector.SetBorderColor(tcell.ColorMediumTurquoise)

	// Add regions to the list
	for _, region := range regions {
		selector.AddItem(region, "", 0, nil)
	}

	// Handle selection
	selector.SetSelectedFunc(func(index int, name string, secondName string, shortcut rune) {
		if selector.onSelect != nil {
			selector.onSelect(name)
		}
	})

	// Handle ESC key
	selector.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			if selector.onCancel != nil {
				selector.onCancel()
			}
			return nil
		}
		return event
	})

	return selector
}

func (rs *RegionSelector) GetItemCount() int {
	return rs.List.GetItemCount()
}

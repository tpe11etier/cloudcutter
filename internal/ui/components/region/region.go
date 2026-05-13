package region

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpe11etier/cloudcutter/internal/ui/style"
)

type ManagerInterface interface {
	Pages() *tview.Pages
	App() *tview.Application
	ActiveView() tview.Primitive
	UpdateRegion(region string)
}

type RegionSelector struct {
	*tview.List
	onSelect func(string)
	onCancel func()
	manager  ManagerInterface
}

func NewRegionSelector(onSelect func(string), onCancel func(), manager ManagerInterface) *RegionSelector {
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
		manager:  manager,
	}

	selector.SetBorder(true)
	selector.SetTitle(" Select Region ")
	selector.SetTitleAlign(tview.AlignLeft)
	selector.SetBorderColor(style.Active.Border)

	for _, region := range regions {
		selector.AddItem(region, "", 0, nil)
	}

	selector.SetSelectedFunc(func(index int, name string, secondName string, shortcut rune) {
		if selector.onSelect != nil {
			go func() {
				selector.manager.UpdateRegion(name)
			}()
		}
		selector.HideRegionSelector()
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

	return selector
}

func (rs *RegionSelector) GetItemCount() int {
	return rs.List.GetItemCount()
}

func (rs *RegionSelector) ShowRegionSelector() (tview.Primitive, error) {
	numEntries := rs.GetItemCount() + 2
	modal := tview.NewGrid().
		SetRows(0, numEntries, 0).
		SetColumns(0, 30, 0).
		AddItem(rs, 1, 1, 1, 1, 0, 0, true)
	rs.manager.Pages().AddPage("regionSelector", modal, true, true)
	return rs, nil
}

func (rs *RegionSelector) HideRegionSelector() {
	rs.manager.Pages().RemovePage("regionSelector")
	if rs.manager.ActiveView() != nil {
		rs.manager.App().SetFocus(rs.manager.ActiveView())
	}
}

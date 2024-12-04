package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type LeftPanel struct {
	*tview.List
	onItemSelected func(index int, name string)
}

func NewLeftPanel() *LeftPanel {
	panel := &LeftPanel{
		List: tview.NewList().
			ShowSecondaryText(false).
			SetMainTextColor(tcell.ColorWhite).
			SetSelectedTextColor(tcell.ColorBlack).
			SetSelectedBackgroundColor(tcell.ColorTeal),
	}

	panel.SetBorder(true).
		SetTitle(" Navigation ").
		SetTitleAlign(tview.AlignLeft).
		SetTitleColor(tcell.ColorTeal)

	return panel
}

func (p *LeftPanel) SetItemSelectedFunc(handler func(index int, name string)) {
	p.onItemSelected = handler
	p.List.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if p.onItemSelected != nil {
			p.onItemSelected(index, mainText)
		}
	})
}

func (p *LeftPanel) AddItems(items []string, selectedFunc func(index int, name string)) {
	for _, item := range items {
		p.AddItem(item, "", 0, nil)
	}
	p.SetItemSelectedFunc(selectedFunc)
}

func (p *LeftPanel) ShowServicesList() {
	p.Clear()
	services := []string{"EC2", "S3", "DynamoDB", "ECR"}
	p.AddItems(services, p.onItemSelected)
}

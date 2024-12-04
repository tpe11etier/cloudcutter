package components

import (
	"github.com/rivo/tview"
)

type Modal struct {
	*tview.Flex
	pages  *tview.Pages
	onHide func()
	modals map[string]tview.Primitive
}

func NewModal(pages *tview.Pages) *Modal {
	return &Modal{
		Flex:   tview.NewFlex(),
		pages:  pages,
		modals: make(map[string]tview.Primitive),
	}
}

func (m *Modal) ShowModal(content tview.Primitive, id string, width, height int) {
	frame := tview.NewFrame(content).
		SetBorders(0, 0, 0, 0, 0, 0)

	// Create flex container for centering
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	m.modals[id] = flex
	m.pages.AddPage(id, flex, true, true)
}

func (m *Modal) HideModal(id string) {
	if _, exists := m.modals[id]; exists {
		m.pages.RemovePage(id)
		delete(m.modals, id)
		if m.onHide != nil {
			m.onHide()
		}
	}
}

func (m *Modal) SetHideFunc(handler func()) {
	m.onHide = handler
}

func (m *Modal) GetInputField() *tview.InputField {
	inputField, ok := m.modals["view-switch"].(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.InputField)
	if !ok {
		return nil
	}
	return inputField
}

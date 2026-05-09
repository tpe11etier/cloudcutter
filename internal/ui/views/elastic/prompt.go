package elastic

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpe11etier/cloudcutter/internal/ui/components"
	"github.com/tpe11etier/cloudcutter/internal/ui/components/types"
)

func (v *View) showFilterPrompt(source tview.Primitive) {
	previousFocus := source

	switch source {
	case v.components.fieldList:
		v.components.filterPrompt.Configure(components.PromptOptions{
			Title:      " Filter Fields ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnChanged: func(text string) {
				// Also persist on every keystroke so rebuildFieldList (which
				// fires on background refresh / new results) re-applies the
				// in-progress filter instead of silently reverting to all
				// fields while the user is still typing.
				v.state.ui.fieldListFilter = text
				v.filterFieldList(text)
			},
			OnDone: func(text string) {
				v.state.ui.fieldListFilter = text
				v.filterFieldList(text)
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.state.ui.fieldListFilter = ""
				v.filterFieldList("")
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
		})

	case v.components.localFilterInput:
		v.components.filterPrompt.Configure(components.PromptOptions{
			Title:      " Filter Results ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnChanged: func(text string) {
				v.displayFilteredResults(text)
			},
			OnDone: func(text string) {
				v.displayFilteredResults(text)
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.displayFilteredResults("")
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
		})
	case v.components.resultsTable:
		v.components.filterPrompt.Configure(components.PromptOptions{
			Title:      " Filter Results ",
			Label:      " >_ ",
			LabelColor: tcell.ColorMediumTurquoise,
			OnChanged: func(text string) {
				v.components.localFilterInput.SetText(v.components.filterPrompt.GetText())
			},
			OnDone: func(text string) {
				v.displayFilteredResults(text)
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
			OnCancel: func() {
				v.displayFilteredResults("")
				v.manager.HideFilterPrompt()
				v.manager.SetFocus(previousFocus)
			},
		})
	}

	v.components.filterPrompt.SetText("")
	promptLayout := v.components.filterPrompt.Layout()
	v.manager.Pages().AddPage(types.ModalFilter, promptLayout, true, true)
	v.manager.App().SetFocus(v.components.filterPrompt.InputField)
}


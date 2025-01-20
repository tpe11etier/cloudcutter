package elastic

import (
	"encoding/json"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/style"
)

func (v *View) showJSONModal(entry *DocEntry) {
	data := map[string]any{
		"_id":    entry.ID,
		"_index": entry.Index,
		"_type":  entry.Type,
	}
	if entry.Score != nil {
		data["_score"] = *entry.Score
	}
	if entry.Version != nil {
		data["_version"] = *entry.Version
	}
	data["_source"] = entry.data

	prettyJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		v.manager.UpdateStatusBar(fmt.Sprintf("Error formatting JSON: %v", err))
		return
	}

	coloredJSON := style.ColorizeJSON(string(prettyJSON))

	textView := tview.NewTextView()
	textView.SetTitle("'y' to copy | 'Esc' to close").
		SetTitleColor(style.GruvboxMaterial.Yellow)
	textView.SetText(coloredJSON).
		SetDynamicColors(true).
		SetRegions(true).
		SetScrollable(true).
		SetWrap(false)
	textView.SetBorder(true).SetBorderColor(tcell.ColorMediumTurquoise)

	frame := tview.NewFrame(textView).
		SetBorders(0, 0, 0, 0, 0, 0)

	grid := tview.NewGrid().
		SetColumns(0, 150, 0).
		SetRows(0, 40, 0)

	grid.AddItem(frame, 1, 1, 1, 1, 0, 0, true)

	jsonStr := string(prettyJSON)

	grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'y' {
				if err := clipboard.WriteAll(jsonStr); err != nil {
					v.manager.UpdateStatusBar("Failed to copy JSON to clipboard")
				} else {
					v.manager.UpdateStatusBar("JSON copied to clipboard")
				}
				return nil
			}
		}
		return event
	})

	pages := v.manager.Pages()
	if pages.HasPage(manager.ModalJSON) {
		pages.RemovePage(manager.ModalJSON)
	}
	pages.AddPage(manager.ModalJSON, grid, true, true)

	v.manager.App().SetFocus(textView)
}

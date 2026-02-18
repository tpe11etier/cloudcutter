package elastic

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/manager"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/style"
)

// EnhancedJSONModal provides vim-like navigation and dynamic resizing for JSON documents
type EnhancedJSONModal struct {
	*tview.Box

	// UI Components
	textView  *tview.TextView
	statusBar *tview.TextView
	container *tview.Flex

	// Modal state
	jsonContent string
	lines       []string
	mode        ModalMode

	// Navigation state
	cursorX int
	cursorY int
	scrollX int
	scrollY int

	// Selection state
	selectStart struct{ x, y int }
	selectEnd   struct{ x, y int }
	isSelecting bool

	// Search state
	searchTerm    string
	searchMatches []struct{ line, start, end int }
	currentMatch  int
	searchBuffer  string
	isSearching   bool

	// Size state
	width     int
	height    int
	minWidth  int
	minHeight int
	maxWidth  int
	maxHeight int

	// Callbacks
	onClose  func()
	onResize func(width, height int)

	// View reference
	view *View
}

type ModalMode int

const (
	ModeNormal ModalMode = iota
	ModeVisual
	ModeCommand
	ModeSearch
	ModeResize
)

// NewEnhancedJSONModal creates a new enhanced JSON modal
func NewEnhancedJSONModal(view *View) *EnhancedJSONModal {
	modal := &EnhancedJSONModal{
		Box:       tview.NewBox(),
		view:      view,
		mode:      ModeNormal,
		width:     200,
		height:    50,
		minWidth:  40,
		minHeight: 10,
		maxWidth:  240,
		maxHeight: 80, // Much larger max height (nearly full screen)
	}

	modal.setupUI()
	return modal
}

// setupUI initializes the UI components
func (m *EnhancedJSONModal) setupUI() {
	// Main text view for JSON content
	m.textView = tview.NewTextView()
	m.textView.SetBorder(true)
	m.textView.SetTitle(" JSON Document - Vim Mode ")
	m.textView.SetTitleColor(style.GruvboxMaterial.Foreground)
	m.textView.SetBorderColor(tcell.ColorMediumTurquoise)
	m.textView.SetTextColor(style.GruvboxMaterial.Foreground)
	m.textView.SetDynamicColors(true)
	m.textView.SetRegions(true)
	m.textView.SetScrollable(true)
	m.textView.SetWrap(false)

	// Status bar showing current mode and shortcuts
	m.statusBar = tview.NewTextView()
	m.statusBar.SetBorder(false)
	m.statusBar.SetBackgroundColor(tcell.ColorDarkSlateGray)
	m.statusBar.SetTextColor(tcell.ColorWhite)
	m.statusBar.SetDynamicColors(true)
	m.updateStatusBar()

	// Container layout
	m.container = tview.NewFlex()
	m.container.SetDirection(tview.FlexRow)
	m.container.AddItem(m.textView, 0, 1, true)
	m.container.AddItem(m.statusBar, 1, 0, false)

	// Set up input capture for vim-like controls
	m.container.SetInputCapture(m.handleInput)
}

// ShowJSON displays JSON content in the enhanced modal
func (m *EnhancedJSONModal) ShowJSON(entry *DocEntry) {
	// Prepare JSON data
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
		m.view.manager.UpdateStatusBar(fmt.Sprintf("Error formatting JSON: %v", err))
		return
	}

	m.jsonContent = string(prettyJSON)
	m.lines = strings.Split(m.jsonContent, "\n")
	m.updateDisplay()

	// Create modal grid with dynamic sizing
	m.showModal()
}

// showModal displays the modal with current size
func (m *EnhancedJSONModal) showModal() {
	// Create grid with dynamic sizing (back to working layout)
	grid := tview.NewGrid().
		SetColumns(0, m.width, 0).
		SetRows(0, m.height, 0)

	grid.AddItem(m.container, 1, 1, 1, 1, 0, 0, true)

	pages := m.view.manager.Pages()
	if pages.HasPage(manager.ModalJSON) {
		pages.RemovePage(manager.ModalJSON)
	}

	// Back to proper modal display
	pages.AddPage(manager.ModalJSON, grid, true, true)

	// LAST RESORT: Override at APPLICATION level - intercept EVERYTHING
	app := m.view.manager.App()
	originalAppCapture := app.GetInputCapture()

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// ONLY when our modal exists, hijack ALL escape events
		if pages.HasPage(manager.ModalJSON) && event.Key() == tcell.KeyEscape {
			// Handle escape ourselves
			switch m.mode {
			case ModeVisual:
				m.exitVisualMode()
			case ModeNormal:
				// Do nothing
			default:
				m.mode = ModeNormal
				m.updateStatusBar()
			}
			// NEVER let escape through - consume it completely
			return nil
		}

		// For all other events, use original handler
		if originalAppCapture != nil {
			return originalAppCapture(event)
		}
		return event
	})

	// Store cleanup function
	m.onClose = func() {
		app.SetInputCapture(originalAppCapture)
	}

	app.SetFocus(m.container)
}

// handleInput processes vim-like keyboard input
func (m *EnhancedJSONModal) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch m.mode {
	case ModeNormal:
		return m.handleNormalMode(event)
	case ModeVisual:
		return m.handleVisualMode(event)
	case ModeCommand:
		return m.handleCommandMode(event)
	case ModeSearch:
		return m.handleSearchMode(event)
	case ModeResize:
		return m.handleResizeInput(event)
	}
	return event
}

// handleNormalMode processes normal mode vim commands
func (m *EnhancedJSONModal) handleNormalMode(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		// In normal mode, escape does nothing (consume the event)
		return nil
	case tcell.KeyRune:
		switch event.Rune() {
		// Movement
		case 'h':
			m.moveCursor(-1, 0)
		case 'j':
			m.moveCursor(0, 1)
		case 'k':
			m.moveCursor(0, -1)
		case 'l':
			m.moveCursor(1, 0)
		case 'w':
			m.moveWordForward()
		case 'b':
			m.moveWordBackward()
		case '0':
			m.cursorX = 0
		case '$':
			if m.cursorY < len(m.lines) {
				m.cursorX = len(m.lines[m.cursorY])
			}
		case 'g':
			// Handle gg (go to top)
			m.cursorY = 0
			m.cursorX = 0
		case 'G':
			// Go to bottom
			m.cursorY = len(m.lines) - 1
			m.cursorX = 0

		// Actions
		case 'v':
			m.enterVisualMode()
		case 'y':
			m.copyCurrentLine()
		case 'Y':
			m.copyEntireDocument()
		case '/':
			m.enterSearchMode()
		case 'n':
			m.nextSearchMatch()
		case 'N':
			m.previousSearchMatch()
		case 'r':
			m.enterResizeMode()
		case 'q':
			m.closeModal()

		// JSON-specific actions
		case 'c':
			m.copyJSONValue()
		case 'f':
			m.formatAndCopy()
		}
		m.updateDisplay()
	}
	return nil
}

// handleVisualMode processes visual selection mode
func (m *EnhancedJSONModal) handleVisualMode(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		m.exitVisualMode()
		return nil
	case tcell.KeyRune:
		switch event.Rune() {
		// Movement (extends selection)
		case 'h':
			m.moveCursor(-1, 0)
			m.updateSelection()
		case 'j':
			m.moveCursor(0, 1)
			m.updateSelection()
		case 'k':
			m.moveCursor(0, -1)
			m.updateSelection()
		case 'l':
			m.moveCursor(1, 0)
			m.updateSelection()
		case 'w':
			m.moveWordForward()
			m.updateSelection()
		case 'b':
			m.moveWordBackward()
			m.updateSelection()

		// Actions
		case 'y':
			m.copySelection()
			m.exitVisualMode()
		case 'd':
			// Could implement delete if needed
			m.exitVisualMode()
		}
		m.updateDisplay()
	}
	return nil
}

// handleCommandMode processes command mode input (TODO: implement commands like :q, :w, etc.)
func (m *EnhancedJSONModal) handleCommandMode(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		m.mode = ModeNormal
		m.updateStatusBar()
		return nil
	}
	// TODO: Implement command processing
	return event
}

// handleSearchMode processes search mode input
func (m *EnhancedJSONModal) handleSearchMode(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		m.exitSearchMode()
		return nil
	case tcell.KeyEnter:
		// Execute search
		if m.searchBuffer != "" {
			m.executeSearch(m.searchBuffer)
		}
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		// Remove character from search buffer
		if len(m.searchBuffer) > 0 {
			m.searchBuffer = m.searchBuffer[:len(m.searchBuffer)-1]
			m.updateStatusBar()
		}
		return nil
	case tcell.KeyRune:
		// Add character to search buffer
		m.searchBuffer += string(event.Rune())
		m.updateStatusBar()
		return nil
	}
	return event
}

// handleResizeInput processes resize mode input
func (m *EnhancedJSONModal) handleResizeInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		m.exitResizeMode()
		return nil
	case tcell.KeyRune:
		switch event.Rune() {
		case 'h': // Decrease width
			if m.width > m.minWidth {
				m.width -= 5
				m.resizeModal()
			}
		case 'l': // Increase width
			// Simple bounds check - don't exceed reasonable terminal width
			if m.width < 240 {
				m.width += 5
				m.resizeModal()
			}
		case 'j': // Decrease height
			if m.height > m.minHeight {
				m.height -= 2
				m.resizeModal()
			}
		case 'k': // Increase height
			// Simple bounds check - don't exceed reasonable terminal height
			if m.height < 60 { // Most terminals can handle up to ~60 lines
				m.height += 2
				m.resizeModal()
			}
		case 'r':
			m.resetSize()
		case 'q':
			m.exitResizeMode()
		}
	}
	return nil
}

// Movement and navigation methods
func (m *EnhancedJSONModal) moveCursor(dx, dy int) {
	m.cursorX += dx
	m.cursorY += dy

	// Bounds checking
	if m.cursorY < 0 {
		m.cursorY = 0
	}
	if m.cursorY >= len(m.lines) {
		m.cursorY = len(m.lines) - 1
	}

	if m.cursorY < len(m.lines) {
		maxX := len(m.lines[m.cursorY])
		if m.cursorX < 0 {
			m.cursorX = 0
		}
		if m.cursorX > maxX {
			m.cursorX = maxX
		}
	}
}

func (m *EnhancedJSONModal) moveWordForward() {
	if m.cursorY >= len(m.lines) {
		return
	}

	line := m.lines[m.cursorY]
	for m.cursorX < len(line) && !unicode.IsSpace(rune(line[m.cursorX])) {
		m.cursorX++
	}
	for m.cursorX < len(line) && unicode.IsSpace(rune(line[m.cursorX])) {
		m.cursorX++
	}
}

func (m *EnhancedJSONModal) moveWordBackward() {
	if m.cursorY >= len(m.lines) {
		return
	}

	line := m.lines[m.cursorY]
	if m.cursorX > 0 {
		m.cursorX--
	}
	for m.cursorX > 0 && unicode.IsSpace(rune(line[m.cursorX])) {
		m.cursorX--
	}
	for m.cursorX > 0 && !unicode.IsSpace(rune(line[m.cursorX-1])) {
		m.cursorX--
	}
}

// Mode management
func (m *EnhancedJSONModal) enterVisualMode() {
	m.mode = ModeVisual
	m.isSelecting = true
	m.selectStart.x = m.cursorX
	m.selectStart.y = m.cursorY
	m.selectEnd = m.selectStart
	m.updateStatusBar()
}

func (m *EnhancedJSONModal) exitVisualMode() {
	m.mode = ModeNormal
	m.isSelecting = false
	m.updateStatusBar()
	m.updateDisplay() // Force display update when exiting visual mode
}

func (m *EnhancedJSONModal) enterSearchMode() {
	m.mode = ModeSearch
	m.searchBuffer = ""
	m.isSearching = true
	m.updateStatusBar()
}

func (m *EnhancedJSONModal) enterResizeMode() {
	m.mode = ModeResize
	m.updateStatusBar()
}

func (m *EnhancedJSONModal) exitResizeMode() {
	m.mode = ModeNormal
	m.updateStatusBar()
}

func (m *EnhancedJSONModal) exitSearchMode() {
	m.mode = ModeNormal
	m.isSearching = false
	m.searchBuffer = ""
	m.updateStatusBar()
	m.updateDisplay()
}

// executeSearch performs the search and highlights matches
func (m *EnhancedJSONModal) executeSearch(searchTerm string) {
	m.searchTerm = searchTerm
	m.searchMatches = nil
	m.currentMatch = 0

	// Case-insensitive search
	searchLower := strings.ToLower(searchTerm)

	// Find all matches
	for lineNum, line := range m.lines {
		lineLower := strings.ToLower(line)
		start := 0
		for {
			pos := strings.Index(lineLower[start:], searchLower)
			if pos == -1 {
				break
			}
			actualPos := start + pos
			m.searchMatches = append(m.searchMatches, struct{ line, start, end int }{
				line:  lineNum,
				start: actualPos,
				end:   actualPos + len(searchTerm),
			})
			start = actualPos + 1
		}
	}

	if len(m.searchMatches) > 0 {
		// Jump to first match
		firstMatch := m.searchMatches[0]
		m.cursorY = firstMatch.line
		m.cursorX = firstMatch.start
		m.view.manager.UpdateStatusBar(fmt.Sprintf("Found %d matches", len(m.searchMatches)))
	} else {
		m.view.manager.UpdateStatusBar(fmt.Sprintf("Pattern not found: %s", searchTerm))
	}

	// Exit search mode and return to normal mode
	m.mode = ModeNormal
	m.isSearching = false
	m.searchBuffer = ""
	m.updateDisplay()
}

// nextSearchMatch navigates to the next search match
func (m *EnhancedJSONModal) nextSearchMatch() {
	if len(m.searchMatches) == 0 {
		m.view.manager.UpdateStatusBar("No search matches")
		return
	}

	m.currentMatch = (m.currentMatch + 1) % len(m.searchMatches)
	match := m.searchMatches[m.currentMatch]
	m.cursorY = match.line
	m.cursorX = match.start
	m.view.manager.UpdateStatusBar(fmt.Sprintf("Match %d of %d: %s", m.currentMatch+1, len(m.searchMatches), m.searchTerm))
}

// previousSearchMatch navigates to the previous search match
func (m *EnhancedJSONModal) previousSearchMatch() {
	if len(m.searchMatches) == 0 {
		m.view.manager.UpdateStatusBar("No search matches")
		return
	}

	m.currentMatch = (m.currentMatch - 1 + len(m.searchMatches)) % len(m.searchMatches)
	match := m.searchMatches[m.currentMatch]
	m.cursorY = match.line
	m.cursorX = match.start
	m.view.manager.UpdateStatusBar(fmt.Sprintf("Match %d of %d: %s", m.currentMatch+1, len(m.searchMatches), m.searchTerm))
}

// Copy operations
func (m *EnhancedJSONModal) copyCurrentLine() {
	if m.cursorY < len(m.lines) {
		if err := clipboard.WriteAll(m.lines[m.cursorY]); err != nil {
			m.view.manager.UpdateStatusBar("Failed to copy line to clipboard")
		} else {
			m.view.manager.UpdateStatusBar("Line copied to clipboard")
		}
	}
}

func (m *EnhancedJSONModal) copyEntireDocument() {
	if err := clipboard.WriteAll(m.jsonContent); err != nil {
		m.view.manager.UpdateStatusBar("Failed to copy JSON to clipboard")
	} else {
		m.view.manager.UpdateStatusBar("JSON copied to clipboard")
	}
}

func (m *EnhancedJSONModal) copySelection() {
	if !m.isSelecting {
		return
	}

	// Get selected text
	selectedText := m.getSelectedText()
	if selectedText != "" {
		if err := clipboard.WriteAll(selectedText); err != nil {
			m.view.manager.UpdateStatusBar("Failed to copy selection to clipboard")
		} else {
			m.view.manager.UpdateStatusBar("Selection copied to clipboard")
		}
	}
}

func (m *EnhancedJSONModal) copyJSONValue() {
	// Try to extract JSON value at cursor position
	line := m.lines[m.cursorY]
	value := m.extractJSONValueAt(line, m.cursorX)
	if value != "" {
		if err := clipboard.WriteAll(value); err != nil {
			m.view.manager.UpdateStatusBar("Failed to copy value to clipboard")
		} else {
			m.view.manager.UpdateStatusBar(fmt.Sprintf("Value copied: %s", value))
		}
	}
}

func (m *EnhancedJSONModal) formatAndCopy() {
	// Copy with custom formatting options
	// TODO: Implement formatting options
	m.copyEntireDocument()
}

// Helper methods
func (m *EnhancedJSONModal) updateSelection() {
	if m.isSelecting {
		m.selectEnd.x = m.cursorX
		m.selectEnd.y = m.cursorY
	}
}

func (m *EnhancedJSONModal) getSelectedText() string {
	if !m.isSelecting {
		return ""
	}

	startY, endY := m.selectStart.y, m.selectEnd.y
	startX, endX := m.selectStart.x, m.selectEnd.x

	// Ensure start comes before end
	if startY > endY || (startY == endY && startX > endX) {
		startY, endY = endY, startY
		startX, endX = endX, startX
	}

	if startY == endY {
		// Single line selection
		if startY < len(m.lines) {
			line := m.lines[startY]
			if endX > len(line) {
				endX = len(line)
			}
			return line[startX:endX]
		}
	} else {
		// Multi-line selection
		var result strings.Builder
		for y := startY; y <= endY && y < len(m.lines); y++ {
			line := m.lines[y]
			if y == startY {
				result.WriteString(line[startX:])
			} else if y == endY {
				if endX <= len(line) {
					result.WriteString(line[:endX])
				}
			} else {
				result.WriteString(line)
			}
			if y < endY {
				result.WriteString("\n")
			}
		}
		return result.String()
	}

	return ""
}

func (m *EnhancedJSONModal) extractJSONValueAt(line string, x int) string {
	// Simple JSON value extraction - finds quoted strings or numbers
	if x >= len(line) {
		return ""
	}

	// Find start and end of value
	start, end := x, x

	// If we're on a quote, extract the quoted string
	if line[x] == '"' {
		start = x
		end = x + 1
		for end < len(line) && line[end] != '"' {
			if line[end] == '\\' {
				end++ // Skip escaped character
			}
			end++
		}
		if end < len(line) {
			end++ // Include closing quote
		}
		return line[start:end]
	}

	// Otherwise, extract alphanumeric value
	for start > 0 && (unicode.IsLetter(rune(line[start-1])) || unicode.IsDigit(rune(line[start-1])) || line[start-1] == '.' || line[start-1] == '-') {
		start--
	}
	for end < len(line) && (unicode.IsLetter(rune(line[end])) || unicode.IsDigit(rune(line[end])) || line[end] == '.' || line[end] == '-') {
		end++
	}

	if end > start {
		return line[start:end]
	}

	return ""
}

func (m *EnhancedJSONModal) resizeModal() {
	// Re-show modal with new size
	m.showModal()
}

func (m *EnhancedJSONModal) resetSize() {
	m.width = 200 // Match new default size
	m.height = 50 // Match new default size
	m.resizeModal()
}

func (m *EnhancedJSONModal) updateDisplay() {
	// Build display text with cursor and visual selection highlighting
	var displayText strings.Builder

	if m.isSelecting && m.mode == ModeVisual {
		// Apply visual selection highlighting with cursor
		displayText.WriteString(m.buildTextWithSelectionAndCursor())
	} else {
		// Regular colored JSON with cursor
		displayText.WriteString(m.buildTextWithCursor())
	}

	m.textView.SetText(displayText.String())

	// Ensure cursor is visible by scrolling the textView
	m.scrollToShowCursor()

	m.updateStatusBar()
}

// buildTextWithCursor creates text with cursor highlighting (normal mode)
func (m *EnhancedJSONModal) buildTextWithCursor() string {
	var result strings.Builder

	for i, line := range m.lines {
		if i == m.cursorY {
			// Current line - highlight cursor position
			if m.cursorX < len(line) {
				// Cursor is on a character
				result.WriteString(style.ColorizeJSON(line[:m.cursorX]))
				result.WriteString("[white:blue]") // Cursor highlight - white text on blue background
				result.WriteString(string(line[m.cursorX]))
				result.WriteString("[-:-]") // Reset colors
				result.WriteString(style.ColorizeJSON(line[m.cursorX+1:]))
			} else {
				// Cursor is at end of line - show as space
				result.WriteString(style.ColorizeJSON(line))
				result.WriteString("[white:blue] [-:-]") // Highlighted space at end
			}
		} else {
			// Other lines - normal coloring
			result.WriteString(style.ColorizeJSON(line))
		}

		if i < len(m.lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// buildTextWithSelectionAndCursor creates text with both selection and cursor highlighting (visual mode)
func (m *EnhancedJSONModal) buildTextWithSelectionAndCursor() string {
	if !m.isSelecting {
		return m.buildTextWithCursor()
	}

	// Get selection bounds
	startY, endY := m.selectStart.y, m.selectEnd.y
	startX, endX := m.selectStart.x, m.selectEnd.x

	// Ensure start comes before end
	if startY > endY || (startY == endY && startX > endX) {
		startY, endY = endY, startY
		startX, endX = endX, startX
	}

	var result strings.Builder

	for i, line := range m.lines {
		if i < startY || i > endY {
			// Lines outside selection - show with cursor if applicable
			if i == m.cursorY {
				if m.cursorX < len(line) {
					result.WriteString(style.ColorizeJSON(line[:m.cursorX]))
					result.WriteString("[white:blue]")
					result.WriteString(string(line[m.cursorX]))
					result.WriteString("[-:-]")
					result.WriteString(style.ColorizeJSON(line[m.cursorX+1:]))
				} else {
					result.WriteString(style.ColorizeJSON(line))
					result.WriteString("[white:blue] [-:-]")
				}
			} else {
				result.WriteString(style.ColorizeJSON(line))
			}
		} else if i == startY && i == endY {
			// Single line selection
			if startX < len(line) && endX <= len(line) {
				result.WriteString(style.ColorizeJSON(line[:startX]))
				result.WriteString("[black:yellow]") // Highlight selection

				// Handle cursor within selection
				if i == m.cursorY && m.cursorX >= startX && m.cursorX < endX {
					// Cursor is inside selection
					if m.cursorX > startX {
						result.WriteString(line[startX:m.cursorX])
					}
					result.WriteString("[blue:yellow]") // Cursor in selection - blue text on yellow
					result.WriteString(string(line[m.cursorX]))
					result.WriteString("[black:yellow]") // Back to selection color
					if m.cursorX+1 < endX {
						result.WriteString(line[m.cursorX+1 : endX])
					}
				} else {
					result.WriteString(line[startX:endX])
				}

				result.WriteString("[-:-]") // Reset colors

				// Handle cursor after selection
				if i == m.cursorY && m.cursorX >= endX {
					if m.cursorX < len(line) {
						result.WriteString(style.ColorizeJSON(line[endX:m.cursorX]))
						result.WriteString("[white:blue]")
						result.WriteString(string(line[m.cursorX]))
						result.WriteString("[-:-]")
						result.WriteString(style.ColorizeJSON(line[m.cursorX+1:]))
					} else {
						result.WriteString(style.ColorizeJSON(line[endX:]))
						result.WriteString("[white:blue] [-:-]")
					}
				} else {
					result.WriteString(style.ColorizeJSON(line[endX:]))
				}
			} else {
				result.WriteString(style.ColorizeJSON(line))
			}
		} else if i == startY {
			// First line of multi-line selection
			result.WriteString(style.ColorizeJSON(line[:startX]))
			result.WriteString("[black:yellow]")

			// Handle cursor in first line
			if i == m.cursorY && m.cursorX >= startX {
				if m.cursorX > startX {
					result.WriteString(line[startX:m.cursorX])
				}
				result.WriteString("[blue:yellow]")
				result.WriteString(string(line[m.cursorX]))
				result.WriteString("[black:yellow]")
				if m.cursorX+1 < len(line) {
					result.WriteString(line[m.cursorX+1:])
				}
			} else {
				result.WriteString(line[startX:])
			}

			result.WriteString("[-:-]")
		} else if i == endY {
			// Last line of multi-line selection
			result.WriteString("[black:yellow]")

			// Handle cursor in last line
			if i == m.cursorY && m.cursorX < endX {
				if m.cursorX > 0 {
					result.WriteString(line[:m.cursorX])
				}
				result.WriteString("[blue:yellow]")
				result.WriteString(string(line[m.cursorX]))
				result.WriteString("[black:yellow]")
				if m.cursorX+1 < endX {
					result.WriteString(line[m.cursorX+1 : endX])
				}
			} else if endX <= len(line) {
				result.WriteString(line[:endX])
			} else {
				result.WriteString(line)
			}

			result.WriteString("[-:-]")

			// Handle cursor after selection on last line
			if i == m.cursorY && m.cursorX >= endX {
				if m.cursorX < len(line) {
					result.WriteString(style.ColorizeJSON(line[endX:m.cursorX]))
					result.WriteString("[white:blue]")
					result.WriteString(string(line[m.cursorX]))
					result.WriteString("[-:-]")
					result.WriteString(style.ColorizeJSON(line[m.cursorX+1:]))
				} else {
					result.WriteString(style.ColorizeJSON(line[endX:]))
					result.WriteString("[white:blue] [-:-]")
				}
			} else {
				result.WriteString(style.ColorizeJSON(line[endX:]))
			}
		} else {
			// Middle lines of selection - fully highlighted
			result.WriteString("[black:yellow]")

			// Handle cursor in middle lines
			if i == m.cursorY {
				if m.cursorX < len(line) {
					if m.cursorX > 0 {
						result.WriteString(line[:m.cursorX])
					}
					result.WriteString("[blue:yellow]")
					result.WriteString(string(line[m.cursorX]))
					result.WriteString("[black:yellow]")
					if m.cursorX+1 < len(line) {
						result.WriteString(line[m.cursorX+1:])
					}
				} else {
					result.WriteString(line)
				}
			} else {
				result.WriteString(line)
			}

			result.WriteString("[-:-]")
		}

		if i < len(m.lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}


// scrollToShowCursor ensures the cursor position is visible in the textView
func (m *EnhancedJSONModal) scrollToShowCursor() {
	if len(m.lines) == 0 {
		return
	}

	// Get current textView dimensions
	_, _, _, height := m.textView.GetRect()
	if height <= 2 { // Account for borders
		return
	}
	viewHeight := height - 2 // Subtract border height

	// Calculate if cursor is outside visible area
	if m.cursorY < m.scrollY {
		// Cursor is above visible area - scroll up
		m.scrollY = m.cursorY
	} else if m.cursorY >= m.scrollY+viewHeight {
		// Cursor is below visible area - scroll down
		m.scrollY = m.cursorY - viewHeight + 1
	}

	// Ensure scrollY is within bounds
	if m.scrollY < 0 {
		m.scrollY = 0
	}

	// Fix: Allow scrolling to show the last lines properly
	// If we have fewer lines than view height, don't scroll at all
	if len(m.lines) <= viewHeight {
		m.scrollY = 0
	} else {
		// Allow scrolling until the last line is visible at the bottom
		maxScrollY := len(m.lines) - viewHeight
		if m.scrollY > maxScrollY {
			m.scrollY = maxScrollY
		}

		// Special case: if cursor is on the very last line, make sure we can see it
		if m.cursorY == len(m.lines)-1 && m.scrollY < len(m.lines)-viewHeight {
			m.scrollY = len(m.lines) - viewHeight
		}
	}

	// Apply vertical scrolling to textView
	m.textView.ScrollTo(m.scrollY, 0)
}

func (m *EnhancedJSONModal) updateStatusBar() {
	var status string
	switch m.mode {
	case ModeNormal:
		status = "[white]NORMAL[white] | hjkl:move | v:visual | /:search | n/N:next/prev | y:copy line | Y:copy all | c:copy value | r:resize | q:quit"
	case ModeVisual:
		status = "[yellow]VISUAL[white] | hjkl:extend | y:copy selection | Esc:exit"
	case ModeResize:
		status = "[green]RESIZE[white] | hjkl:resize | r:reset | q:exit resize"
	case ModeSearch:
		status = fmt.Sprintf("[blue]SEARCH[white] | /%s | Esc:exit", m.searchBuffer)
	case ModeCommand:
		status = "[red]COMMAND[white] | Esc:exit"
	}

	status += fmt.Sprintf(" | Pos: %d,%d | Size: %dx%d", m.cursorX, m.cursorY, m.width, m.height)
	m.statusBar.SetText(status)
}

func (m *EnhancedJSONModal) closeModal() {
	pages := m.view.manager.Pages()
	if pages.HasPage(manager.ModalJSON) {
		pages.RemovePage(manager.ModalJSON)
	}

	// Return focus to the results table when modal closes
	m.view.manager.App().SetFocus(m.view.components.resultsTable)
}

// Public API for the enhanced modal
func (v *View) showEnhancedJSONModal(entry *DocEntry) {
	modal := NewEnhancedJSONModal(v)
	modal.ShowJSON(entry)
}

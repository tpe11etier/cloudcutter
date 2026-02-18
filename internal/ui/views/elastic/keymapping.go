package elastic

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ActionType represents the type of action to execute
type ActionType int

const (
	ActionFocus ActionType = iota
	ActionToggle
	ActionNavigate
	ActionFilter
	ActionDocument
	ActionEdit
	ActionClear
	ActionDelete
	ActionMove
	ActionReorder
)

// ComponentType represents different UI components in the elastic view
type ComponentType int

const (
	ComponentFilterInput ComponentType = iota
	ComponentActiveFilters
	ComponentIndexInput
	ComponentFieldList
	ComponentSelectedList
	ComponentResultsTable
	ComponentTimeframeInput
	ComponentNumResultsInput
	ComponentLocalFilterInput
)

// KeyAction represents an action that can be executed
type KeyAction struct {
	Type ActionType
	Data interface{} // Flexible data payload for the action
}

// KeyMapping represents a key binding configuration
type KeyMapping struct {
	Key       tcell.Key
	Rune      rune
	Modifiers tcell.ModMask
	Component *ComponentType // nil means global shortcut
	Action    KeyAction
}

// KeyMappingConfig holds all key mappings for the elastic view
type KeyMappingConfig struct {
	GlobalMappings    []KeyMapping
	ComponentMappings map[ComponentType][]KeyMapping
}

// NewKeyMappingConfig creates the default key mapping configuration
func NewKeyMappingConfig() *KeyMappingConfig {
	config := &KeyMappingConfig{
		ComponentMappings: make(map[ComponentType][]KeyMapping),
	}

	// Global shortcuts that work regardless of focus
	config.GlobalMappings = []KeyMapping{
		{Key: tcell.KeyCtrlA, Modifiers: tcell.ModCtrl, Action: KeyAction{Type: ActionFocus, Data: ComponentFieldList}},
		{Key: tcell.KeyCtrlS, Modifiers: tcell.ModCtrl, Action: KeyAction{Type: ActionFocus, Data: ComponentSelectedList}},
		{Key: tcell.KeyCtrlR, Modifiers: tcell.ModCtrl, Action: KeyAction{Type: ActionFocus, Data: ComponentResultsTable}},
		{Key: tcell.KeyTab, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "forward"}},
		{Key: tcell.KeyBacktab, Modifiers: tcell.ModShift, Action: KeyAction{Type: ActionNavigate, Data: "backward"}},
	}

	// Component-specific mappings
	config.ComponentMappings[ComponentFilterInput] = []KeyMapping{
		{Key: tcell.KeyEsc, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionClear, Data: "text"}},
		{Key: tcell.KeyEnter, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionFilter, Data: "add"}},
	}

	config.ComponentMappings[ComponentActiveFilters] = []KeyMapping{
		{Key: tcell.KeyEsc, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionFocus, Data: ComponentFilterInput}},
		{Key: tcell.KeyDelete, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionDelete, Data: "selected_filter"}},
		{Key: tcell.KeyBackspace2, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionDelete, Data: "selected_filter"}},
		{Key: tcell.KeyBackspace, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionDelete, Data: "selected_filter"}},
	}

	config.ComponentMappings[ComponentIndexInput] = []KeyMapping{
		{Key: tcell.KeyEsc, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionClear, Data: "reset_to_current"}},
		{Key: tcell.KeyEnter, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionEdit, Data: "change_index"}},
	}

	config.ComponentMappings[ComponentResultsTable] = []KeyMapping{
		// Vim navigation
		{Rune: 'h', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "left"}},
		{Rune: 'j', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "down"}},
		{Rune: 'k', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "up"}},
		{Rune: 'l', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "right"}},
		// Existing actions
		{Rune: 'f', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionToggle, Data: "field_list"}},
		{Rune: 'a', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionFocus, Data: ComponentFieldList}},
		{Rune: 's', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionFocus, Data: ComponentSelectedList}},
		{Rune: 'r', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionToggle, Data: "row_numbers"}},
		{Rune: 'n', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "next_page"}},
		{Rune: 'p', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "previous_page"}},
		{Rune: '/', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionFilter, Data: "show_prompt"}},
		{Key: tcell.KeyEnter, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionDocument, Data: "fetch_full"}},
	}

	config.ComponentMappings[ComponentFieldList] = []KeyMapping{
		// Vim navigation
		{Rune: 'j', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "down"}},
		{Rune: 'k', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "up"}},
		{Rune: 'h', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "left"}},
		{Rune: 'l', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "right"}},
		// Existing actions
		{Rune: 's', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionFocus, Data: ComponentSelectedList}},
		{Rune: '/', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionFilter, Data: "show_prompt"}},
		{Key: tcell.KeyEnter, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionToggle, Data: "field_selection"}},
		{Key: tcell.KeyBackspace, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionClear, Data: "field_filter"}},
		{Key: tcell.KeyBackspace2, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionClear, Data: "field_filter"}},
	}

	config.ComponentMappings[ComponentSelectedList] = []KeyMapping{
		// Vim navigation (cursor movement)
		{Rune: 'k', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "up"}},
		{Rune: 'j', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionNavigate, Data: "down"}},
		// SHIFT+j/k for reordering fields (uppercase letters)
		{Rune: 'J', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionReorder, Data: "down"}},
		{Rune: 'K', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionReorder, Data: "up"}},
		// Other actions
		{Rune: 'a', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionFocus, Data: ComponentFieldList}},
		{Key: tcell.KeyEnter, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionToggle, Data: "field_selection"}},
	}

	config.ComponentMappings[ComponentTimeframeInput] = []KeyMapping{
		{Key: tcell.KeyEsc, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionFocus, Data: ComponentFilterInput}},
		{Key: tcell.KeyEnter, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionEdit, Data: "apply_timeframe"}},
	}

	config.ComponentMappings[ComponentNumResultsInput] = []KeyMapping{
		{Key: tcell.KeyEsc, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionFocus, Data: ComponentFilterInput}},
	}

	config.ComponentMappings[ComponentLocalFilterInput] = []KeyMapping{
		{Key: tcell.KeyEsc, Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionFocus, Data: ComponentFilterInput}},
		{Rune: '/', Modifiers: tcell.ModNone, Action: KeyAction{Type: ActionFilter, Data: "show_prompt"}},
	}

	return config
}

// KeyMappingResolver resolves key events to actions
type KeyMappingResolver struct {
	config *KeyMappingConfig
	view   *View
}

// NewKeyMappingResolver creates a new key mapping resolver
func NewKeyMappingResolver(view *View) *KeyMappingResolver {
	return &KeyMappingResolver{
		config: NewKeyMappingConfig(),
		view:   view,
	}
}

// ResolveKeyEvent resolves a key event to an action, returns nil if no action found
func (r *KeyMappingResolver) ResolveKeyEvent(event *tcell.EventKey, currentFocus tview.Primitive) *KeyAction {
	// First check global mappings
	for _, mapping := range r.config.GlobalMappings {
		if r.matchesKeyMapping(event, mapping) {
			return &mapping.Action
		}
	}

	// Then check component-specific mappings
	component := r.getComponentType(currentFocus)
	if component != nil {
		if mappings, exists := r.config.ComponentMappings[*component]; exists {
			for _, mapping := range mappings {
				if r.matchesKeyMapping(event, mapping) {
					return &mapping.Action
				}
			}
		}
	}

	// Handle special cases for numeric keys in active filters
	if component != nil && *component == ComponentActiveFilters {
		if event.Key() == tcell.KeyRune {
			if rune := event.Rune(); rune >= '1' && rune <= '9' {
				return &KeyAction{Type: ActionDelete, Data: int(rune - '1')} // Convert to 0-based index
			}
		}
	}

	// Handle global Esc behavior
	if event.Key() == tcell.KeyEsc {
		return r.resolveEscapeAction(currentFocus)
	}

	return nil
}

// matchesKeyMapping checks if an event matches a key mapping
func (r *KeyMappingResolver) matchesKeyMapping(event *tcell.EventKey, mapping KeyMapping) bool {
	if mapping.Key != tcell.KeyNUL && event.Key() == mapping.Key {
		return event.Modifiers() == mapping.Modifiers
	}
	if mapping.Rune != 0 && event.Key() == tcell.KeyRune && event.Rune() == mapping.Rune {
		return event.Modifiers() == mapping.Modifiers
	}
	return false
}

// getComponentType maps tview primitives to our component types
func (r *KeyMappingResolver) getComponentType(focus tview.Primitive) *ComponentType {
	switch focus {
	case r.view.components.filterInput:
		return &[]ComponentType{ComponentFilterInput}[0]
	case r.view.components.activeFilters:
		return &[]ComponentType{ComponentActiveFilters}[0]
	case r.view.components.indexInput:
		return &[]ComponentType{ComponentIndexInput}[0]
	case r.view.components.fieldList:
		return &[]ComponentType{ComponentFieldList}[0]
	case r.view.components.selectedList:
		return &[]ComponentType{ComponentSelectedList}[0]
	case r.view.components.resultsTable:
		return &[]ComponentType{ComponentResultsTable}[0]
	case r.view.components.timeframeInput:
		return &[]ComponentType{ComponentTimeframeInput}[0]
	case r.view.components.numResultsInput:
		return &[]ComponentType{ComponentNumResultsInput}[0]
	case r.view.components.localFilterInput:
		return &[]ComponentType{ComponentLocalFilterInput}[0]
	}
	return nil
}

// resolveEscapeAction handles context-dependent Esc key behavior
func (r *KeyMappingResolver) resolveEscapeAction(currentFocus tview.Primitive) *KeyAction {
	switch currentFocus {
	case r.view.components.resultsTable:
		return &KeyAction{Type: ActionFocus, Data: ComponentFieldList}
	case r.view.components.fieldList:
		return &KeyAction{Type: ActionFocus, Data: ComponentFilterInput}
	default:
		return &KeyAction{Type: ActionFocus, Data: ComponentFilterInput} // Default behavior
	}
}

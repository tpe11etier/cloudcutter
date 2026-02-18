package elastic

import (
	"reflect"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestNewKeyMappingConfig(t *testing.T) {
	config := NewKeyMappingConfig()

	// Test that global mappings are properly configured
	if len(config.GlobalMappings) != 5 {
		t.Errorf("Expected 5 global mappings, got %d", len(config.GlobalMappings))
	}

	if config.ComponentMappings == nil {
		t.Error("Component mappings should be initialized")
	}

	// Test that all components have mappings
	expectedComponents := []ComponentType{
		ComponentFilterInput,
		ComponentActiveFilters,
		ComponentIndexInput,
		ComponentResultsTable,
		ComponentFieldList,
		ComponentSelectedList,
		ComponentTimeframeInput,
		ComponentNumResultsInput,
		ComponentLocalFilterInput,
	}

	for _, component := range expectedComponents {
		mappings, exists := config.ComponentMappings[component]
		if !exists {
			t.Errorf("Component %v should have mappings", component)
		}
		if len(mappings) == 0 {
			t.Errorf("Component %v should have non-empty mappings", component)
		}
	}
}

func TestKeyMappingResolver_GlobalMappings(t *testing.T) {
	mockView := &View{
		components: viewComponents{
			filterInput:      tview.NewInputField(),
			activeFilters:    tview.NewTextView(),
			indexInput:       tview.NewInputField(),
			fieldList:        tview.NewList(),
			selectedList:     tview.NewList(),
			resultsTable:     tview.NewTable(),
			timeframeInput:   tview.NewInputField(),
			numResultsInput:  tview.NewInputField(),
			localFilterInput: tview.NewInputField(),
		},
	}

	resolver := NewKeyMappingResolver(mockView)

	tests := []struct {
		name           string
		event          *tcell.EventKey
		expectedAction *KeyAction
	}{
		{
			name:  "Ctrl+A focuses field list",
			event: tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl),
			expectedAction: &KeyAction{
				Type: ActionFocus,
				Data: ComponentFieldList,
			},
		},
		{
			name:  "Ctrl+S focuses selected list",
			event: tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModCtrl),
			expectedAction: &KeyAction{
				Type: ActionFocus,
				Data: ComponentSelectedList,
			},
		},
		{
			name:  "Ctrl+R focuses results table",
			event: tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModCtrl),
			expectedAction: &KeyAction{
				Type: ActionFocus,
				Data: ComponentResultsTable,
			},
		},
		{
			name:  "Tab navigates forward",
			event: tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone),
			expectedAction: &KeyAction{
				Type: ActionNavigate,
				Data: "forward",
			},
		},
		{
			name:  "Shift+Tab navigates backward",
			event: tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModShift),
			expectedAction: &KeyAction{
				Type: ActionNavigate,
				Data: "backward",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := resolver.ResolveKeyEvent(tt.event, nil)
			if !reflect.DeepEqual(tt.expectedAction, action) {
				t.Errorf("Expected action %+v, got %+v", tt.expectedAction, action)
			}
		})
	}
}

func TestKeyMappingResolver_ComponentSpecificMappings(t *testing.T) {
	mockView := &View{
		components: viewComponents{
			filterInput:      tview.NewInputField(),
			activeFilters:    tview.NewTextView(),
			indexInput:       tview.NewInputField(),
			fieldList:        tview.NewList(),
			selectedList:     tview.NewList(),
			resultsTable:     tview.NewTable(),
			timeframeInput:   tview.NewInputField(),
			numResultsInput:  tview.NewInputField(),
			localFilterInput: tview.NewInputField(),
		},
	}

	resolver := NewKeyMappingResolver(mockView)

	tests := []struct {
		name           string
		component      tview.Primitive
		event          *tcell.EventKey
		expectedAction *KeyAction
	}{
		// FilterInput tests
		{
			name:      "FilterInput: Esc clears text",
			component: mockView.components.filterInput,
			event:     tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone),
			expectedAction: &KeyAction{
				Type: ActionClear,
				Data: "text",
			},
		},
		{
			name:      "FilterInput: Enter adds filter",
			component: mockView.components.filterInput,
			event:     tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone),
			expectedAction: &KeyAction{
				Type: ActionFilter,
				Data: "add",
			},
		},

		// ActiveFilters tests
		{
			name:      "ActiveFilters: Delete removes selected filter",
			component: mockView.components.activeFilters,
			event:     tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone),
			expectedAction: &KeyAction{
				Type: ActionDelete,
				Data: "selected_filter",
			},
		},

		// ResultsTable tests
		{
			name:      "ResultsTable: 'f' toggles field list",
			component: mockView.components.resultsTable,
			event:     tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone),
			expectedAction: &KeyAction{
				Type: ActionToggle,
				Data: "field_list",
			},
		},
		{
			name:      "ResultsTable: 'a' focuses field list",
			component: mockView.components.resultsTable,
			event:     tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone),
			expectedAction: &KeyAction{
				Type: ActionFocus,
				Data: ComponentFieldList,
			},
		},
		{
			name:      "ResultsTable: 'n' navigates to next page",
			component: mockView.components.resultsTable,
			event:     tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone),
			expectedAction: &KeyAction{
				Type: ActionNavigate,
				Data: "next_page",
			},
		},

		// FieldList tests
		{
			name:      "FieldList: 's' focuses selected list",
			component: mockView.components.fieldList,
			event:     tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone),
			expectedAction: &KeyAction{
				Type: ActionFocus,
				Data: ComponentSelectedList,
			},
		},
		{
			name:      "FieldList: Enter toggles field selection",
			component: mockView.components.fieldList,
			event:     tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone),
			expectedAction: &KeyAction{
				Type: ActionToggle,
				Data: "field_selection",
			},
		},

		// SelectedList tests
		{
			name:      "SelectedList: shift 'k' moves field up",
			component: mockView.components.selectedList,
			event:     tcell.NewEventKey(tcell.KeyRune, 'K', tcell.ModNone),
			expectedAction: &KeyAction{
				Type: ActionReorder,
				Data: "up",
			},
		},
		{
			name:      "SelectedList: shift 'j' moves field down",
			component: mockView.components.selectedList,
			event:     tcell.NewEventKey(tcell.KeyRune, 'J', tcell.ModNone),
			expectedAction: &KeyAction{
				Type: ActionReorder,
				Data: "down",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := resolver.ResolveKeyEvent(tt.event, tt.component)
			if !reflect.DeepEqual(tt.expectedAction, action) {
				t.Errorf("Expected action %+v, got %+v", tt.expectedAction, action)
			}
		})
	}
}

func TestKeyMappingResolver_NumericKeysInActiveFilters(t *testing.T) {
	mockView := &View{
		components: viewComponents{
			activeFilters: tview.NewTextView(),
		},
	}

	resolver := NewKeyMappingResolver(mockView)

	tests := []struct {
		name           string
		rune           rune
		expectedAction *KeyAction
	}{
		{
			name: "Key '1' deletes filter at index 0",
			rune: '1',
			expectedAction: &KeyAction{
				Type: ActionDelete,
				Data: 0,
			},
		},
		{
			name: "Key '5' deletes filter at index 4",
			rune: '5',
			expectedAction: &KeyAction{
				Type: ActionDelete,
				Data: 4,
			},
		},
		{
			name: "Key '9' deletes filter at index 8",
			rune: '9',
			expectedAction: &KeyAction{
				Type: ActionDelete,
				Data: 8,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tcell.NewEventKey(tcell.KeyRune, tt.rune, tcell.ModNone)
			action := resolver.ResolveKeyEvent(event, mockView.components.activeFilters)
			if !reflect.DeepEqual(tt.expectedAction, action) {
				t.Errorf("Expected action %+v, got %+v", tt.expectedAction, action)
			}
		})
	}

	// Test non-numeric keys don't trigger this behavior
	t.Run("Non-numeric keys don't trigger delete", func(t *testing.T) {
		event := tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone)
		action := resolver.ResolveKeyEvent(event, mockView.components.activeFilters)
		if action != nil {
			t.Errorf("Expected nil action, got %+v", action)
		}
	})

	t.Run("Key '0' doesn't trigger delete", func(t *testing.T) {
		event := tcell.NewEventKey(tcell.KeyRune, '0', tcell.ModNone)
		action := resolver.ResolveKeyEvent(event, mockView.components.activeFilters)
		if action != nil {
			t.Errorf("Expected nil action, got %+v", action)
		}
	})
}

func TestKeyMappingResolver_EscapeKeyBehavior(t *testing.T) {
	mockView := &View{
		components: viewComponents{
			filterInput:  tview.NewInputField(),
			fieldList:    tview.NewList(),
			resultsTable: tview.NewTable(),
		},
	}

	resolver := NewKeyMappingResolver(mockView)

	tests := []struct {
		name           string
		currentFocus   tview.Primitive
		expectedAction *KeyAction
	}{
		{
			name:         "Esc from results table focuses field list",
			currentFocus: mockView.components.resultsTable,
			expectedAction: &KeyAction{
				Type: ActionFocus,
				Data: ComponentFieldList,
			},
		},
		{
			name:         "Esc from field list focuses filter input",
			currentFocus: mockView.components.fieldList,
			expectedAction: &KeyAction{
				Type: ActionFocus,
				Data: ComponentFilterInput,
			},
		},
		{
			name:         "Esc from unknown component focuses filter input",
			currentFocus: tview.NewBox(), // Unknown component
			expectedAction: &KeyAction{
				Type: ActionFocus,
				Data: ComponentFilterInput,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)
			action := resolver.ResolveKeyEvent(event, tt.currentFocus)
			if !reflect.DeepEqual(tt.expectedAction, action) {
				t.Errorf("Expected action %+v, got %+v", tt.expectedAction, action)
			}
		})
	}
}

func TestKeyMappingResolver_GetComponentType(t *testing.T) {
	mockView := &View{
		components: viewComponents{
			filterInput:      tview.NewInputField(),
			activeFilters:    tview.NewTextView(),
			indexInput:       tview.NewInputField(),
			fieldList:        tview.NewList(),
			selectedList:     tview.NewList(),
			resultsTable:     tview.NewTable(),
			timeframeInput:   tview.NewInputField(),
			numResultsInput:  tview.NewInputField(),
			localFilterInput: tview.NewInputField(),
		},
	}

	resolver := NewKeyMappingResolver(mockView)

	tests := []struct {
		name     string
		focus    tview.Primitive
		expected *ComponentType
	}{
		{
			name:     "FilterInput maps correctly",
			focus:    mockView.components.filterInput,
			expected: &[]ComponentType{ComponentFilterInput}[0],
		},
		{
			name:     "ActiveFilters maps correctly",
			focus:    mockView.components.activeFilters,
			expected: &[]ComponentType{ComponentActiveFilters}[0],
		},
		{
			name:     "ResultsTable maps correctly",
			focus:    mockView.components.resultsTable,
			expected: &[]ComponentType{ComponentResultsTable}[0],
		},
		{
			name:     "Unknown component returns nil",
			focus:    tview.NewBox(),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.getComponentType(tt.focus)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil result, got %+v", result)
				}
			} else {
				if result == nil {
					t.Errorf("Expected non-nil result, got nil")
				} else if *tt.expected != *result {
					t.Errorf("Expected component type %v, got %v", *tt.expected, *result)
				}
			}
		})
	}
}

func TestKeyMappingResolver_MatchesKeyMapping(t *testing.T) {
	resolver := &KeyMappingResolver{}

	tests := []struct {
		name     string
		event    *tcell.EventKey
		mapping  KeyMapping
		expected bool
	}{
		{
			name:     "Key match with no modifiers",
			event:    tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone),
			mapping:  KeyMapping{Key: tcell.KeyEnter, Modifiers: tcell.ModNone},
			expected: true,
		},
		{
			name:     "Key match with modifiers",
			event:    tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl),
			mapping:  KeyMapping{Key: tcell.KeyCtrlA, Modifiers: tcell.ModCtrl},
			expected: true,
		},
		{
			name:     "Rune match",
			event:    tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone),
			mapping:  KeyMapping{Rune: 'a', Modifiers: tcell.ModNone},
			expected: true,
		},
		{
			name:     "Key mismatch",
			event:    tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone),
			mapping:  KeyMapping{Key: tcell.KeyEsc, Modifiers: tcell.ModNone},
			expected: false,
		},
		{
			name:     "Modifier mismatch",
			event:    tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModNone),
			mapping:  KeyMapping{Key: tcell.KeyCtrlA, Modifiers: tcell.ModCtrl},
			expected: false,
		},
		{
			name:     "Rune mismatch",
			event:    tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone),
			mapping:  KeyMapping{Rune: 'b', Modifiers: tcell.ModNone},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.matchesKeyMapping(tt.event, tt.mapping)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestKeyMappingResolver_NoAction(t *testing.T) {
	mockView := &View{
		components: viewComponents{
			filterInput: tview.NewInputField(),
		},
	}

	resolver := NewKeyMappingResolver(mockView)

	// Test unmapped key returns nil
	event := tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone)
	action := resolver.ResolveKeyEvent(event, mockView.components.filterInput)
	if action != nil {
		t.Errorf("Expected nil action for unmapped key, got %+v", action)
	}
}

// Benchmark tests to ensure key resolution is fast
func BenchmarkKeyMappingResolver_ResolveKeyEvent(b *testing.B) {
	mockView := &View{
		components: viewComponents{
			filterInput:  tview.NewInputField(),
			resultsTable: tview.NewTable(),
		},
	}

	resolver := NewKeyMappingResolver(mockView)
	event := tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolver.ResolveKeyEvent(event, mockView.components.resultsTable)
	}
}

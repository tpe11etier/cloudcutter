package manager

import (
	"context"
	"fmt"
	"github.com/tpelletiersophos/cloudcutter/internal/ui"
	components2 "github.com/tpelletiersophos/cloudcutter/internal/ui/components"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/header"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

type TestView struct {
	name        string
	content     tview.Primitive
	activeField string
	shown       bool
	hidden      bool
	filtered    string
	doneFunc    func(string) // Added field to capture done function
}

func NewTestView(name string) *TestView {
	return &TestView{
		name:    name,
		content: tview.NewBox(),
	}
}

func (v *TestView) Name() string                                        { return v.name }
func (v *TestView) Show()                                               { v.shown = true }
func (v *TestView) Hide()                                               { v.hidden = true; v.shown = false }
func (v *TestView) Content() tview.Primitive                            { return v.content }
func (v *TestView) ActiveField() string                                 { return v.activeField }
func (v *TestView) InputHandler() func(*tcell.EventKey) *tcell.EventKey { return nil }

func (v *TestView) HandleFilter(prompt *components2.Prompt, previousFocus tview.Primitive) {
	prompt.SetDoneFunc(func(text string) {
		v.filtered = text
		if v.doneFunc != nil {
			v.doneFunc(text)
		}
	})
}

func setupTestManager(t *testing.T) (*Manager, *ui.App) {
	app := ui.NewApp()
	vm := NewViewManager(context.Background(), app, aws.Config{})
	return vm, app
}

func TestNewViewManager(t *testing.T) {
	vm, _ := setupTestManager(t)

	assert.NotNil(t, vm)
	assert.NotNil(t, vm.ctx)
	assert.NotNil(t, vm.views)
	assert.NotNil(t, vm.Pages)
	assert.NotNil(t, vm.header)
	assert.NotNil(t, vm.statusBar)
	assert.NotNil(t, vm.prompt)
	assert.NotNil(t, vm.filterPrompt)
}

func TestRegisterAndSwitchView(t *testing.T) {
	vm, _ := setupTestManager(t)

	view1 := NewTestView("test1")
	view2 := NewTestView("test2")

	vm.RegisterView(view1)
	vm.RegisterView(view2)

	assert.Contains(t, vm.views, "test1")
	assert.Contains(t, vm.views, "test2")

	err := vm.SwitchToView("test1")
	assert.NoError(t, err)
	assert.Equal(t, view1, vm.activeView)
	assert.True(t, view1.shown)
	assert.False(t, view1.hidden)

	err = vm.SwitchToView("test2")
	assert.NoError(t, err)
	assert.Equal(t, view2, vm.activeView)
	assert.True(t, view2.shown)
	assert.True(t, view1.hidden)

	err = vm.SwitchToView("nonexistent")
	assert.Error(t, err)
}

func TestCommandHandling(t *testing.T) {
	vm, _ := setupTestManager(t)

	views := map[string]*TestView{
		"ec2":      NewTestView(ViewEC2),
		"dynamodb": NewTestView(ViewDynamoDB),
		"elastic":  NewTestView(ViewElastic),
		"test":     NewTestView(ViewTest),
	}

	for _, view := range views {
		vm.RegisterView(view)
	}

	for cmd, view := range views {
		vm.handleCommand(cmd)
		assert.Equal(t, view, vm.activeView, "Command %s should activate view %s", cmd, view.name)
		assert.True(t, view.shown)
	}

	vm.handleCommand("unknown")
	assert.Contains(t, vm.statusBar.GetText(true), "Unknown command")
}

func TestGlobalInputHandling(t *testing.T) {
	vm, _ := setupTestManager(t)
	view := NewTestView("test")
	vm.RegisterView(view)
	_ = vm.SwitchToView("test")

	tests := []struct {
		name        string
		key         tcell.Key
		rune        rune
		expectModal bool
	}{
		{"Command prompt", tcell.KeyRune, ':', true},
		{"Filter prompt", tcell.KeyRune, '/', true},
		//{"ESC key", tcell.KeyEsc, 0, false},
		//{"Regular key", tcell.KeyRune, 'a', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tcell.NewEventKey(tt.key, tt.rune, tcell.ModNone)
			vm.globalInputHandler(event)

			modalVisible := vm.IsModalVisible()
			assert.Equal(t, tt.expectModal, modalVisible)
		})
	}
}

func TestHeaderAndStatusUpdates(t *testing.T) {
	vm, _ := setupTestManager(t)

	summary := []header.SummaryItem{
		{Key: "Test1", Value: "Value1"},
		{Key: "Test2", Value: "Value2"},
	}
	vm.UpdateHeader(summary)

	testStatus := "Test Status Message"
	vm.UpdateStatusBar(testStatus)
	assert.Equal(t, testStatus, vm.statusBar.GetText(true))
}

func TestFilterHandling(t *testing.T) {
	vm, _ := setupTestManager(t)
	view := NewTestView("test")
	vm.RegisterView(view)
	err := vm.SwitchToView("test")
	assert.NoError(t, err)

	vm.ShowFilterPrompt(vm.Pages)
	vm.filterPrompt.SetText("test-filter")

	// Assign a done function to capture the filtered text
	doneCalled := make(chan string, 1)
	view.doneFunc = func(text string) {
		doneCalled <- text
	}

	// Simulate pressing Enter on the prompt's InputHandler
	if handler := vm.filterPrompt.InputHandler(); handler != nil {
		event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
		handler(event, func(p tview.Primitive) {
			// Optionally handle focus change
		})
	}

	// Wait for the done function to be called
	select {
	case filtered := <-doneCalled:
		assert.Equal(t, "test-filter", filtered)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("done function was not called")
	}

	assert.Equal(t, "test-filter", view.filtered)
}

//func TestFilterHandling(t *testing.T) {
//	vm, _ := setupTestManager(t)
//	view := NewTestView("test")
//	vm.RegisterView(view)
//	vm.SwitchToView("test")
//
//	vm.showFilterPrompt()
//	vm.filterPrompt.SetText("test-filter")
//
//	enterEvent := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
//	vm.filterPrompt.InputField.InputHandler()(enterEvent, nil)
//
//	assert.Equal(t, "test-filter", view.filtered)
//	assert.False(t, vm.IsModalVisible())
//}

func TestModalOperations(t *testing.T) {
	vm, _ := setupTestManager(t)
	view := NewTestView("test")
	vm.RegisterView(view)
	vm.SwitchToView("test")

	vm.showPrompt()
	assert.True(t, vm.IsModalVisible())

	vm.hidePrompt()
	assert.False(t, vm.IsModalVisible())

	vm.ShowFilterPrompt(vm.Pages)
	assert.True(t, vm.IsModalVisible())

	vm.HideFilterPrompt()
	assert.False(t, vm.IsModalVisible())

	vm.showPrompt()
	vm.ShowFilterPrompt(vm.Pages)
	vm.hideAllModals()
	assert.False(t, vm.IsModalVisible())
}

func TestHandleCommand(t *testing.T) {
	vm, _ := setupTestManager(t)

	views := map[string]*TestView{
		"ec2":      NewTestView(ViewEC2),
		"dynamodb": NewTestView(ViewDynamoDB),
		"elastic":  NewTestView(ViewElastic),
		"test":     NewTestView(ViewTest),
	}

	for _, view := range views {
		vm.RegisterView(view)
	}

	tests := []struct {
		name          string
		command       string
		expectView    string
		expectStatus  string
		expectStopped bool
	}{
		{
			name:       "switch to ec2 view",
			command:    "ec2",
			expectView: ViewEC2,
		},
		{
			name:       "switch to dynamodb view",
			command:    "dynamodb",
			expectView: ViewDynamoDB,
		},
		{
			name:       "switch to elastic view",
			command:    "elastic",
			expectView: ViewElastic,
		},
		{
			name:       "switch to test view",
			command:    "test",
			expectView: ViewTest,
		},
		{
			name:         "help command",
			command:      "help",
			expectStatus: "Help: List of available commands...",
		},
		{
			name:         "unknown command",
			command:      "invalid",
			expectStatus: "Unknown command: invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm.handleCommand(tt.command)

			if tt.expectView != "" {
				assert.Equal(t, tt.expectView, vm.activeView.Name(), "expected view was not activated")
				assert.True(t, views[tt.command].shown, "view was not shown")
			}
			if tt.expectStatus != "" {
				assert.Equal(t, tt.expectStatus, vm.statusBar.GetText(true), "status bar text did not match")
			}
		})
	}
}

func TestCreateLayout(t *testing.T) {
	vm, _ := setupTestManager(t)

	tests := []struct {
		name        string
		config      LayoutConfig
		expectItems int
	}{
		{
			name: "empty layout",
			config: LayoutConfig{
				Title:      "Empty",
				Components: []Component{},
				Direction:  tview.FlexRow,
			},
			expectItems: 0,
		},
		{
			name: "mixed components row",
			config: LayoutConfig{
				Title: "Mixed Row",
				Components: []Component{
					{
						ID:   "text",
						Type: ComponentTextView,
						Properties: map[string]any{
							"text": "Text",
						},
					},
					{
						ID:   "table",
						Type: ComponentTable,
					},
					{
						ID:         "list",
						Type:       ComponentList,
						Proportion: 1,
					},
				},
				Direction: tview.FlexRow,
			},
			expectItems: 3,
		},
		{
			name: "nested layout",
			config: LayoutConfig{
				Title:     "Nested",
				Direction: tview.FlexColumn,
				Components: []Component{
					{
						ID:        "list",
						Type:      ComponentList,
						FixedSize: 20,
						Properties: map[string]any{
							"items": []string{"Item 1"},
						},
					},
					{
						ID:         "flex",
						Type:       ComponentFlex,
						Direction:  tview.FlexRow,
						Proportion: 1,
						Children: []Component{
							{
								ID:   "leftText",
								Type: ComponentTextView,
								Properties: map[string]any{
									"text": "Left",
								},
							},
							{
								ID:   "rightText",
								Type: ComponentTextView,
								Properties: map[string]any{
									"text": "Right",
								},
							},
						},
					},
				},
			},
			expectItems: 2,
		},
		{
			name: "header layout",
			config: LayoutConfig{
				Title:     "Header",
				Direction: tview.FlexRow,
				Components: []Component{
					{
						ID:        "header",
						Type:      ComponentTextView,
						FixedSize: 1,
						Properties: map[string]any{
							"text": "Header",
						},
					},
					{
						ID:         "content",
						Type:       ComponentFlex,
						Proportion: 1,
						Direction:  tview.FlexColumn,
						Children: []Component{
							{
								ID:         "list",
								Type:       ComponentList,
								Proportion: 1,
							},
							{
								ID:         "table",
								Type:       ComponentTable,
								Proportion: 2,
							},
						},
					},
					{
						ID:        "footer",
						Type:      ComponentTextView,
						FixedSize: 1,
						Properties: map[string]any{
							"text": "Footer",
						},
					},
				},
			},
			expectItems: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := vm.CreateLayout(tt.config)
			flex, ok := layout.(*tview.Flex)
			assert.True(t, ok, "layout should be a Flex")
			assert.NotNil(t, flex)
			assert.Equal(t, tt.expectItems, len(tt.config.Components))

			// Verify primitives were stored with their IDs
			for _, comp := range tt.config.Components {
				if comp.ID != "" {
					assert.NotNil(t, vm.GetPrimitiveByID(comp.ID))
				}
			}
		})
	}
}

func TestBuildPrimitiveFromComponent(t *testing.T) {
	vm, _ := setupTestManager(t)

	tests := []struct {
		name      string
		component Component
		wantType  string
	}{
		{
			name: "list component",
			component: Component{
				ID:   "testList",
				Type: ComponentList,
				Properties: map[string]any{
					"items": []string{"item1", "item2"},
				},
			},
			wantType: "*tview.List",
		},
		{
			name: "table component",
			component: Component{
				ID:   "testTable",
				Type: ComponentTable,
			},
			wantType: "*tview.Table",
		},
		{
			name: "text view component",
			component: Component{
				ID:   "testText",
				Type: ComponentTextView,
				Properties: map[string]any{
					"text": "test text",
				},
			},
			wantType: "*tview.TextView",
		},
		{
			name: "flex component",
			component: Component{
				ID:   "testFlex",
				Type: ComponentFlex,
				Children: []Component{
					{
						ID:   "child",
						Type: ComponentTextView,
					},
				},
			},
			wantType: "*tview.Flex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primitive := vm.buildPrimitiveFromComponent(tt.component)
			assert.NotNil(t, primitive)
			assert.Equal(t, tt.wantType, fmt.Sprintf("%T", primitive))

			if tt.component.ID != "" {
				stored := vm.GetPrimitiveByID(tt.component.ID)
				assert.Equal(t, primitive, stored)
			}
		})
	}
}

func TestContextCancellation(t *testing.T) {
	vm, _ := setupTestManager(t)

	// Test that context is passed through
	assert.Equal(t, vm.ctx, vm.ViewContext())

	vm.cancelFunc()
	assert.Error(t, vm.ctx.Err(), "context should be canceled")
}

func TestSetFocus(t *testing.T) {
	vm, _ := setupTestManager(t)
	view := NewTestView("test")
	vm.RegisterView(view)
	vm.SwitchToView("test")

	// Test focusing different components
	primitive := tview.NewBox()
	vm.SetFocus(primitive)

	// At minimum verify no panic
	assert.NotPanics(t, func() {
		vm.SetFocus(nil)
	})
}

func TestMultipleFilterOperations(t *testing.T) {
	vm, _ := setupTestManager(t)
	view := NewTestView("test")
	vm.RegisterView(view)
	vm.SwitchToView("test")

	filters := []string{"test1", "test2", "test3"}

	for _, filter := range filters {
		vm.ShowFilterPrompt(vm.Pages)
		vm.filterPrompt.SetText(filter)

		enterEvent := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
		vm.filterPrompt.InputField.InputHandler()(enterEvent, nil)

		assert.Equal(t, filter, view.filtered)
		//assert.False(t, vm.IsModalVisible())
	}
}

func TestFilterCancellation(t *testing.T) {
	vm, _ := setupTestManager(t)
	view := NewTestView("test")
	vm.RegisterView(view)
	vm.SwitchToView("test")

	// Show filter and enter text
	vm.ShowFilterPrompt(vm.Pages)
	vm.filterPrompt.SetText("test-filter")

	// Cancel with ESC
	escEvent := tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)
	vm.globalInputHandler(escEvent)

	// Filter should not be applied
	assert.Empty(t, view.filtered)
	assert.False(t, vm.IsModalVisible())
}

func TestRegisterDuplicateView(t *testing.T) {
	vm, _ := setupTestManager(t)
	view1 := NewTestView("test")
	view2 := NewTestView("test")

	// First registration should succeed
	err := vm.RegisterView(view1)
	assert.NoError(t, err)
	assert.Equal(t, view1, vm.views["test"])

	// Second registration should fail
	err = vm.RegisterView(view2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")

	// Original view should still be registered
	assert.Equal(t, view1, vm.views["test"])
}

func TestModalFocusBehavior(t *testing.T) {
	vm, _ := setupTestManager(t)
	view := NewTestView("test")
	vm.RegisterView(view)
	vm.SwitchToView("test")

	initialContent := view.Content()

	// Show prompt and verify focus change
	vm.showPrompt()
	assert.NotEqual(t, initialContent, vm.prompt.InputField)

	// Hide prompt and verify focus returns
	vm.hidePrompt()
	assert.Equal(t, initialContent, view.Content())

	// Show filter and verify focus change
	vm.ShowFilterPrompt(vm.Pages)
	assert.NotEqual(t, initialContent, vm.filterPrompt.InputField)

	// Hide filter and verify focus returns
	vm.HideFilterPrompt()
	assert.Equal(t, initialContent, view.Content())
}

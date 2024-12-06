package manager

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/ui"
	"github.com/tpelletiersophos/cloudcutter/ui/components"
)

const (
	ec2View    = "ec2"
	ecrView    = "ecr"
	dynamoView = "dynamodb"
	s3View     = "s3"
	testView   = "test"
)

// LayoutConfig holds configuration for creating a layout using tview.Flex.
type LayoutConfig struct {
	Title       string
	Components  []tview.Primitive
	Direction   int
	FixedSizes  []int
	Proportions []int
}

// ServiceView defines the interface that any view in the application must implement.
type ServiceView interface {
	Name() string
	GetContent() tview.Primitive
	Show()
	Hide()
	InputHandler() func(event *tcell.EventKey) *tcell.EventKey
}

// Manager manages the application's views, layout, and global input handling.
type Manager struct {
	App        *ui.App
	ctx        context.Context
	cancelFunc context.CancelFunc
	views      map[string]ServiceView
	activeView ServiceView
	pages      *tview.Pages
	prompt     *components.Prompt
	header     *components.Header
	statusBar  *components.StatusBar
	layout     *tview.Flex
	awsConfig  aws.Config
}

// NewViewManager creates and initializes a new Manager instance.
func NewViewManager(ctx context.Context, app *ui.App, awsConfig aws.Config) *Manager {
	ctx, cancel := context.WithCancel(ctx)
	return &Manager{
		ctx:        ctx,
		cancelFunc: cancel,
		App:        app,
		views:      make(map[string]ServiceView),
		pages:      tview.NewPages(),
		header:     components.NewHeader(),
		statusBar:  components.NewStatusBar(),
		prompt:     components.NewPrompt(),
		awsConfig:  awsConfig,
	}
}

// CreateLayout creates a layout based on the provided configuration.
func (vm *Manager) CreateLayout(cfg LayoutConfig) tview.Primitive {
	flex := tview.NewFlex().SetDirection(cfg.Direction)

	for i, component := range cfg.Components {
		fixedSize := 0
		proportion := 1
		if i < len(cfg.FixedSizes) {
			fixedSize = cfg.FixedSizes[i]
		}
		if i < len(cfg.Proportions) {
			proportion = cfg.Proportions[i]
		}
		flex.AddItem(component, fixedSize, proportion, i == 0)
	}

	return flex
}

// setupLayout sets up the main application layout.
func (vm *Manager) setupLayout() {
	vm.layout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(vm.header, 8, 0, false).
		AddItem(vm.pages, 0, 1, true).
		AddItem(vm.statusBar, 1, 0, false)

}

// RegisterView registers a view with the manager.
func (vm *Manager) RegisterView(view ServiceView) {
	vm.views[view.Name()] = view
}

// SwitchToView switches the application to display a different view.
func (vm *Manager) SwitchToView(name string) error {
	view, exists := vm.views[name]
	if !exists {
		return fmt.Errorf("view %s not found", name)
	}

	if vm.activeView != nil {
		vm.activeView.Hide()
	}

	vm.activeView = view
	view.Show()

	if !vm.pages.HasPage(name) {
		vm.pages.AddPage(name, view.GetContent(), true, true)
	}
	vm.pages.SwitchToPage(name)

	vm.statusBar.SetText(fmt.Sprintf("Status: %s view active", name))
	vm.header.SetTitle(fmt.Sprintf(" Cloud Cutter - %s ", name))

	return nil
}

// UpdateHeader updates the header component with summary information.
func (vm *Manager) UpdateHeader(summary []components.SummaryItem) {
	vm.header.UpdateSummary(summary)
}

// UpdateStatusBar updates the status bar with a new message.
func (vm *Manager) UpdateStatusBar(text string) {
	vm.statusBar.SetText(text)
}

func (vm *Manager) ViewContext() context.Context {
	return vm.ctx
}

// showPrompt displays the prompt for user input.
func (vm *Manager) showPrompt() {
	vm.prompt.SetDoneFunc(func(command string) {
		vm.handleCommand(command)
	})
	vm.prompt.SetCancelFunc(func() {
		vm.hidePrompt()
	})
	vm.prompt.InputField.SetLabel(" > ")
	vm.prompt.InputField.SetLabelColor(tcell.ColorTeal)
	vm.prompt.SetFieldWidth(30)

	// Configure the prompt's appearance
	vm.prompt.SetTitle(" Command ") // Set the title
	vm.prompt.SetBorder(true)       // Enable the border
	vm.prompt.SetTitleAlign(tview.AlignLeft)
	vm.prompt.SetBorderColor(tcell.ColorMediumTurquoise)

	vm.App.SetFocus(vm.prompt.InputField)

	modal := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false). // Top spacer
		AddItem(
			tview.NewFlex().
				AddItem(nil, 0, 1, false).       // Left spacer
				AddItem(vm.prompt, 50, 0, true). // Prompt with fixed width
				AddItem(nil, 0, 1, false),       // Right spacer
						3, 0, true). // Prompt height
		AddItem(nil, 0, 1, false) // Bottom spacer

	// Add the modal as a page
	vm.pages.AddPage("modalPrompt", modal, true, true)
}

// hidePrompt hides the prompt and returns focus to the active view.
func (vm *Manager) hidePrompt() {
	vm.pages.RemovePage("modalPrompt")
	vm.App.SetFocus(vm.activeView.GetContent())
}

// SetFocus sets focus to a specific UI component.
func (vm *Manager) SetFocus(p tview.Primitive) {
	vm.App.SetFocus(p)
}

// handleCommand handles commands entered by the user in the prompt.
func (vm *Manager) handleCommand(command string) {
	vm.hidePrompt()
	switch command {
	case "ec2":
		_ = vm.SwitchToView(ec2View)
	case "dynamodb":
		_ = vm.SwitchToView(dynamoView)
	case "test":
		_ = vm.SwitchToView(testView)
	case "help":
		vm.statusBar.SetText("Help: List of available commands...")
	case "exit":
		vm.App.Stop()
	default:
		vm.statusBar.SetText(fmt.Sprintf("Unknown command: %s", command))
	}
}

// globalInputHandler captures and handles global input events.
func (vm *Manager) globalInputHandler(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyRune && event.Rune() == ':' {
		vm.showPrompt()
		return nil
	}
	if event.Key() == tcell.KeyEsc {
		vm.hidePrompt()
		return nil
	}

	if vm.activeView != nil {
		return vm.activeView.InputHandler()(event)
	}
	return event
}

func (vm *Manager) Run() error {
	vm.setupLayout()
	vm.App.SetRoot(vm.layout, true)
	vm.App.EnableMouse(true)
	vm.App.SetInputCapture(vm.globalInputHandler)
	return vm.App.Run()
}

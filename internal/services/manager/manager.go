package manager

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/ahtui/internal/types"
	"github.com/tpelletiersophos/ahtui/ui"
	"github.com/tpelletiersophos/ahtui/ui/components"
)

type Manager struct {
	App        *ui.App
	ctx        context.Context
	views      map[string]types.ServiceView
	activeView types.ServiceView
	pages      *tview.Pages
	header     *components.Header
	statusBar  *components.StatusBar
	layout     *tview.Flex
	prompt     *components.Prompt
	awsConfig  aws.Config
}

func NewViewManager(ctx context.Context, app *ui.App, awsConfig aws.Config) *Manager {
	return &Manager{
		ctx:       ctx,
		App:       app,
		views:     make(map[string]types.ServiceView),
		pages:     tview.NewPages(),
		header:    components.NewHeader(),
		statusBar: components.NewStatusBar(),
		prompt:    components.NewPrompt("Command: "),
		awsConfig: awsConfig,
	}
}

func (vm *Manager) setupLayout() {
	vm.layout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(vm.header, 8, 0, false).
		AddItem(vm.pages, 0, 1, true).
		AddItem(vm.statusBar, 1, 0, false)

	vm.pages.AddPage("prompt", vm.prompt, true, false)
}

func (vm *Manager) RegisterView(view types.ServiceView) {
	vm.views[view.Name()] = view
	vm.pages.AddPage(view.Name(), view.GetContent(), true, false)
}

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
	vm.pages.SwitchToPage(name)
	vm.statusBar.SetText(fmt.Sprintf("Status: %s view active", name))

	vm.header.SetTitle(fmt.Sprintf(" AWS TUI - %s ", name)).SetTitleColor(tcell.ColorYellow)

	return nil
}

func (vm *Manager) UpdateHeader(summary []components.SummaryItem) {
	vm.header.UpdateSummary(summary)
}

func (vm *Manager) UpdateStatusBar(text string) {
	vm.statusBar.SetText(text)
}

func (vm *Manager) showPrompt() {
	vm.prompt.SetDoneFunc(func(command string) {
		vm.handleCommand(command)
	})
	vm.prompt.SetCancelFunc(func() {
		vm.hidePrompt()
	})
	vm.pages.ShowPage("prompt")
	vm.prompt.InputField.SetText("") // Clear any previous input
	vm.App.SetFocus(vm.prompt.InputField)
}
func (vm *Manager) hidePrompt() {
	vm.pages.HidePage("prompt")
	vm.App.SetFocus(vm.activeView.GetContent())
}

func (vm *Manager) SetFocus(p tview.Primitive) {
	vm.App.SetFocus(p)
}

func (vm *Manager) handleCommand(command string) {
	vm.hidePrompt()

	// Process the command
	switch command {
	case "ec2":
		_ = vm.SwitchToView("ec2")
	case "dynamodb":
		_ = vm.SwitchToView("dynamodb")
	case "help":
		vm.statusBar.SetText("Help: List of available commands...")
	case "exit":
		vm.App.Stop()
	default:
		vm.statusBar.SetText(fmt.Sprintf("Unknown command: %s", command))
	}

}

func (vm *Manager) globalInputHandler(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyRune && event.Rune() == ':' {
		vm.showPrompt()
		return nil
	}

	if vm.activeView != nil {
		return vm.activeView.InputHandler()(event)
	}

	return event
}

func (vm *Manager) Run(app *ui.App) error {
	vm.App = app
	vm.setupLayout()

	// Set up the app root
	vm.App.SetRoot(vm.layout, true)

	// Enable mouse
	vm.App.EnableMouse(true)

	vm.App.SetInputCapture(vm.globalInputHandler)

	// Run the application
	return vm.App.Run()
}

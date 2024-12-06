package manager

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/services"
	"github.com/tpelletiersophos/cloudcutter/internal/ui"
	components "github.com/tpelletiersophos/cloudcutter/internal/ui/components"
)

// View-related constants
const (
	ViewEC2      = "ec2"
	ViewECR      = "ecr"
	ViewDynamoDB = "dynamodb"
	ViewElastic  = "elastic"
	ViewS3       = "s3"
	ViewTest     = "test"
)

// Component-related constants
const (
	ComponentList       ComponentType = "list"
	ComponentTextView   ComponentType = "textview"
	ComponentFlex       ComponentType = "flex"
	ComponentTable      ComponentType = "table"
	ComponentInputField ComponentType = "inputfield"
	ComponentHelp       ComponentType = "help"
)

// Modal-related constants
const (
	ModalCmdPrompt = "modalPrompt"
	ModalFilter    = "filterModal"
	ModalHelp      = "helpModal"
	ModalProfile   = "profileSelector"
	ModalRegion    = "regionSelector"
)

// Manager represents the main view manager
type Manager struct {
	App            *ui.App
	ctx            context.Context
	cancelFunc     context.CancelFunc
	views          map[string]services.View
	activeView     services.View
	Pages          *tview.Pages
	layout         *tview.Flex
	awsConfig      aws.Config
	primitivesByID map[string]tview.Primitive

	prompt       *components.Prompt
	filterPrompt *components.Prompt
	header       *components.Header
	statusBar    *components.StatusBar
	help         *components.Help

	profileManager *ProfileManager

	StatusChan         chan string
	focusedComponentID string
}

// NewViewManager creates a new view manager instance
func NewViewManager(ctx context.Context, app *ui.App, awsConfig aws.Config) *Manager {
	ctx, cancel := context.WithCancel(ctx)
	vm := &Manager{
		ctx:            ctx,
		cancelFunc:     cancel,
		App:            app,
		views:          make(map[string]services.View),
		Pages:          tview.NewPages(),
		header:         components.NewHeader(),
		statusBar:      components.NewStatusBar(),
		prompt:         components.NewPrompt(),
		filterPrompt:   components.NewPrompt(),
		awsConfig:      awsConfig,
		primitivesByID: make(map[string]tview.Primitive),
		profileManager: NewProfileManager(),
		StatusChan:     make(chan string, 10),
		help:           components.NewHelp(),
	}

	vm.initialize()
	return vm
}

// initialize sets up the manager's initial state
func (vm *Manager) initialize() {
	vm.profileManager.statusChan = vm.StatusChan
	vm.setupLayout()
	vm.setupPrompts()
	vm.startStatusListener()
}

// setupLayout initializes the main layout
func (vm *Manager) setupLayout() {
	vm.layout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(vm.header, 8, 0, false).
		AddItem(vm.Pages, 0, 1, true).
		AddItem(vm.statusBar, 1, 0, false)
}

func (vm *Manager) setupPrompts() {
	vm.prompt.SetDoneFunc(func(command string) {
		if newFocus := vm.handleCommand(command); newFocus != nil {
			vm.Pages.RemovePage(ModalCmdPrompt)
			vm.App.SetFocus(newFocus)
		} else {
			vm.hideModal(ModalCmdPrompt)
		}
	})

	vm.prompt.SetCancelFunc(func() {
		vm.hideModal(ModalCmdPrompt)
	})
}

func (vm *Manager) showModal(name string, content tview.Primitive, width int, height int) {
	modal := vm.createModalFlex(content, width, height)
	vm.Pages.AddPage(name, modal, true, true)
}

func (vm *Manager) hideModal(name string) {
	vm.Pages.RemovePage(name)
	if vm.activeView != nil {
		vm.App.SetFocus(vm.activeView.GetContent())
	}
}

func (vm *Manager) createModalFlex(content tview.Primitive, width int, height int) tview.Primitive {
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(content, width, 0, true).
			AddItem(nil, 0, 1, false),
			height, 1, true).
		AddItem(nil, 0, 1, false)
}

func (vm *Manager) updateConfig(updateFn func(aws.Config) aws.Config) error {
	cfg := updateFn(vm.awsConfig)
	vm.awsConfig = cfg
	return vm.reinitializeViews()
}

func (vm *Manager) handleCommand(command string) (newFocus tview.Primitive) {
	handlers := map[string]func() (tview.Primitive, error){
		"profile":  vm.showProfileSelector,
		"region":   vm.showRegionSelector,
		"ec2":      func() (tview.Primitive, error) { return nil, vm.SwitchToView(ViewEC2) },
		"dynamodb": func() (tview.Primitive, error) { return nil, vm.SwitchToView(ViewDynamoDB) },
		"elastic":  func() (tview.Primitive, error) { return nil, vm.SwitchToView(ViewElastic) },
		"test":     func() (tview.Primitive, error) { return nil, vm.SwitchToView(ViewTest) },
		"help": func() (tview.Primitive, error) {
			vm.statusBar.SetText("Help: List of available commands...")
			return nil, nil
		},
		"exit": func() (tview.Primitive, error) {
			vm.App.Stop()
			return nil, nil
		},
	}

	if handler, exists := handlers[command]; exists {
		if primitive, err := handler(); err != nil {
			vm.statusBar.SetText(fmt.Sprintf("Error executing command: %s", err))
		} else {
			return primitive
		}
	} else {
		vm.statusBar.SetText(fmt.Sprintf("Unknown command: %s", command))
	}

	return nil
}

// Run starts the application
func (vm *Manager) Run() error {
	vm.App.SetRoot(vm.layout, true)
	vm.App.EnableMouse(true)
	vm.App.SetInputCapture(vm.globalInputHandler)
	return vm.App.Run()
}

type ComponentType string

type Style struct {
	Border          bool
	BorderColor     tcell.Color
	Title           string
	TitleAlign      int
	TitleColor      tcell.Color
	BackgroundColor tcell.Color
	TextColor       tcell.Color
}

type Component struct {
	ID         string
	Type       ComponentType
	Direction  int
	FixedSize  int
	Proportion int
	Focus      bool
	Style      Style
	Properties map[string]any
	Children   []Component
	Help       []components.HelpCommand
}

type LayoutConfig struct {
	Title       string
	Components  []Component
	Direction   int
	FixedSizes  []int
	Proportions []int
}

func (vm *Manager) CreateLayout(cfg LayoutConfig) tview.Primitive {
	flex := tview.NewFlex().SetDirection(cfg.Direction)
	for _, comp := range cfg.Components {
		prim := vm.buildPrimitiveFromComponent(comp)
		flex.AddItem(prim, comp.FixedSize, comp.Proportion, comp.Focus)
	}
	return flex
}

func (vm *Manager) buildPrimitiveFromComponent(c Component) tview.Primitive {
	var primitive tview.Primitive

	switch c.Type {
	case ComponentList:
		list := tview.NewList().ShowSecondaryText(false)
		applyStyleToBox(list, c.Style)
		if items, ok := c.Properties["items"].([]string); ok {
			for _, item := range items {
				list.AddItem(item, "", 0, nil)
			}
		}

		var origOnFocus func(*tview.List)
		if onFocus, ok := c.Properties["onFocus"].(func(*tview.List)); ok {
			origOnFocus = onFocus
		}

		list.SetFocusFunc(func() {
			vm.focusedComponentID = c.ID
			if help, ok := c.Properties["help"].(components.HelpCategory); ok {
				vm.help.SetContextHelp(&help)
			} else {
				vm.help.ClearContextHelp()
			}
			if origOnFocus != nil {
				origOnFocus(list)
			}
		})

		if bgColor, ok := c.Properties["selectedBackgroundColor"].(tcell.Color); ok {
			if textColor, ok := c.Properties["selectedTextColor"].(tcell.Color); ok {
				list.SetSelectedStyle(tcell.StyleDefault.Foreground(textColor).Background(bgColor))
			}
		}
		if onBlur, ok := c.Properties["onBlur"].(func(*tview.List)); ok {
			origOnBlur := onBlur
			list.SetBlurFunc(func() {
				if vm.focusedComponentID == c.ID {
					vm.focusedComponentID = ""
					vm.help.ClearContextHelp()
				}
				if origOnBlur != nil {
					origOnBlur(list)
				}
			})
		}

		if onFocus, ok := c.Properties["onFocus"].(func(*tview.List)); ok {
			list.SetFocusFunc(func() { onFocus(list) })
		}
		if onBlur, ok := c.Properties["onBlur"].(func(*tview.List)); ok {
			list.SetBlurFunc(func() { onBlur(list) })
		}
		if onChanged, ok := c.Properties["onChanged"].(func(int, string, string, rune)); ok {
			list.SetChangedFunc(onChanged)
		}
		if onSelected, ok := c.Properties["onSelected"].(func(int, string, string, rune)); ok {
			list.SetSelectedFunc(onSelected)
		}

		primitive = list

	case ComponentTable:
		table := tview.NewTable()
		applyStyleToBox(table, c.Style)

		if bgColor, ok := c.Properties["selectedBackgroundColor"].(tcell.Color); ok {
			if textColor, ok := c.Properties["selectedTextColor"].(tcell.Color); ok {
				table.SetSelectedStyle(tcell.StyleDefault.Foreground(textColor).Background(bgColor))
			}
		}
		if onFocus, ok := c.Properties["onFocus"].(func(*tview.Table)); ok {
			table.SetFocusFunc(func() { onFocus(table) })
		}
		if onBlur, ok := c.Properties["onBlur"].(func(*tview.Table)); ok {
			table.SetBlurFunc(func() { onBlur(table) })
		}
		if onSelected, ok := c.Properties["onSelected"].(func(row, col int)); ok {
			table.SetSelectedFunc(func(row, col int) {
				onSelected(row, col)
			})
		}

		table.SetSelectable(true, false)

		primitive = table

	case ComponentTextView:
		textView := tview.NewTextView()
		applyStyleToBox(textView, c.Style)

		if text, ok := c.Properties["text"].(string); ok {
			textView.SetText(text)
		}
		if wrap, ok := c.Properties["wrap"].(bool); ok {
			textView.SetWrap(wrap)
		}
		if scroll, ok := c.Properties["scrollable"].(bool); ok {
			textView.SetScrollable(scroll)
		}
		if dynamic, ok := c.Properties["dynamicColors"].(bool); ok {
			textView.SetDynamicColors(dynamic)
		}
		if onFocus, ok := c.Properties["onFocus"].(func(*tview.TextView)); ok {
			textView.SetFocusFunc(func() { onFocus(textView) })
		}
		if onBlur, ok := c.Properties["onBlur"].(func(*tview.TextView)); ok {
			textView.SetBlurFunc(func() { onBlur(textView) })
		}

		primitive = textView

	case ComponentFlex:
		flex := tview.NewFlex().SetDirection(c.Direction)
		applyStyleToBox(flex, c.Style)

		if onFocus, ok := c.Properties["onFocus"].(func(*tview.Flex)); ok {
			flex.SetFocusFunc(func() { onFocus(flex) })
		}
		if onBlur, ok := c.Properties["onBlur"].(func(*tview.Flex)); ok {
			flex.SetBlurFunc(func() { onBlur(flex) })
		}

		for _, child := range c.Children {
			childPrimitive := vm.buildPrimitiveFromComponent(child)
			flex.AddItem(childPrimitive, child.FixedSize, child.Proportion, child.Focus)
		}

		primitive = flex

	case ComponentInputField:
		input := tview.NewInputField()
		applyStyleToBox(input, c.Style)

		if label, ok := c.Properties["label"].(string); ok {
			input.SetLabel(label)
		}
		if labelColor, ok := c.Properties["labelColor"].(tcell.Color); ok {
			input.SetLabelColor(labelColor)
		}
		if fieldWidth, ok := c.Properties["fieldWidth"].(int); ok {
			input.SetFieldWidth(fieldWidth)
		}
		if text, ok := c.Properties["text"].(string); ok {
			input.SetText(text)
		}
		if bgColor, ok := c.Properties["fieldBackgroundColor"].(tcell.Color); ok {
			input.SetFieldBackgroundColor(bgColor)
		}
		if textColor, ok := c.Properties["fieldTextColor"].(tcell.Color); ok {
			input.SetFieldTextColor(textColor)
		}

		if doneFunc, ok := c.Properties["doneFunc"].(func(text string)); ok {
			input.SetDoneFunc(func(key tcell.Key) {
				if key == tcell.KeyEnter {
					doneFunc(input.GetText())
				}
			})
		}
		if changedFunc, ok := c.Properties["changedFunc"].(func(text string)); ok {
			input.SetChangedFunc(changedFunc)
		}

		var origOnFocus func(*tview.InputField)
		var origOnBlur func(*tview.InputField)
		if onFocus, ok := c.Properties["onFocus"].(func(*tview.InputField)); ok {
			origOnFocus = onFocus
		}
		if onBlur, ok := c.Properties["onBlur"].(func(*tview.InputField)); ok {
			origOnBlur = onBlur
		}

		input.SetFocusFunc(func() {
			vm.focusedComponentID = c.ID

			// Update help context if help is visible
			if len(c.Help) > 0 {
				helpCategory := &components.HelpCategory{
					Title:    c.Style.Title,
					Commands: c.Help,
				}
				vm.help.SetContextHelp(helpCategory)
			} else {
				vm.help.ClearContextHelp()
			}

			if origOnFocus != nil {
				origOnFocus(input)
			}
		})

		input.SetBlurFunc(func() {
			if vm.focusedComponentID == c.ID {
				vm.focusedComponentID = ""
				// Don't clear context help here anymore
			}
			if origOnBlur != nil {
				origOnBlur(input)
			}
		})

		primitive = input
	case ComponentHelp:
		help := components.NewHelp()
		applyStyleToBox(help, c.Style)

		// Apply any help-specific properties
		if commands, ok := c.Properties["commands"].([]components.HelpCommand); ok {
			help.SetCommands(commands)
		}
		if onFocus, ok := c.Properties["onFocus"].(func(*components.Help)); ok {
			help.SetFocusFunc(onFocus)
		}
		if onBlur, ok := c.Properties["onBlur"].(func(*components.Help)); ok {
			help.SetBlurFunc(onBlur)
		}

		primitive = help
	}
	if c.ID != "" && primitive != nil {
		vm.primitivesByID[c.ID] = primitive
	}

	return primitive
}

func (vm *Manager) RegisterView(view services.View) error {
	if view, exists := vm.views[view.Name()]; exists {
		return fmt.Errorf("view %s already registered", view.Name())
	}
	vm.views[view.Name()] = view
	return nil
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

	if !vm.Pages.HasPage(name) {
		vm.Pages.AddPage(name, view.GetContent(), true, true)
	}
	vm.Pages.SwitchToPage(name)
	vm.header.ClearSummary()
	vm.statusBar.SetText(fmt.Sprintf("Status: %s view active", name))
	vm.header.SetTitle(fmt.Sprintf(" Cloud Cutter - %s ", name)).SetTitleColor(tcell.ColorYellow)

	return nil
}

func (vm *Manager) UpdateHeader(summary []components.SummaryItem) {
	vm.header.UpdateSummary(summary)
}

func (vm *Manager) UpdateStatusBar(text string) {
	vm.statusBar.SetText(text)
}

func (vm *Manager) ViewContext() context.Context {
	return vm.ctx
}

func (vm *Manager) hidePrompt() {
	vm.Pages.RemovePage("modalPrompt")
	vm.App.SetFocus(vm.activeView.GetContent())
}

func (vm *Manager) SetFocus(p tview.Primitive) {
	vm.App.SetFocus(p)
}

func (vm *Manager) IsModalVisible() bool {
	if vm.help.IsVisible() {
		return true
	}

	if page, _ := vm.Pages.GetFrontPage(); page != "" {
		for _, name := range []string{ModalCmdPrompt, ModalFilter, ModalHelp} {
			if page == name {
				return true
			}
		}
	}
	return false
}

func (vm *Manager) hideAllModals() {
	vm.hidePrompt()
	vm.HideFilterPrompt()
	vm.hideProfileSelector()
}

func (vm *Manager) globalInputHandler(event *tcell.EventKey) *tcell.EventKey {
	currentFocus := vm.App.GetFocus()
	if view, ok := vm.activeView.(services.View); ok {
		if view.GetActiveField() == "filterPrompt" {
			return vm.activeView.InputHandler()(event)
		}
	}

	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case '?':
			if !vm.help.IsVisible() {
				vm.help.Show(vm.Pages, func() {
					if vm.activeView != nil {
						vm.App.SetFocus(currentFocus)
					}
				})
				return nil
			}
			return event
		case ':':
			vm.showPrompt()
			return nil
		case '/':
			vm.showFilterPrompt()
			return nil
		}
		if vm.help.IsVisible() {
			return event
		}
	}

	// Delegate to active view if applicable
	if !vm.IsModalVisible() && vm.activeView != nil {
		if handler := vm.activeView.InputHandler(); handler != nil {
			if result := handler(event); result == nil {
				return nil
			}
		}
	}

	if event.Key() == tcell.KeyEsc {
		// Handle escape key hierarchically
		if vm.help.IsVisible() {
			vm.help.Hide(vm.Pages)
			if vm.activeView != nil {
				vm.App.SetFocus(currentFocus)
			}
			return nil
		}
		if vm.Pages.HasPage("modalPrompt") {
			vm.Pages.RemovePage("modalPrompt")
			if vm.activeView != nil {
				vm.App.SetFocus(vm.activeView.GetContent())
			}
			return nil
		}
		if vm.Pages.HasPage("filterModal") {
			vm.HideFilterPrompt()
			return nil
		}
		if vm.Pages.HasPage("profileSelector") {
			vm.hideProfileSelector()
			return nil
		}
		return event
	}
	return event
}

func (vm *Manager) showFilterPrompt() {
	// Store currently focused primitive before changing focus
	previousFocus := vm.App.GetFocus()

	vm.filterPrompt.SetText("")
	vm.filterPrompt.InputField.SetLabel(" > ").SetLabelColor(tcell.ColorTeal)
	vm.filterPrompt.SetTitle(" Filter ")
	vm.filterPrompt.SetBorder(true)
	vm.filterPrompt.SetTitleAlign(tview.AlignLeft)
	vm.filterPrompt.SetBorderColor(tcell.ColorMediumTurquoise)

	modal := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().
				AddItem(nil, 0, 1, false).
				AddItem(vm.filterPrompt, 50, 0, true).
				AddItem(nil, 0, 1, false),
			3, 0, true).
		AddItem(nil, 0, 1, false)

	vm.Pages.AddPage("filterModal", modal, true, true)
	vm.App.SetFocus(vm.filterPrompt.InputField)

	if view, ok := vm.activeView.(interface {
		HandleFilter(*components.Prompt, tview.Primitive)
	}); ok {
		view.HandleFilter(vm.filterPrompt, previousFocus)
	}
}
func (vm *Manager) HideFilterPrompt() {
	vm.Pages.RemovePage("filterModal")
	if vm.activeView != nil {
		vm.App.SetFocus(vm.activeView.GetContent())
	}
}
func (vm *Manager) DetermineDirection(direction int) int {
	// Ensure the direction is either FlexRow or FlexColumn
	if direction == tview.FlexRow || direction == tview.FlexColumn {
		return direction
	}
	return tview.FlexColumn
}

func (vm *Manager) GetPrimitiveByID(id string) tview.Primitive {
	return vm.primitivesByID[id]
}

func applyStyleToBox(box tview.Primitive, style Style) {
	if b, ok := box.(interface {
		SetBorder(bool) *tview.Box
		SetTitle(string) *tview.Box
		SetTitleAlign(int) *tview.Box
		SetTitleColor(tcell.Color) *tview.Box
		SetBorderColor(tcell.Color) *tview.Box
	}); ok {
		if style.Border {
			b.SetBorder(true)
			if style.Title != "" {
				b.SetTitle(style.Title).SetTitleAlign(style.TitleAlign).SetTitleColor(style.TitleColor)
			}
			b.SetBorderColor(style.BorderColor)
		}
	}
}

func (vm *Manager) switchToDevProfile() error {
	if vm.profileManager.IsAuthenticating() {
		status := "Authentication already in progress"
		vm.StatusChan <- status
		return fmt.Errorf(status)
	}

	vm.profileManager.SwitchProfile(vm.ctx, "opal_dev", func(cfg aws.Config, err error) {
		if err != nil {
			vm.StatusChan <- fmt.Sprintf("Failed to switch to dev profile: %v", err)
			return
		}

		vm.awsConfig = cfg
		vm.header.UpdateEnvVar("Profile", "opal_dev")
		if err := vm.reinitializeViews(); err != nil {
			vm.StatusChan <- fmt.Sprintf("Error reinitializing views: %v", err)
			return
		}

		vm.StatusChan <- "Successfully switched to dev profile"
	})

	return nil
}

func (vm *Manager) switchToProdProfile() error {
	if vm.profileManager.IsAuthenticating() {
		status := "Authentication already in progress"
		vm.StatusChan <- status
		return fmt.Errorf(status)
	}

	vm.profileManager.SwitchProfile(vm.ctx, "opal_prod", func(cfg aws.Config, err error) {
		if err != nil {
			vm.StatusChan <- fmt.Sprintf("Failed to switch to prod profile: %v", err)
			return
		}

		vm.awsConfig = cfg
		vm.header.UpdateEnvVar("Profile", "opal_prod")
		if err := vm.reinitializeViews(); err != nil {
			vm.StatusChan <- fmt.Sprintf("Error reinitializing views: %v", err)
			return
		}

		vm.StatusChan <- "Successfully switched to prod profile"
		return
	})

	return nil
}

func (vm *Manager) reinitializeViews() error {
	currentViewName := ""
	if vm.activeView != nil {
		currentViewName = vm.activeView.Name()
	}

	// Reinitialize all views with new config
	for _, view := range vm.views {
		if reinitView, ok := view.(services.Reinitializer); ok {
			reinitView.Reinitialize(vm.awsConfig)
		}
	}

	if currentViewName != "" {
		return vm.SwitchToView(currentViewName)
	}

	return nil
}

func (vm *Manager) showProfileSelector() (tview.Primitive, error) {
	profileSelector := components.NewProfileSelector(
		func(profile string) {
			vm.Pages.RemovePage("profileSelector")
			vm.App.SetFocus(vm.activeView.GetContent())

			vm.statusBar.SetText(fmt.Sprintf("Switching to %s profile...", profile))

			if profile == "opal_dev" {
				vm.switchToDevProfile()
			} else if profile == "opal_prod" {
				vm.switchToProdProfile()
			}
		},
		func() {
			vm.Pages.RemovePage("profileSelector")
			vm.App.SetFocus(vm.activeView.GetContent())
		},
	)

	numEntries := profileSelector.GetItemCount() + 2
	modal := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(profileSelector, 30, 0, true).
			AddItem(nil, 0, 1, false),
			numEntries, 1, true).
		AddItem(nil, 0, 1, false)

	vm.Pages.AddPage("profileSelector", modal, true, true)
	return profileSelector, nil
}

func (vm *Manager) hideProfileSelector() {
	vm.Pages.RemovePage("profileSelector")
	if vm.activeView != nil {
		vm.App.SetFocus(vm.activeView.GetContent())
	}
}

func (vm *Manager) hideHelp() {
	vm.Pages.RemovePage(ModalHelp)
	if vm.activeView != nil {
		vm.App.SetFocus(vm.activeView.GetContent())
	}
}

func (vm *Manager) HideAllModals() {
	vm.hideAllModals()
}

func (vm *Manager) startStatusListener() {
	go func() {
		for {
			select {
			case <-vm.ctx.Done():
				return
			case status := <-vm.StatusChan:
				vm.statusBar.SetText(status)
				vm.App.Draw()
			}
		}
	}()
}

func (vm *Manager) ShowFilterPrompt(modal tview.Primitive) {
	vm.Pages.AddPage("filterModal", modal, true, true)
}
func (vm *Manager) UpdateRegion(region string) error {
	cfg := vm.awsConfig.Copy()
	cfg.Region = region

	vm.awsConfig = cfg

	vm.header.UpdateEnvVar("Region", region)

	if err := vm.reinitializeViews(); err != nil {
		vm.StatusChan <- fmt.Sprintf("Error reinitializing views with new region: %v", err)
		return err
	}

	vm.StatusChan <- fmt.Sprintf("Successfully switched to region: %s", region)
	return nil
}

func (vm *Manager) showRegionSelector() (tview.Primitive, error) {
	regionSelector := components.NewRegionSelector(
		func(region string) {
			vm.Pages.RemovePage("regionSelector")
			vm.App.SetFocus(vm.activeView.GetContent())

			vm.statusBar.SetText(fmt.Sprintf("Switching to region %s...", region))

			cfg := vm.awsConfig.Copy()
			cfg.Region = region
			vm.awsConfig = cfg

			vm.header.UpdateEnvVar("Region", region)

			if err := vm.reinitializeViews(); err != nil {
				vm.StatusChan <- fmt.Sprintf("Error reinitializing views: %v", err)
				return
			}

			vm.StatusChan <- fmt.Sprintf("Successfully switched to region: %s", region)
		},
		func() {
			vm.Pages.RemovePage("regionSelector")
			vm.App.SetFocus(vm.activeView.GetContent())
		},
	)

	numEntries := regionSelector.GetItemCount() + 2
	modal := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(regionSelector, 30, 0, true).
			AddItem(nil, 0, 1, false),
			numEntries, 1, true).
		AddItem(nil, 0, 1, false)

	vm.Pages.AddPage("regionSelector", modal, true, true)
	return regionSelector, nil
}

func (vm *Manager) hideRegionSelector() {
	vm.Pages.RemovePage("regionSelector")
	if vm.activeView != nil {
		vm.App.SetFocus(vm.activeView.GetContent())
	}
}

func (vm *Manager) showPrompt() {
	vm.prompt.InputField.SetLabel(" > ")
	vm.prompt.InputField.SetLabelColor(tcell.ColorTeal)
	vm.prompt.SetTitle(" Command ")
	vm.prompt.SetBorder(true)
	vm.prompt.SetTitleAlign(tview.AlignLeft)
	vm.prompt.SetBorderColor(tcell.ColorMediumTurquoise)

	vm.App.SetFocus(vm.prompt.InputField)

	vm.showModal(ModalCmdPrompt, vm.prompt, 50, 3)
}

func (vm *Manager) CurrentProfile() string {
	return vm.profileManager.currentProfile
}

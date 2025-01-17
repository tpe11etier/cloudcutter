package manager

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/logger"
	"github.com/tpelletiersophos/cloudcutter/internal/ui"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/header"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/profile"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/region"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/spinner"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/statusbar"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/components/types"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/help"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/style"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/views"
	"strings"
)

const (
	ViewDynamoDB   = "dynamodb"
	ViewElastic    = "elastic"
	ModalCmdPrompt = "modalPrompt"
	ModalJSON      = "modalJSON"
)

// Manager represents the main view manager
type Manager struct {
	app               *ui.App
	ctx               context.Context
	cancelFunc        context.CancelFunc
	views             map[string]views.View
	lazyViews         map[string]func() (views.View, error)
	activeView        views.View
	pages             *tview.Pages
	layout            *tview.Flex
	awsConfig         aws.Config
	primitivesByID    map[string]tview.Primitive
	logger            *logger.Logger
	spinner           *spinner.Spinner
	loadingCancelFunc context.CancelFunc

	prompt       *components.Prompt
	filterPrompt *components.Prompt
	header       *header.Header
	statusBar    *statusbar.StatusBar
	help         *help.Help

	StatusChan         chan string
	focusedComponentID string
	profileHandler     *profile.Handler
}

func (vm *Manager) Pages() *tview.Pages {
	return vm.pages
}

func (vm *Manager) App() *tview.Application {
	return vm.app.Application
}

func (vm *Manager) ActiveView() tview.Primitive {
	return vm.activeView.Content()
}

func NewViewManager(ctx context.Context, app *ui.App, awsConfig aws.Config, log *logger.Logger) *Manager {
	ctx, cancel := context.WithCancel(ctx)
	vm := &Manager{
		ctx:            ctx,
		cancelFunc:     cancel,
		app:            app,
		views:          make(map[string]views.View),
		pages:          tview.NewPages(),
		header:         header.NewHeader(),
		statusBar:      statusbar.NewStatusBar(),
		prompt:         components.NewPrompt(),
		filterPrompt:   components.NewPrompt(),
		awsConfig:      awsConfig,
		primitivesByID: make(map[string]tview.Primitive),
		StatusChan:     make(chan string, 10),
		help:           help.NewHelp(),
		logger:         log,
	}

	vm.profileHandler = profile.NewProfileHandler(
		vm.StatusChan,
		func(message string) {
			vm.pages.RemovePage("profileSelector")
			vm.showLoading(message)
		},
		func() { vm.hideLoading() },
	)

	vm.initialize()
	return vm
}

func (vm *Manager) initialize() {
	vm.setupLayout()
	vm.setupPrompts()
	vm.startStatusListener()
}

func (vm *Manager) setupLayout() {
	vm.layout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(vm.header, 8, 0, false).
		AddItem(vm.pages, 0, 1, true).
		AddItem(vm.statusBar, 1, 0, false)
}

func (vm *Manager) setupPrompts() {
	vm.prompt.SetAutocompleteFunc(func(input string) []string {
		if input == "" {
			return nil
		}
		commands := []string{"profile", "region", "dynamodb", "elastic", "help", "exit"}
		var matches []string
		for _, cmd := range commands {
			if strings.HasPrefix(cmd, input) {
				matches = append(matches, cmd)
			}
		}
		return matches
	})

	vm.prompt.InputField.SetFieldBackgroundColor(tcell.ColorBlack)
	vm.prompt.InputField.SetFieldTextColor(tcell.ColorBeige)

	vm.prompt.SetAutocompleteStyles(
		tcell.ColorBlack,
		tcell.StyleDefault.Foreground(tcell.ColorBeige),
		tcell.StyleDefault.Background(tcell.ColorDarkCyan).Foreground(tcell.ColorBeige))

	vm.prompt.SetDoneFunc(func(command string) {
		if newFocus := vm.handleCommand(command); newFocus != nil {
			vm.pages.RemovePage(types.ModalCmdPrompt)
			vm.app.SetFocus(newFocus)
		} else {
			vm.HideModal(types.ModalCmdPrompt)
		}
		vm.prompt.InputField.SetText("")
	})

	vm.prompt.SetCancelFunc(func() {
		vm.HideModal(ModalCmdPrompt)
		vm.prompt.InputField.SetText("")
		vm.HideModal(types.ModalCmdPrompt)
	})
}

func (vm *Manager) showModal(name string, content tview.Primitive, width int, height int) {
	modal := vm.createModalFlex(content, width, height)
	vm.pages.AddPage(name, modal, true, true)
}

func (vm *Manager) HideModal(name string) {
	vm.pages.RemovePage(name)
	if vm.activeView != nil {
		vm.app.SetFocus(vm.activeView.Content())
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

func (vm *Manager) handleCommand(command string) (newFocus tview.Primitive) {
	handlers := map[string]func() (tview.Primitive, error){
		"profile":  vm.showProfileSelector,
		"region":   vm.showRegionSelector,
		"dynamodb": func() (tview.Primitive, error) { return nil, vm.SwitchToView(ViewDynamoDB) },
		"elastic":  func() (tview.Primitive, error) { return nil, vm.SwitchToView(ViewElastic) },
		"help": func() (tview.Primitive, error) {
			vm.statusBar.SetText("Help: List of available commands...")
			return nil, nil
		},
		"exit": func() (tview.Primitive, error) {
			vm.app.Stop()
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
	vm.app.SetRoot(vm.layout, true)
	vm.app.EnableMouse(true)
	vm.app.SetInputCapture(vm.globalInputHandler)
	return vm.app.Run()
}

func (vm *Manager) CreateLayout(cfg types.LayoutConfig) tview.Primitive {
	flex := tview.NewFlex().SetDirection(cfg.Direction)
	for _, comp := range cfg.Components {
		prim := vm.buildPrimitiveFromComponent(comp)
		flex.AddItem(prim, comp.FixedSize, comp.Proportion, comp.Focus)
	}
	return flex
}

func (vm *Manager) buildPrimitiveFromComponent(c types.Component) tview.Primitive {
	var primitive tview.Primitive

	switch c.Type {
	case types.ComponentList:
		list := tview.NewList().ShowSecondaryText(false)

		if style, ok := c.Style.(types.ListStyle); ok {
			applyStyleToBox(list, style.BaseStyle)
			list.SetSelectedStyle(tcell.StyleDefault.
				Foreground(style.SelectedTextColor).
				Background(style.SelectedBackgroundColor))
			list.SetMainTextColor(style.TextColor).
				SetTitleColor(style.TitleColor)
		}

		if props, ok := c.Properties.(types.ListProperties); ok {
			for _, item := range props.Items {
				list.AddItem(item, "", 0, nil)
			}

			list.SetFocusFunc(func() {
				vm.focusedComponentID = c.ID
				if len(c.Help) > 0 {
					helpCategory := &help.HelpCategory{
						Title:    c.ID,
						Commands: c.Help,
					}
					vm.help.SetContextHelp(helpCategory)
				} else {
					vm.help.ClearContextHelp()
				}
				if props.OnFocus != nil {
					props.OnFocus(list)
				}
			})

			list.SetBlurFunc(func() {
				if vm.focusedComponentID == c.ID {
					vm.focusedComponentID = ""
					vm.help.ClearContextHelp()
				}
				if props.OnBlur != nil {
					props.OnBlur(list)
				}
			})

			if props.OnChanged != nil {
				list.SetChangedFunc(props.OnChanged)
			}
			if props.OnSelected != nil {
				list.SetSelectedFunc(props.OnSelected)
			}
		}

		primitive = list

	case types.ComponentTable:
		table := tview.NewTable()
		if style, ok := c.Style.(types.TableStyle); ok {
			applyStyleToBox(table, style.BaseStyle)
			table.SetSelectedStyle(tcell.StyleDefault.
				Foreground(style.SelectedTextColor).
				Background(style.SelectedBackgroundColor))
		}

		if props, ok := c.Properties.(types.TableProperties); ok {
			if props.OnFocus != nil {
				table.SetFocusFunc(func() { props.OnFocus(table) })
			}
			if props.OnBlur != nil {
				table.SetBlurFunc(func() { props.OnBlur(table) })
			}
			if props.OnSelected != nil {
				table.SetSelectedFunc(props.OnSelected)
			}
		}

		table.SetSelectable(true, false)
		primitive = table

	case types.ComponentTextView:
		textView := tview.NewTextView()
		if style, ok := c.Style.(types.TextViewStyle); ok {
			applyStyleToBox(textView, style.BaseStyle)
		}

		if props, ok := c.Properties.(types.TextViewProperties); ok {
			textView.SetText(props.Text)
			textView.SetWrap(props.Wrap)
			textView.SetScrollable(props.Scrollable)
			textView.SetDynamicColors(props.DynamicColors)

			if props.OnFocus != nil {
				textView.SetFocusFunc(func() { props.OnFocus(textView) })
			}
			if props.OnBlur != nil {
				textView.SetBlurFunc(func() { props.OnBlur(textView) })
			}
		}
		primitive = textView

	case types.ComponentFlex:
		flex := tview.NewFlex().SetDirection(c.Direction)
		if style, ok := c.Style.(types.FlexStyle); ok {
			applyStyleToBox(flex, style.BaseStyle)
		}

		if props, ok := c.Properties.(types.FlexProperties); ok {
			if props.OnFocus != nil {
				flex.SetFocusFunc(func() { props.OnFocus(flex) })
			}
			if props.OnBlur != nil {
				flex.SetBlurFunc(func() { props.OnBlur(flex) })
			}
		}

		for _, child := range c.Children {
			childPrimitive := vm.buildPrimitiveFromComponent(child)
			flex.AddItem(childPrimitive, child.FixedSize, child.Proportion, child.Focus)
		}
		primitive = flex

	case types.ComponentInputField:
		input := tview.NewInputField()
		if style, ok := c.Style.(types.InputFieldStyle); ok {
			applyStyleToBox(input, style.BaseStyle)
			input.SetLabelColor(style.LabelColor)
			input.SetFieldBackgroundColor(style.FieldBackgroundColor)
			input.SetFieldTextColor(style.FieldTextColor)
		}

		if props, ok := c.Properties.(types.InputFieldProperties); ok {
			input.SetLabel(props.Label)
			input.SetFieldWidth(props.FieldWidth)
			input.SetText(props.Text)

			if props.DoneFunc != nil {
				input.SetDoneFunc(func(key tcell.Key) {
					if key == tcell.KeyEnter {
						props.DoneFunc(input.GetText())
					}
				})
			}
			if props.ChangedFunc != nil {
				input.SetChangedFunc(props.ChangedFunc)
			}

			input.SetFocusFunc(func() {
				vm.focusedComponentID = c.ID
				if len(c.Help) > 0 {
					helpCategory := &help.HelpCategory{
						Title:    c.ID,
						Commands: c.Help,
					}
					vm.help.SetContextHelp(helpCategory)
					if c.HelpProps != nil {
						// Set all properties at once
						c.HelpProps.Commands = c.Help
						vm.help.SetProperties(*c.HelpProps)
					} else {
						vm.help.SetProperties(help.HelpProperties{
							Commands: c.Help,
						})
					}
				}
				if props.OnFocus != nil {
					props.OnFocus(input)
				}
			})

			input.SetBlurFunc(func() {
				if props.OnBlur != nil {
					props.OnBlur(input)
				}
			})
		}
		primitive = input

	}
	if c.ID != "" && primitive != nil {
		vm.primitivesByID[c.ID] = primitive
	}

	return primitive
}

func (vm *Manager) RegisterView(view views.View) error {
	if _, exists := vm.views[view.Name()]; exists {
		return fmt.Errorf("view %s already registered", view.Name())
	}
	vm.views[view.Name()] = view
	return nil
}

func (vm *Manager) SwitchToView(name string) error {
	if vm.spinner == nil {
		vm.logger.Info("Spinner not initialized in SwitchToView!")
	}

	vm.showLoading("Switching view...")
	vm.UpdateHeader(nil)
	if view, exists := vm.views[name]; exists {
		// View already exists; switch directly
		vm.logger.Debug("Switching to existing view", "view", name)
		vm.setActiveView(view)
	} else if constructor, exists := vm.lazyViews[name]; exists {
		// Lazy view; construct it
		vm.logger.Debug("Initializing lazy view", "view", name)
		go func() {
			view, err := constructor()
			vm.App().QueueUpdateDraw(func() {
				if err != nil {
					vm.logger.Error("Failed to initialize lazy view", "view", name, "error", err)
					vm.hideLoading()
					return
				}
				vm.views[name] = view
				vm.setActiveView(view)
				vm.hideLoading()
			})
		}()
	} else {
		// View doesn't exist
		vm.logger.Error("View not found", "view", name)
		vm.hideLoading()
		return fmt.Errorf("view %s not found", name)
	}

	vm.logger.Info("SwitchToView completed successfully")
	return nil
}

func (vm *Manager) UpdateHeader(summary []types.SummaryItem) {
	vm.header.UpdateSummary(summary)
}

func (vm *Manager) UpdateStatusBar(text string) {
	vm.statusBar.SetText(text)
}

func (vm *Manager) ViewContext() context.Context {
	return vm.ctx
}

func (vm *Manager) hidePrompt() {
	vm.pages.RemovePage(types.ModalCmdPrompt)
	vm.app.SetFocus(vm.activeView.Content())
}

func (vm *Manager) SetFocus(p tview.Primitive) {
	vm.app.SetFocus(p)
}

func (vm *Manager) IsModalVisible() bool {
	if vm.help.IsVisible() {
		return true
	}

	if page, _ := vm.pages.GetFrontPage(); page != "" {
		for _, name := range []string{types.ModalCmdPrompt, types.ModalFilter, help.ModalHelp} {
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
	vm.hideHelp()
	vm.hideJSON()
	vm.hideRowDetails()
}

func (vm *Manager) globalInputHandler(event *tcell.EventKey) *tcell.EventKey {
	currentFocus := vm.app.GetFocus()

	if event.Key() == tcell.KeyEsc {
		if vm.pages.HasPage(types.ModalCmdPrompt) {
			vm.pages.RemovePage(types.ModalCmdPrompt)
			vm.app.SetFocus(vm.activeView.Content())
			return nil
		}
		if vm.pages.HasPage(types.ModalFilter) {
			vm.HideFilterPrompt()
			return nil
		}
		if vm.pages.HasPage("profileSelector") {
			vm.hideProfileSelector()
			return nil
		}
		if vm.help.IsVisible() {
			vm.help.Hide(vm.pages)
			vm.app.SetFocus(currentFocus)
			return nil
		}
	}

	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case '?':
			if !vm.help.IsVisible() {
				vm.help.Show(vm.pages, func() {
					if vm.activeView != nil {
						vm.app.SetFocus(currentFocus)
					}
				})
				return nil
			}
			return event
		case ':':
			vm.showCmdPrompt()
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

	return event
}

func (vm *Manager) switchToDevProfile() error {
	if vm.profileHandler.IsAuthenticating() {
		status := "Authentication already in progress"
		vm.StatusChan <- status
		return fmt.Errorf(status)
	}

	vm.profileHandler.SwitchProfile(vm.ctx, "opal_dev", func(cfg aws.Config, err error) {
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

func (vm *Manager) switchToLocalProfile() error {
	if vm.profileHandler.IsAuthenticating() {
		status := "Authentication already in progress"
		vm.StatusChan <- status
		return fmt.Errorf(status)
	}

	vm.profileHandler.SwitchProfile(vm.ctx, "local", func(cfg aws.Config, err error) {
		if err != nil {
			vm.StatusChan <- fmt.Sprintf("Failed to switch to local profile: %v", err)
			return
		}

		vm.awsConfig = cfg
		vm.header.UpdateEnvVar("Profile", "local")
		if err := vm.reinitializeViews(); err != nil {
			vm.StatusChan <- fmt.Sprintf("Error reinitializing views: %v", err)
			return
		}

		vm.StatusChan <- "Successfully switched to local profile"
	})

	return nil
}

func (vm *Manager) reinitializeViews() error {
	currentViewName := ""
	if vm.activeView != nil {
		currentViewName = vm.activeView.Name()
	}

	var reinitErrors []error

	for name, view := range vm.views {
		if reinitView, ok := view.(views.Reinitializer); ok {
			if err := reinitView.Reinitialize(vm.awsConfig); err != nil {
				reinitErrors = append(reinitErrors, fmt.Errorf("failed to reinitialize %s view: %w", name, err))
			}
		}
	}

	if len(reinitErrors) > 0 {
		return fmt.Errorf("reinitialization errors: %v", reinitErrors)
	}

	if currentViewName != "" {
		return vm.SwitchToView(currentViewName)
	}

	return nil
}

func (vm *Manager) showProfileSelector() (tview.Primitive, error) {
	profileSelector := profile.NewSelector(
		vm.profileHandler,
		func(profile string) {
			vm.app.SetFocus(vm.activeView.Content())

			vm.statusBar.SetText(fmt.Sprintf("Switching to %s profile...", profile))

			switch profile {
			case "opal_dev":
				vm.switchToDevProfile()
			case "opal_prod":
				vm.switchToProdProfile()
			case "local":
				vm.switchToLocalProfile()
			}
		},
		func() {
			vm.pages.RemovePage("profileSelector")
			vm.app.SetFocus(vm.activeView.Content())
		},
		vm.statusBar,
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

	vm.pages.AddPage("profileSelector", modal, true, true)
	return profileSelector, nil
}

func (vm *Manager) hideProfileSelector() {
	vm.pages.RemovePage("profileSelector")
	if vm.activeView != nil {
		vm.app.SetFocus(vm.activeView.Content())
	}
}

func (vm *Manager) hideHelp() {
	vm.pages.RemovePage(help.ModalHelp)
	if vm.activeView != nil {
		vm.app.SetFocus(vm.activeView.Content())
	}
}

func (vm *Manager) hideJSON() {
	vm.pages.RemovePage(ModalJSON)
}

func (vm *Manager) hideRowDetails() {
	vm.pages.RemovePage(types.ModalRowDetails)

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
				vm.app.Draw()
			}
		}
	}()
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

func (vm *Manager) showCmdPrompt() {
	vm.prompt.InputField.SetLabel(" > ")
	vm.prompt.InputField.SetLabelColor(tcell.ColorTeal)
	vm.prompt.SetTitle(" Command ")
	vm.prompt.SetTitleColor(style.GruvboxMaterial.Yellow)
	vm.prompt.SetBorder(true)
	vm.prompt.SetTitleAlign(tview.AlignLeft)
	vm.prompt.SetBorderColor(tcell.ColorMediumTurquoise)

	vm.app.SetFocus(vm.prompt.InputField)

	vm.showModal(types.ModalCmdPrompt, vm.prompt, 50, 3)
}

func (vm *Manager) CurrentProfile() string {
	return vm.profileHandler.GetCurrentProfile()
}

func (vm *Manager) HideFilterPrompt() {
	vm.pages.RemovePage(types.ModalFilter)
	if vm.activeView != nil {
		vm.app.SetFocus(vm.activeView.Content())
	}
}

func (vm *Manager) GetPrimitiveByID(id string) tview.Primitive {
	return vm.primitivesByID[id]
}

func applyStyleToBox(box tview.Primitive, style types.BaseStyle) {
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

func (vm *Manager) showRegionSelector() (tview.Primitive, error) {
	regionSelector := region.NewRegionSelector(
		// First argument: func
		func(region string) {
			vm.statusBar.SetText(fmt.Sprintf("Switching to region %s...", region))
			if err := vm.UpdateRegion(region); err != nil {
				vm.StatusChan <- fmt.Sprintf("Error switching region: %v", err)
			} else {
				vm.StatusChan <- fmt.Sprintf("Successfully switched to region: %s", region)
			}
		},
		// Second argument: func()
		func() {
			vm.pages.RemovePage("regionSelector")
			vm.app.SetFocus(vm.activeView.Content())
		},
		// Third argument: *statusbar.StatusBar
		vm.statusBar,
		vm, // Fourth argument: ManagerInterface
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

	vm.pages.AddPage("regionSelector", modal, true, true)
	return regionSelector, nil
}

func (vm *Manager) hideRegionSelector() {
	vm.pages.RemovePage("regionSelector")
	if vm.activeView != nil {
		vm.app.SetFocus(vm.activeView.Content())
	}
}

func (vm *Manager) Help() *help.Help {
	return vm.help
}

func (vm *Manager) Logger() *logger.Logger {
	return vm.logger
}

func (vm *Manager) RegisterLazyView(name string, constructor func() (views.View, error)) {
	if vm.lazyViews == nil {
		vm.lazyViews = make(map[string]func() (views.View, error))
	}
	vm.lazyViews[name] = constructor
}

func (vm *Manager) setActiveView(view views.View) {
	if vm.activeView != nil {
		vm.activeView.Hide()
	}
	vm.activeView = view
	view.Show()
	vm.pages.SwitchToPage(view.Name())
}

func (vm *Manager) showLoading(message string) {
	if vm.spinner == nil {
		vm.spinner = spinner.NewSpinner(message)
		vm.spinner.SetOnComplete(func() {
			vm.pages.RemovePage("loading")
			if vm.loadingCancelFunc != nil {
				vm.loadingCancelFunc()
				vm.loadingCancelFunc = nil
			}
		})
	} else {
		vm.spinner.SetMessage(message)
	}

	if !vm.spinner.IsLoading() {
		// Create a new context with cancellation
		var ctx context.Context
		ctx, vm.loadingCancelFunc = context.WithCancel(vm.ctx)

		modal := spinner.CreateSpinnerModal(vm.spinner)
		vm.pages.AddPage("loading", modal, true, true)
		vm.app.SetFocus(modal)

		// Start the spinner with the cancellable context
		vm.spinner.StartWithContext(ctx, vm.App())
	}
}

func (vm *Manager) hideLoading() {
	if vm.spinner != nil {
		if vm.loadingCancelFunc != nil {
			vm.loadingCancelFunc()
			vm.loadingCancelFunc = nil
		}
		vm.spinner.Stop()
	}
}

func (vm *Manager) switchToProdProfile() error {
	if vm.profileHandler.IsAuthenticating() {
		status := "Authentication already in progress"
		vm.StatusChan <- status
		return fmt.Errorf(status)
	}

	// Hide modal immediately
	vm.hideProfileSelector()

	vm.profileHandler.SwitchProfile(vm.ctx, "opal_prod", func(cfg aws.Config, err error) {
		if err != nil {
			vm.StatusChan <- fmt.Sprintf("Failed to switch to prod profile: %v", err)
			return
		}

		vm.showLoading("Authenticating with prod profile...")
		vm.awsConfig = cfg
		vm.header.UpdateEnvVar("Profile", "opal_prod")

		if vm.spinner == nil {
			vm.spinner = spinner.NewSpinner("Loading Available Fields...")
			vm.spinner.SetOnComplete(func() {
				vm.pages.RemovePage("loading")
				if vm.loadingCancelFunc != nil {
					vm.loadingCancelFunc()
					vm.loadingCancelFunc = nil
				}
			})
		} else {
			vm.spinner.SetMessage("Loading Available Fields...")
		}

		if !vm.spinner.IsLoading() {
			var ctx context.Context
			ctx, vm.loadingCancelFunc = context.WithCancel(vm.ctx)

			modal := spinner.CreateSpinnerModal(vm.spinner)
			vm.pages.AddPage("loading", modal, true, true)
			vm.app.SetFocus(modal)

			vm.spinner.StartWithContext(ctx, vm.App())
		}

		if err := vm.reinitializeViews(); err != nil {
			vm.StatusChan <- fmt.Sprintf("Error reinitializing views: %v", err)
			return
		}

		vm.StatusChan <- "Successfully switched to prod profile"
	})

	return nil
}

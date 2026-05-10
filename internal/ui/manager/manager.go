package manager

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpe11etier/cloudcutter/internal/auth"
	"github.com/tpe11etier/cloudcutter/internal/environments"
	"github.com/tpe11etier/cloudcutter/internal/logger"
	"github.com/tpe11etier/cloudcutter/internal/ui"
	"github.com/tpe11etier/cloudcutter/internal/ui/components"
	"github.com/tpe11etier/cloudcutter/internal/ui/components/header"
	"github.com/tpe11etier/cloudcutter/internal/ui/components/profile"
	"github.com/tpe11etier/cloudcutter/internal/ui/components/region"
	"github.com/tpe11etier/cloudcutter/internal/ui/components/spinner"
	"github.com/tpe11etier/cloudcutter/internal/ui/components/statusbar"
	"github.com/tpe11etier/cloudcutter/internal/ui/components/types"
	"github.com/tpe11etier/cloudcutter/internal/ui/help"
	"github.com/tpe11etier/cloudcutter/internal/ui/style"
	"github.com/tpe11etier/cloudcutter/internal/ui/views"
)

const (
	ViewDynamoDB = "dynamodb"
	ViewElastic  = "elastic"
	ModalJSON    = "modalJSON"
)

type Manager struct {
	app               *ui.App
	ctx               context.Context
	cancelFunc        context.CancelFunc
	views             map[string]views.View
	lazyViews         map[string]func() (views.View, error)
	activeView        views.View
	pages             *tview.Pages
	layout            *tview.Flex
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
	resolver           *environments.Resolver
}

// CurrentSession returns the active auth session (nil before first profile
// switch). Lets views read profile-specific state — e.g. the Dragos JWT/base
// URL that doesn't fit into aws.Config.
func (vm *Manager) CurrentSession() *auth.Session {
	if vm.profileHandler == nil {
		return nil
	}
	return vm.profileHandler.CurrentSession()
}

// focusActiveView focuses the active view's content if there is one. No-op
// before the first profile is selected (when activeView is nil).
func (vm *Manager) focusActiveView() {
	if vm.activeView == nil {
		return
	}
	vm.app.SetFocus(vm.activeView.Content())
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

// Resolver returns the environments.Resolver. May be nil if the binary was
// constructed without one (only happens in tests today).
func (vm *Manager) Resolver() *environments.Resolver {
	return vm.resolver
}

func NewViewManager(ctx context.Context, app *ui.App, log *logger.Logger, resolver *environments.Resolver) *Manager {
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
		primitivesByID: make(map[string]tview.Primitive),
		StatusChan:     make(chan string, 10),
		help:           help.NewHelp(),
		logger:         log,
		resolver:       resolver,
	}

	var err error
	vm.profileHandler, err = profile.NewProfileHandler(
		vm.StatusChan,
		func(message string) {
			vm.pages.RemovePage("profileSelector")
			vm.showLoading(message)
		},
		func() { vm.hideLoading() },
	)
	if err != nil {
		vm.logger.Error("Failed to initialize profile handler", "error", err)
	}

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
		vm.HideModal(types.ModalCmdPrompt)
		vm.prompt.InputField.SetText("")
	})
}

func (vm *Manager) showModal(name string, content tview.Primitive, width int, height int) {
	modal := vm.createModalFlex(content, width, height)
	vm.pages.AddPage(name, modal, true, true)
}

func (vm *Manager) HideModal(name string) {
	vm.pages.RemovePage(name)
	if vm.activeView != nil {
		vm.focusActiveView()
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
		"profile":  vm.ShowProfileSelector,
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
	vm.focusActiveView()
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
			if vm.activeView != nil {
				vm.focusActiveView()
			}
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
		if vm.pages.HasPage("regionSelector") {
			vm.hideRegionSelector()
			return nil
		}

		if vm.pages.HasPage(ModalJSON) {
			vm.hideJSON()
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

func (vm *Manager) reinitializeViews() error {
	if vm.activeView != nil {
		activeName := vm.activeView.Name()
		if reinitView, ok := vm.activeView.(views.Reinitializer); ok {
			if err := reinitView.Reinitialize(); err != nil {
				return fmt.Errorf("failed to reinitialize %s view: %w", activeName, err)
			}
		}
	}
	return nil
}

func (vm *Manager) ShowProfileSelector() (tview.Primitive, error) {
	profileSelector := profile.NewSelector(
		vm.profileHandler,
		func(name string) {
			if vm.activeView != nil {
				vm.focusActiveView()
			}
			vm.statusBar.SetText(fmt.Sprintf("Switching to %s...", name))
			vm.switchToEnvironment(name)
		},
		func() {
			vm.pages.RemovePage("profileSelector")
			if vm.activeView != nil {
				vm.focusActiveView()
			}
		},
		vm.statusBar,
		vm,
	)

	return profileSelector.ShowSelector()
}

func (vm *Manager) hideProfileSelector() {
	vm.pages.RemovePage("profileSelector")
	if vm.activeView != nil {
		vm.focusActiveView()
	}
}

func (vm *Manager) hideHelp() {
	vm.pages.RemovePage(help.ModalHelp)
	if vm.activeView != nil {
		vm.focusActiveView()
	}
}

func (vm *Manager) hideJSON() {
	vm.pages.RemovePage(ModalJSON)
	if resultsTable := vm.GetPrimitiveByID("resultsTable"); resultsTable != nil {
		vm.app.SetFocus(resultsTable)
	}
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

// UpdateRegion re-materializes the active environment against the new region.
// For non-AWS environments the re-switch is a cheap no-op aside from the
// header label change.
func (vm *Manager) UpdateRegion(region string) {
	vm.profileHandler.SetRegion(region)
	vm.header.UpdateEnvVar("Region", region)

	sess := vm.profileHandler.CurrentSession()
	if sess != nil {
		vm.switchToEnvironment(sess.Profile)
		vm.StatusChan <- fmt.Sprintf("Switched region to %s", region)
	}
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

func (vm *Manager) HideFilterPrompt() {
	vm.pages.RemovePage(types.ModalFilter)
	if vm.activeView != nil {
		vm.focusActiveView()
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
		func(region string) {
			vm.pages.RemovePage("regionSelector")
			vm.focusActiveView()
			vm.UpdateRegion(region)
		},
		func() {
			vm.pages.RemovePage("regionSelector")
			vm.focusActiveView()
		},
		vm,
	)

	return regionSelector.ShowRegionSelector()
}

func (vm *Manager) hideRegionSelector() {
	vm.pages.RemovePage("regionSelector")
	if vm.activeView != nil {
		vm.focusActiveView()
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
		vm.logger.Info("Loading stopped, current focus:", "primitive", vm.app.GetFocus())
	}
}

func (vm *Manager) UpdateViewCommands(commands []header.ViewCommands) {
	vm.header.SetViewCommands(commands)
}

func (vm *Manager) GetStatusBar() *statusbar.StatusBar {
	return vm.statusBar
}

// switchToEnvironment is the unified profile-switch entry point introduced
// in phase 4. Resolves the named environment via the YAML resolver,
// materializes it with the current region, and dispatches via
// profileHandler.SwitchEnvironment. The callback updates the header,
// reinitializes the active view, and (for jwt environments) pops the
// login modal on auth failure.
func (vm *Manager) switchToEnvironment(name string) {
	if vm.resolver == nil {
		vm.StatusChan <- "Configuration error: no environment resolver"
		return
	}
	if vm.profileHandler.IsAuthenticating() {
		vm.StatusChan <- "Authentication already in progress"
		return
	}

	spec, err := vm.resolver.Resolve(name)
	if err != nil {
		vm.StatusChan <- fmt.Sprintf("Resolve %q: %v", name, err)
		return
	}

	region := vm.profileHandler.GetRegion()
	env, err := environments.Materialize(spec, region)
	if err != nil {
		vm.StatusChan <- fmt.Sprintf("Materialize %q: %v", name, err)
		return
	}

	// Dragos-style jwt environments need the login modal popped on auth
	// failure if the user has no token cached. Detect this before invoking
	// SwitchEnvironment so the modal appears with no detour through a
	// status-bar error.
	needsLoginModal := env.Auth.Type == "jwt" && env.Auth.Login != nil

	vm.profileHandler.SwitchEnvironment(vm.ctx, env, func(sess *auth.Session, err error) {
		if err != nil {
			vm.app.QueueUpdateDraw(func() {
				vm.statusBar.SetText(fmt.Sprintf("Auth failed: %v", err))
				if needsLoginModal {
					vm.ShowJWTLoginModal(env,
						func() { vm.switchToEnvironment(name) },
						func() { vm.StatusChan <- "Auth canceled" },
					)
				}
			})
			return
		}

		vm.app.QueueUpdateDraw(func() {
			vm.header.UpdateEnvVar("Profile", sess.Environment.Name)
			if sess.Environment.Auth.Type == "aws_sdk" {
				vm.header.UpdateEnvVar("Region", sess.Environment.Region)
			} else {
				vm.header.UpdateEnvVar("Region", "—")
			}
		})

		if err := vm.reinitializeActiveView(); err != nil {
			vm.app.QueueUpdateDraw(func() {
				vm.statusBar.SetText(fmt.Sprintf("Reinit failed: %v", err))
				if needsLoginModal {
					vm.ShowJWTLoginModal(env,
						func() { vm.switchToEnvironment(name) },
						func() { vm.StatusChan <- "Auth canceled" },
					)
				}
			})
			return
		}

		if err := vm.SwitchToView(ViewElastic); err != nil {
			vm.Logger().Error("Failed to switch to Elastic", "error", err)
		}

		vm.StatusChan <- fmt.Sprintf("Switched to %s", sess.Environment.Name)
	})
}

// ShowJWTLoginModal opens a form derived from env.Auth.Login.BodyFields,
// POSTs the submitted values via auth.LoginJWT, persists the resulting
// token to env.Auth.Path, and calls onSuccess. Esc / Cancel calls
// onCancel. Every backend-specific knob (URL, query params, body fields,
// token extraction) comes from env.Auth.Login.
func (vm *Manager) ShowJWTLoginModal(env environments.Environment, onSuccess func(), onCancel func()) {
	const pageName = "jwtLogin"

	if env.Auth.Login == nil {
		vm.statusBar.SetText(fmt.Sprintf("Environment %q has no login spec", env.Name))
		if onCancel != nil {
			onCancel()
		}
		return
	}
	loginSpec := *env.Auth.Login

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" %s Login — %s (Tab to move, Enter to submit, Esc to cancel) ", env.Name, loginSpec.URL))
	form.SetTitleAlign(tview.AlignLeft)
	form.SetTitleColor(style.GruvboxMaterial.Yellow)
	form.SetBorderColor(tcell.ColorMediumTurquoise)
	form.SetFieldBackgroundColor(tcell.ColorBlack)
	form.SetFieldTextColor(tcell.ColorBeige)
	form.SetButtonBackgroundColor(tcell.ColorDarkCyan)
	form.SetButtonTextColor(tcell.ColorBeige)

	for _, f := range loginSpec.BodyFields {
		label := titleCase(f.Name)
		if f.Kind == "password" {
			form.AddPasswordField(label, "", 40, '*', nil)
		} else {
			form.AddInputField(label, "", 40, nil, nil)
		}
	}

	closeModal := func() { vm.pages.RemovePage(pageName) }

	submit := func() {
		values := make(map[string]string, len(loginSpec.BodyFields))
		missing := []string{}
		for _, f := range loginSpec.BodyFields {
			label := titleCase(f.Name)
			input, ok := form.GetFormItemByLabel(label).(*tview.InputField)
			if !ok {
				continue
			}
			val := input.GetText()
			if f.Kind != "password" {
				val = strings.TrimSpace(val)
			}
			if val == "" {
				missing = append(missing, label)
			}
			values[f.Name] = val
		}
		if len(missing) > 0 {
			vm.statusBar.SetText(fmt.Sprintf("Required: %s", strings.Join(missing, ", ")))
			return
		}

		vm.statusBar.SetText(fmt.Sprintf("Authenticating with %s...", env.Name))
		go func() {
			token, err := auth.LoginJWT(vm.ctx, loginSpec, values)
			vm.app.QueueUpdateDraw(func() {
				if err != nil {
					vm.statusBar.SetText(fmt.Sprintf("Login failed: %v", err))
					return
				}
				if err := auth.WriteTokenFile(env.Auth.Path, token); err != nil {
					vm.statusBar.SetText(fmt.Sprintf("Login OK but couldn't save token: %v", err))
					return
				}
				closeModal()
				vm.statusBar.SetText(fmt.Sprintf("%s login successful", env.Name))
				if onSuccess != nil {
					onSuccess()
				}
			})
		}()
	}

	form.AddButton("Login", submit)
	form.AddButton("Cancel", func() {
		closeModal()
		if onCancel != nil {
			onCancel()
		}
	})
	form.SetCancelFunc(func() {
		closeModal()
		if onCancel != nil {
			onCancel()
		}
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() != tcell.KeyEnter {
			return event
		}
		_, btnIdx := form.GetFocusedItemIndex()
		if btnIdx >= 0 {
			return event
		}
		submit()
		return nil
	})

	height := 5 + 2*len(loginSpec.BodyFields)
	width := 60
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(form, width, 0, true).
			AddItem(nil, 0, 1, false),
			height, 0, true).
		AddItem(nil, 0, 1, false)

	vm.pages.AddPage(pageName, layout, true, true)
	vm.app.SetFocus(form)
}

// titleCase capitalizes the first letter of s. Used to render lowercase
// body_fields names ("username") as proper form labels ("Username").
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (vm *Manager) reinitializeActiveView() error {
	if vm.activeView == nil {
		return nil
	}
	if reinit, ok := vm.activeView.(views.Reinitializer); ok {
		return reinit.Reinitialize()
	}
	return nil
}

// Stop stops the application
func (vm *Manager) Stop() {
	vm.app.Stop()
}

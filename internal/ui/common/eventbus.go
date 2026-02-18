package common

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type EventBus struct {
	handlers        map[ComponentType]ComponentHandler
	view            ViewInterface
	errorHandler    ErrorHandler
	logger          Logger
	globalHandler   GlobalShortcutHandler
	componentMapper ComponentMapper
}

type EventBusConfig struct {
	View          ViewInterface
	ErrorHandler  ErrorHandler
	Logger        Logger
	GlobalHandler GlobalShortcutHandler
}

func NewEventBus(config *EventBusConfig) *EventBus {
	return &EventBus{
		handlers:        make(map[ComponentType]ComponentHandler),
		view:            config.View,
		errorHandler:    config.ErrorHandler,
		logger:          config.Logger,
		globalHandler:   config.GlobalHandler,
		componentMapper: config.View.GetComponentMapper(),
	}
}

func (eb *EventBus) RegisterHandler(componentType ComponentType, handler ComponentHandler) {
	eb.handlers[componentType] = handler
	if eb.logger != nil {
		eb.logger.Debug("Registered handler", "componentType", int(componentType), "view", eb.view.GetName())
	}
}

func (eb *EventBus) ProcessEvent(event *tcell.EventKey, currentFocus tview.Primitive) *tcell.EventKey {
	// Try component handlers first
	componentType := eb.componentMapper.GetComponentType(currentFocus)
	if componentType != nil {
		if handler, exists := eb.handlers[*componentType]; exists {
			if handler.CanHandle(event, currentFocus) {
				ctx := &HandlerContext{
					View:          eb.view,
					Component:     currentFocus,
					ComponentType: componentType,
					ErrorHandler:  eb.errorHandler,
					Logger:        eb.logger,
				}
				if result := handler.HandleEvent(event, ctx); result == nil {
					return nil
				}
			}
		}
	}

	// Try global shortcuts
	if eb.globalHandler != nil {
		if result := eb.globalHandler.HandleGlobalShortcut(event, currentFocus, eb.view); result == nil {
			return nil
		}
	}

	// Handle common shortcuts (Tab navigation)
	if result := eb.handleCommonShortcuts(event, currentFocus); result == nil {
		return nil
	}

	return event
}

func (eb *EventBus) handleCommonShortcuts(event *tcell.EventKey, currentFocus tview.Primitive) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyTab:
		eb.handleBasicNavigation(currentFocus, true)
		return nil
	case tcell.KeyBacktab:
		eb.handleBasicNavigation(currentFocus, false)
		return nil
	}
	return event
}

func (eb *EventBus) handleBasicNavigation(currentFocus tview.Primitive, forward bool) {
	componentList := eb.componentMapper.GetNavigationOrder()
	if len(componentList) == 0 {
		components := eb.componentMapper.GetComponents()
		for _, component := range components {
			if component != nil {
				componentList = append(componentList, component)
			}
		}
	}

	if len(componentList) == 0 {
		return
	}

	currentIndex := -1
	for i, component := range componentList {
		if component == currentFocus {
			currentIndex = i
			break
		}
	}

	var nextIndex int
	if forward {
		nextIndex = (currentIndex + 1) % len(componentList)
	} else {
		nextIndex = (currentIndex - 1 + len(componentList)) % len(componentList)
	}

	if nextIndex >= 0 && nextIndex < len(componentList) {
		eb.view.GetManager().SetFocus(componentList[nextIndex])
	}
}
package common

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type BaseHandler struct {
	ComponentType ComponentType
	View          ViewInterface
}

func NewBaseHandler(componentType ComponentType, view ViewInterface) *BaseHandler {
	return &BaseHandler{
		ComponentType: componentType,
		View:          view,
	}
}

func (h *BaseHandler) GetComponentType() ComponentType {
	return h.ComponentType
}

func (h *BaseHandler) CanHandle(event *tcell.EventKey, component tview.Primitive) bool {
	mapper := h.View.GetComponentMapper()
	componentType := mapper.GetComponentType(component)
	return componentType != nil && *componentType == h.ComponentType
}

type DefaultGlobalShortcutHandler struct{}

func NewDefaultGlobalShortcutHandler() *DefaultGlobalShortcutHandler {
	return &DefaultGlobalShortcutHandler{}
}

func (h *DefaultGlobalShortcutHandler) HandleGlobalShortcut(event *tcell.EventKey, currentFocus tview.Primitive, view ViewInterface) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlC:
		view.GetManager().Stop()
		return nil
	}
	return event
}

type SimpleErrorHandler struct {
	manager ManagerInterface
	logger  Logger
}

func NewSimpleErrorHandler(mgr ManagerInterface, logger Logger) *SimpleErrorHandler {
	return &SimpleErrorHandler{manager: mgr, logger: logger}
}

func (h *SimpleErrorHandler) HandleError(err error) {
	if h.logger != nil {
		h.logger.Error("Error occurred", "error", err)
	}
	if h.manager != nil {
		h.manager.UpdateStatusBar("Error: " + err.Error())
	}
}

func (h *SimpleErrorHandler) UpdateStatus(message string) {
	if h.manager != nil {
		h.manager.UpdateStatusBar(message)
	}
}

type NoOpLogger struct{}

func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}

func (l *NoOpLogger) Debug(message string, args ...interface{}) {}
func (l *NoOpLogger) Info(message string, args ...interface{})  {}
func (l *NoOpLogger) Error(message string, args ...interface{}) {}

type LoggerAdapter struct {
	debugFunc func(string, ...interface{})
	infoFunc  func(string, ...interface{})
	errorFunc func(string, ...interface{})
}

func NewLoggerAdapter(
	debugFunc func(string, ...interface{}),
	infoFunc func(string, ...interface{}),
	errorFunc func(string, ...interface{}),
) *LoggerAdapter {
	return &LoggerAdapter{
		debugFunc: debugFunc,
		infoFunc:  infoFunc,
		errorFunc: errorFunc,
	}
}

func (l *LoggerAdapter) Debug(message string, args ...interface{}) {
	if l.debugFunc != nil {
		l.debugFunc(message, args...)
	}
}

func (l *LoggerAdapter) Info(message string, args ...interface{}) {
	if l.infoFunc != nil {
		l.infoFunc(message, args...)
	}
}

func (l *LoggerAdapter) Error(message string, args ...interface{}) {
	if l.errorFunc != nil {
		l.errorFunc(message, args...)
	}
}
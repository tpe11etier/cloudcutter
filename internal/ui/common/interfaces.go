package common

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ComponentType int

type ViewInterface interface {
	GetManager() ManagerInterface
	GetComponentMapper() ComponentMapper
	GetName() string
}

type ComponentMapper interface {
	GetComponentType(primitive tview.Primitive) *ComponentType
	GetComponents() map[ComponentType]tview.Primitive
	GetNavigationOrder() []tview.Primitive
}

type ComponentHandler interface {
	CanHandle(event *tcell.EventKey, component tview.Primitive) bool
	HandleEvent(event *tcell.EventKey, ctx *HandlerContext) *tcell.EventKey
}

type HandlerContext struct {
	View          ViewInterface
	Component     tview.Primitive
	ComponentType *ComponentType
	ErrorHandler  ErrorHandler
	Logger        Logger
}

type ErrorHandler interface {
	HandleError(err error)
	UpdateStatus(message string)
}

type Logger interface {
	Debug(message string, args ...interface{})
	Info(message string, args ...interface{})
	Error(message string, args ...interface{})
}

type GlobalShortcutHandler interface {
	HandleGlobalShortcut(event *tcell.EventKey, currentFocus tview.Primitive, view ViewInterface) *tcell.EventKey
}

type ManagerInterface interface {
	UpdateStatusBar(message string)
	HideAllModals()
	SetFocus(primitive tview.Primitive)
	Stop()
}
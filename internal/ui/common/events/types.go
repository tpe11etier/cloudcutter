package events

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ActionType represents the type of action to execute - views can extend this
type ActionType int

const (
	ActionFocus ActionType = iota
	ActionToggle
	ActionNavigate
	ActionFilter
	ActionDocument
	ActionEdit
	ActionClear
	ActionDelete
	ActionMove
	// Views can define their own action types starting from ActionCustomBase
	ActionCustomBase ActionType = 1000
)

// ComponentType represents UI components - each view defines their own
type ComponentType int

// EventResult indicates the outcome of event processing
type EventResult int

const (
	EventHandled EventResult = iota
	EventUnhandled
	EventPropagated
	EventCancelled
	EventError
)

// String returns the string representation of EventResult
func (r EventResult) String() string {
	switch r {
	case EventHandled:
		return "handled"
	case EventUnhandled:
		return "unhandled"
	case EventPropagated:
		return "propagated"
	case EventCancelled:
		return "cancelled"
	case EventError:
		return "error"
	default:
		return "unknown"
	}
}

// KeyAction represents an action that can be executed
type KeyAction struct {
	Type ActionType
	Data interface{} // Flexible data payload for the action
}

// KeyMapping represents a key binding configuration
type KeyMapping struct {
	Key       tcell.Key
	Rune      rune
	Modifiers tcell.ModMask
	Component *ComponentType // nil means global shortcut
	Action    KeyAction
}

// EventContext provides rich context for event processing
type EventContext struct {
	Event        *tcell.EventKey    `json:"event"`
	CurrentFocus tview.Primitive    `json:"-"`
	Component    *ComponentType     `json:"component"`
	Timestamp    time.Time          `json:"timestamp"`
	TraceID      string             `json:"trace_id"`
	SessionID    string             `json:"session_id"`
	Metadata     map[string]any     `json:"metadata"`
	Action       *KeyAction         `json:"action,omitempty"`
	Result       EventResult        `json:"result"`
	Duration     time.Duration      `json:"duration"`
	Error        error              `json:"error,omitempty"`
}

// EventMiddleware provides a way to intercept and modify event processing
type EventMiddleware func(ctx *EventContext, next func(*EventContext) *tcell.EventKey) *tcell.EventKey

// EventInterceptor allows inspection and potential modification of events before processing
type EventInterceptor func(ctx *EventContext) bool // return false to cancel event

// ComponentResolver maps UI primitives to component types - implemented by each view
type ComponentResolver interface {
	GetComponentType(focus tview.Primitive) *ComponentType
	FormatComponent(component *ComponentType) string
}

// ActionExecutor executes actions - implemented by each view
type ActionExecutor interface {
	ExecuteAction(action *KeyAction) *tcell.EventKey
}

// KeyMappingResolver resolves key events to actions - can be implemented by each view
type KeyMappingResolver interface {
	ResolveKeyEvent(event *tcell.EventKey, currentFocus tview.Primitive) *KeyAction
}

// HandlerManager manages component handlers - implemented by each view
type HandlerManager interface {
	HandleEvent(event *tcell.EventKey, currentFocus tview.Primitive) *tcell.EventKey
}

// EventMetrics tracks event handling performance and patterns
type EventMetrics struct {
	TotalEvents       int64                       `json:"total_events"`
	HandledEvents     int64                       `json:"handled_events"`
	UnhandledEvents   int64                       `json:"unhandled_events"`
	ErrorCount        int64                       `json:"error_count"`
	AverageDuration   time.Duration               `json:"average_duration"`
	ComponentMetrics  map[ComponentType]*ComponentMetrics `json:"component_metrics"`
	ActionMetrics     map[ActionType]*ActionMetrics       `json:"action_metrics"`
	KeyMetrics        map[string]*KeyMetrics              `json:"key_metrics"`
	EventHistory      []*EventContext             `json:"event_history"`
}

// ComponentMetrics tracks metrics for specific components
type ComponentMetrics struct {
	EventCount      int64         `json:"event_count"`
	AverageDuration time.Duration `json:"average_duration"`
	LastEvent       time.Time     `json:"last_event"`
	ErrorCount      int64         `json:"error_count"`
}

// ActionMetrics tracks metrics for specific actions
type ActionMetrics struct {
	ExecutionCount  int64         `json:"execution_count"`
	AverageDuration time.Duration `json:"average_duration"`
	SuccessCount    int64         `json:"success_count"`
	ErrorCount      int64         `json:"error_count"`
}

// KeyMetrics tracks metrics for specific key combinations
type KeyMetrics struct {
	PressCount      int64         `json:"press_count"`
	AverageDuration time.Duration `json:"average_duration"`
	LastPressed     time.Time     `json:"last_pressed"`
}
package events

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/tpelletiersophos/cloudcutter/internal/ui/common/errors"
)

// EventManagerConfig contains configuration for enhanced event management
type EventManagerConfig struct {
	EnableLogging        bool          `json:"enable_logging"`
	EnableMetrics        bool          `json:"enable_metrics"`
	EnableTracing        bool          `json:"enable_tracing"`
	LogUnhandledEvents   bool          `json:"log_unhandled_events"`
	MetricsFlushInterval time.Duration `json:"metrics_flush_interval"`
	MaxEventHistory      int           `json:"max_event_history"`
	DebugMode            bool          `json:"debug_mode"`
}

// NewEventManagerConfig creates default event manager configuration
func NewEventManagerConfig() *EventManagerConfig {
	return &EventManagerConfig{
		EnableLogging:        true,
		EnableMetrics:        true,
		EnableTracing:        false,
		LogUnhandledEvents:   true,
		MetricsFlushInterval: 30 * time.Second,
		MaxEventHistory:      1000,
		DebugMode:            false,
	}
}

// EventManager provides enhanced event management with logging, middleware, and monitoring
type EventManager struct {
	config           *EventManagerConfig
	middleware       []EventMiddleware
	interceptors     []EventInterceptor
	metrics          *EventMetrics
	logger           errors.Logger
	mu               sync.RWMutex

	// Pluggable components - each view provides these
	componentResolver  ComponentResolver
	actionExecutor     ActionExecutor
	keyResolver        KeyMappingResolver
	handlerManager     HandlerManager
}

// NewEventManager creates a new enhanced event manager
func NewEventManager(
	logger errors.Logger,
	config *EventManagerConfig,
	componentResolver ComponentResolver,
	actionExecutor ActionExecutor,
	keyResolver KeyMappingResolver,
	handlerManager HandlerManager,
) *EventManager {
	if config == nil {
		config = NewEventManagerConfig()
	}

	em := &EventManager{
		config:             config,
		middleware:         make([]EventMiddleware, 0),
		interceptors:       make([]EventInterceptor, 0),
		logger:            logger,
		componentResolver: componentResolver,
		actionExecutor:    actionExecutor,
		keyResolver:       keyResolver,
		handlerManager:    handlerManager,
		metrics: &EventMetrics{
			ComponentMetrics: make(map[ComponentType]*ComponentMetrics),
			ActionMetrics:    make(map[ActionType]*ActionMetrics),
			KeyMetrics:       make(map[string]*KeyMetrics),
			EventHistory:     make([]*EventContext, 0, config.MaxEventHistory),
		},
	}

	// Add default middleware
	if config.EnableLogging {
		em.AddMiddleware(em.loggingMiddleware)
	}
	if config.EnableMetrics {
		em.AddMiddleware(em.metricsMiddleware)
	}
	if config.EnableTracing {
		em.AddMiddleware(em.tracingMiddleware)
	}

	return em
}

// AddMiddleware adds an event middleware to the processing pipeline
func (em *EventManager) AddMiddleware(middleware EventMiddleware) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.middleware = append(em.middleware, middleware)
}

// AddInterceptor adds an event interceptor
func (em *EventManager) AddInterceptor(interceptor EventInterceptor) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.interceptors = append(em.interceptors, interceptor)
}

// ProcessEvent processes an event through the enhanced pipeline
func (em *EventManager) ProcessEvent(event *tcell.EventKey, currentFocus tview.Primitive) *tcell.EventKey {
	start := time.Now()

	// Create event context
	ctx := &EventContext{
		Event:        event,
		CurrentFocus: currentFocus,
		Timestamp:    start,
		TraceID:      em.generateTraceID(),
		SessionID:    em.generateSessionID(),
		Metadata:     make(map[string]any),
		Result:       EventUnhandled,
	}

	// Get component type
	if em.componentResolver != nil {
		ctx.Component = em.componentResolver.GetComponentType(currentFocus)
	}

	// Run interceptors
	for _, interceptor := range em.interceptors {
		if !interceptor(ctx) {
			ctx.Result = EventCancelled
			ctx.Duration = time.Since(start)
			em.recordEvent(ctx)
			return nil
		}
	}

	// Process through middleware chain
	result := em.processWithMiddleware(ctx, 0)

	ctx.Duration = time.Since(start)
	em.recordEvent(ctx)

	return result
}

// processWithMiddleware processes the event through the middleware chain
func (em *EventManager) processWithMiddleware(ctx *EventContext, index int) *tcell.EventKey {
	if index >= len(em.middleware) {
		// End of middleware chain, execute core event handling
		return em.coreEventHandler(ctx)
	}

	// Call the current middleware with a next function
	return em.middleware[index](ctx, func(ctx *EventContext) *tcell.EventKey {
		return em.processWithMiddleware(ctx, index+1)
	})
}

// coreEventHandler handles the core event processing logic
func (em *EventManager) coreEventHandler(ctx *EventContext) *tcell.EventKey {
	// First try key mapping resolution
	if em.keyResolver != nil {
		if action := em.keyResolver.ResolveKeyEvent(ctx.Event, ctx.CurrentFocus); action != nil {
			ctx.Action = action
			var result *tcell.EventKey
			if em.actionExecutor != nil {
				result = em.actionExecutor.ExecuteAction(action)
			}
			if result == nil {
				ctx.Result = EventHandled
			} else {
				ctx.Result = EventPropagated
			}
			return result
		}
	}

	// Then try handler manager
	if em.handlerManager != nil {
		result := em.handlerManager.HandleEvent(ctx.Event, ctx.CurrentFocus)
		if result == nil {
			ctx.Result = EventHandled
		} else if result == ctx.Event {
			ctx.Result = EventUnhandled
		} else {
			ctx.Result = EventPropagated
		}
		return result
	}

	ctx.Result = EventUnhandled
	return ctx.Event
}

// Built-in middleware implementations

// loggingMiddleware logs event processing
func (em *EventManager) loggingMiddleware(ctx *EventContext, next func(*EventContext) *tcell.EventKey) *tcell.EventKey {
	if em.config.DebugMode {
		em.logger.Debug("Processing event",
			"key", em.formatEventKey(ctx.Event),
			"component", em.formatComponent(ctx.Component),
			"trace_id", ctx.TraceID)
	}

	result := next(ctx)

	if em.config.EnableLogging {
		logLevel := "Debug"
		if ctx.Result == EventError {
			logLevel = "Error"
		} else if ctx.Result == EventUnhandled && em.config.LogUnhandledEvents {
			logLevel = "Info"
		}

		logArgs := []interface{}{
			"key", em.formatEventKey(ctx.Event),
			"component", em.formatComponent(ctx.Component),
			"result", ctx.Result.String(),
			"duration", ctx.Duration,
			"trace_id", ctx.TraceID,
		}

		if ctx.Action != nil {
			logArgs = append(logArgs, "action", ctx.Action.Type)
		}
		if ctx.Error != nil {
			logArgs = append(logArgs, "error", ctx.Error)
		}

		switch logLevel {
		case "Error":
			em.logger.Error("Event processing failed", logArgs...)
		case "Info":
			em.logger.Info("Event processed", logArgs...)
		case "Debug":
			em.logger.Debug("Event processed", logArgs...)
		}
	}

	return result
}

// metricsMiddleware tracks event metrics
func (em *EventManager) metricsMiddleware(ctx *EventContext, next func(*EventContext) *tcell.EventKey) *tcell.EventKey {
	result := next(ctx)

	em.updateMetrics(ctx)
	return result
}

// tracingMiddleware adds tracing information
func (em *EventManager) tracingMiddleware(ctx *EventContext, next func(*EventContext) *tcell.EventKey) *tcell.EventKey {
	// Add tracing metadata
	ctx.Metadata["trace_start"] = ctx.Timestamp
	ctx.Metadata["trace_context"] = fmt.Sprintf("component=%s,action=%v",
		em.formatComponent(ctx.Component), ctx.Action)

	result := next(ctx)

	ctx.Metadata["trace_end"] = time.Now()
	ctx.Metadata["trace_duration"] = ctx.Duration

	return result
}

// Helper methods

// generateTraceID generates a unique trace ID for event tracking
func (em *EventManager) generateTraceID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}

// generateSessionID generates a session ID (simplified implementation)
func (em *EventManager) generateSessionID() string {
	return "session_1" // In a real implementation, this would be more sophisticated
}

// formatEventKey formats an event key for logging
func (em *EventManager) formatEventKey(event *tcell.EventKey) string {
	if event.Key() == tcell.KeyRune {
		return fmt.Sprintf("'%c'", event.Rune())
	}
	return em.keyToString(event.Key())
}

// keyToString converts a tcell.Key to its string representation
func (em *EventManager) keyToString(key tcell.Key) string {
	switch key {
	case tcell.KeyEnter:
		return "Enter"
	case tcell.KeyEsc:
		return "Esc"
	case tcell.KeyTab:
		return "Tab"
	case tcell.KeyBacktab:
		return "Backtab"
	case tcell.KeyBackspace:
		return "Backspace"
	case tcell.KeyBackspace2:
		return "Backspace2"
	case tcell.KeyDelete:
		return "Delete"
	case tcell.KeyCtrlA:
		return "Ctrl+A"
	case tcell.KeyCtrlS:
		return "Ctrl+S"
	case tcell.KeyCtrlR:
		return "Ctrl+R"
	case tcell.KeyUp:
		return "Up"
	case tcell.KeyDown:
		return "Down"
	case tcell.KeyLeft:
		return "Left"
	case tcell.KeyRight:
		return "Right"
	case tcell.KeyHome:
		return "Home"
	case tcell.KeyEnd:
		return "End"
	case tcell.KeyPgUp:
		return "PgUp"
	case tcell.KeyPgDn:
		return "PgDn"
	case tcell.KeyF1:
		return "F1"
	case tcell.KeyF2:
		return "F2"
	case tcell.KeyF3:
		return "F3"
	case tcell.KeyF4:
		return "F4"
	case tcell.KeyF5:
		return "F5"
	case tcell.KeyF6:
		return "F6"
	case tcell.KeyF7:
		return "F7"
	case tcell.KeyF8:
		return "F8"
	case tcell.KeyF9:
		return "F9"
	case tcell.KeyF10:
		return "F10"
	case tcell.KeyF11:
		return "F11"
	case tcell.KeyF12:
		return "F12"
	default:
		return fmt.Sprintf("Key(%d)", int(key))
	}
}

// formatComponent formats a component type for logging
func (em *EventManager) formatComponent(component *ComponentType) string {
	if em.componentResolver != nil && component != nil {
		return em.componentResolver.FormatComponent(component)
	}
	if component == nil {
		return "none"
	}
	return fmt.Sprintf("component_%d", int(*component))
}

// updateMetrics updates event metrics
func (em *EventManager) updateMetrics(ctx *EventContext) {
	em.mu.Lock()
	defer em.mu.Unlock()

	em.metrics.TotalEvents++

	switch ctx.Result {
	case EventHandled:
		em.metrics.HandledEvents++
	case EventUnhandled:
		em.metrics.UnhandledEvents++
	case EventError:
		em.metrics.ErrorCount++
	}

	// Update average duration
	if em.metrics.TotalEvents == 1 {
		em.metrics.AverageDuration = ctx.Duration
	} else {
		em.metrics.AverageDuration = time.Duration(
			(int64(em.metrics.AverageDuration)*(em.metrics.TotalEvents-1) + int64(ctx.Duration)) / em.metrics.TotalEvents,
		)
	}

	// Update component metrics
	if ctx.Component != nil {
		if _, exists := em.metrics.ComponentMetrics[*ctx.Component]; !exists {
			em.metrics.ComponentMetrics[*ctx.Component] = &ComponentMetrics{}
		}
		metrics := em.metrics.ComponentMetrics[*ctx.Component]
		metrics.EventCount++
		metrics.LastEvent = ctx.Timestamp
		if ctx.Result == EventError {
			metrics.ErrorCount++
		}
		if metrics.EventCount == 1 {
			metrics.AverageDuration = ctx.Duration
		} else {
			metrics.AverageDuration = time.Duration(
				(int64(metrics.AverageDuration)*(metrics.EventCount-1) + int64(ctx.Duration)) / metrics.EventCount,
			)
		}
	}

	// Update action metrics
	if ctx.Action != nil {
		if _, exists := em.metrics.ActionMetrics[ctx.Action.Type]; !exists {
			em.metrics.ActionMetrics[ctx.Action.Type] = &ActionMetrics{}
		}
		metrics := em.metrics.ActionMetrics[ctx.Action.Type]
		metrics.ExecutionCount++
		if ctx.Result == EventHandled {
			metrics.SuccessCount++
		} else if ctx.Result == EventError {
			metrics.ErrorCount++
		}
		if metrics.ExecutionCount == 1 {
			metrics.AverageDuration = ctx.Duration
		} else {
			metrics.AverageDuration = time.Duration(
				(int64(metrics.AverageDuration)*(metrics.ExecutionCount-1) + int64(ctx.Duration)) / metrics.ExecutionCount,
			)
		}
	}

	// Update key metrics
	keyStr := em.formatEventKey(ctx.Event)
	if _, exists := em.metrics.KeyMetrics[keyStr]; !exists {
		em.metrics.KeyMetrics[keyStr] = &KeyMetrics{}
	}
	keyMetric := em.metrics.KeyMetrics[keyStr]
	keyMetric.PressCount++
	keyMetric.LastPressed = ctx.Timestamp
	if keyMetric.PressCount == 1 {
		keyMetric.AverageDuration = ctx.Duration
	} else {
		keyMetric.AverageDuration = time.Duration(
			(int64(keyMetric.AverageDuration)*(keyMetric.PressCount-1) + int64(ctx.Duration)) / keyMetric.PressCount,
		)
	}
}

// recordEvent records an event in the history
func (em *EventManager) recordEvent(ctx *EventContext) {
	if !em.config.EnableMetrics {
		return
	}

	em.mu.Lock()
	defer em.mu.Unlock()

	// Add to history (with circular buffer behavior)
	if len(em.metrics.EventHistory) >= em.config.MaxEventHistory {
		// Remove oldest event
		em.metrics.EventHistory = em.metrics.EventHistory[1:]
	}
	em.metrics.EventHistory = append(em.metrics.EventHistory, ctx)
}

// GetMetrics returns a copy of current metrics
func (em *EventManager) GetMetrics() *EventMetrics {
	em.mu.RLock()
	defer em.mu.RUnlock()

	// Create a deep copy of metrics
	return &EventMetrics{
		TotalEvents:      em.metrics.TotalEvents,
		HandledEvents:    em.metrics.HandledEvents,
		UnhandledEvents:  em.metrics.UnhandledEvents,
		ErrorCount:       em.metrics.ErrorCount,
		AverageDuration:  em.metrics.AverageDuration,
		ComponentMetrics: em.copyComponentMetrics(),
		ActionMetrics:    em.copyActionMetrics(),
		KeyMetrics:       em.copyKeyMetrics(),
		EventHistory:     em.copyEventHistory(),
	}
}

// Helper methods for deep copying metrics
func (em *EventManager) copyComponentMetrics() map[ComponentType]*ComponentMetrics {
	copy := make(map[ComponentType]*ComponentMetrics)
	for k, v := range em.metrics.ComponentMetrics {
		copy[k] = &ComponentMetrics{
			EventCount:      v.EventCount,
			AverageDuration: v.AverageDuration,
			LastEvent:       v.LastEvent,
			ErrorCount:      v.ErrorCount,
		}
	}
	return copy
}

func (em *EventManager) copyActionMetrics() map[ActionType]*ActionMetrics {
	copy := make(map[ActionType]*ActionMetrics)
	for k, v := range em.metrics.ActionMetrics {
		copy[k] = &ActionMetrics{
			ExecutionCount:  v.ExecutionCount,
			AverageDuration: v.AverageDuration,
			SuccessCount:    v.SuccessCount,
			ErrorCount:      v.ErrorCount,
		}
	}
	return copy
}

func (em *EventManager) copyKeyMetrics() map[string]*KeyMetrics {
	copy := make(map[string]*KeyMetrics)
	for k, v := range em.metrics.KeyMetrics {
		copy[k] = &KeyMetrics{
			PressCount:      v.PressCount,
			AverageDuration: v.AverageDuration,
			LastPressed:     v.LastPressed,
		}
	}
	return copy
}

func (em *EventManager) copyEventHistory() []*EventContext {
	copy := make([]*EventContext, len(em.metrics.EventHistory))
	for i, ctx := range em.metrics.EventHistory {
		copy[i] = &EventContext{
			Component: ctx.Component,
			Timestamp: ctx.Timestamp,
			TraceID:   ctx.TraceID,
			SessionID: ctx.SessionID,
			Action:    ctx.Action,
			Result:    ctx.Result,
			Duration:  ctx.Duration,
			Error:     ctx.Error,
		}
	}
	return copy
}

// FormatMetricsSummary returns a formatted summary of metrics
func (em *EventManager) FormatMetricsSummary() string {
	metrics := em.GetMetrics()

	var summary strings.Builder
	summary.WriteString("Event Manager Metrics Summary:\n")
	summary.WriteString(fmt.Sprintf("Total Events: %d\n", metrics.TotalEvents))
	if metrics.TotalEvents > 0 {
		summary.WriteString(fmt.Sprintf("Handled: %d (%.1f%%)\n",
			metrics.HandledEvents,
			float64(metrics.HandledEvents)/float64(metrics.TotalEvents)*100))
		summary.WriteString(fmt.Sprintf("Unhandled: %d (%.1f%%)\n",
			metrics.UnhandledEvents,
			float64(metrics.UnhandledEvents)/float64(metrics.TotalEvents)*100))
		summary.WriteString(fmt.Sprintf("Errors: %d (%.1f%%)\n",
			metrics.ErrorCount,
			float64(metrics.ErrorCount)/float64(metrics.TotalEvents)*100))
	}
	summary.WriteString(fmt.Sprintf("Average Duration: %v\n", metrics.AverageDuration))

	return summary.String()
}
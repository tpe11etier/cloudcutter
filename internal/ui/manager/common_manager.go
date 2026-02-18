package manager

import (
	"context"
	"time"

	"github.com/tpelletiersophos/cloudcutter/internal/ui/common"
	commonErrors "github.com/tpelletiersophos/cloudcutter/internal/ui/common/errors"
	commonEvents "github.com/tpelletiersophos/cloudcutter/internal/ui/common/events"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/views"
)

// CommonSystemsManager extends the Manager to coordinate common systems across all views
type CommonSystemsManager struct {
	*Manager

	// Common systems that are shared across all views
	globalErrorHandler *commonErrors.ErrorHandler
	globalEventConfig  *commonEvents.EventManagerConfig
	systemMetrics      *SystemMetrics
	factory            *common.SystemsFactory

	// Configuration for common systems
	errorConfig *CommonErrorConfig
	eventConfig *CommonEventConfig
}

// CommonErrorConfig holds global error handling configuration
type CommonErrorConfig struct {
	EnableGlobalErrorLogging bool        `json:"enable_global_error_logging"`
	EnableErrorMetrics       bool        `json:"enable_error_metrics"`
	GlobalLogLevel           string      `json:"global_log_level"`
	MaxErrorHistory          int         `json:"max_error_history"`
	ErrorRetryPolicy         RetryPolicy `json:"error_retry_policy"`
}

// CommonEventConfig holds global event management configuration
type CommonEventConfig struct {
	EnableGlobalEventLogging bool          `json:"enable_global_event_logging"`
	EnableEventMetrics       bool          `json:"enable_event_metrics"`
	EnableTracing            bool          `json:"enable_tracing"`
	EventHistorySize         int           `json:"event_history_size"`
	MetricsFlushInterval     time.Duration `json:"metrics_flush_interval"`
}

// RetryPolicy defines how the manager should handle retryable errors
type RetryPolicy struct {
	MaxRetries    int           `json:"max_retries"`
	InitialDelay  time.Duration `json:"initial_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	BackoffFactor float64       `json:"backoff_factor"`
}

// SystemMetrics tracks health and performance across all views
type SystemMetrics struct {
	ViewCount        int              `json:"view_count"`
	ActiveView       string           `json:"active_view"`
	ErrorCounts      map[string]int64 `json:"error_counts_by_view"`
	EventCounts      map[string]int64 `json:"event_counts_by_view"`
	LastActivity     time.Time        `json:"last_activity"`
	ViewSwitchCount  int64            `json:"view_switch_count"`
	GlobalErrorCount int64            `json:"global_error_count"`
	SystemUptime     time.Duration    `json:"system_uptime"`
	MemoryUsage      int64            `json:"memory_usage"`
}

// NewCommonSystemsManager creates an enhanced manager with common system coordination
func NewCommonSystemsManager(ctx context.Context, baseManager *Manager) *CommonSystemsManager {
	csm := &CommonSystemsManager{
		Manager: baseManager,
		factory: common.NewSystemsFactory(baseManager.Logger()),
		errorConfig: &CommonErrorConfig{
			EnableGlobalErrorLogging: true,
			EnableErrorMetrics:       true,
			GlobalLogLevel:           "info",
			MaxErrorHistory:          1000,
			ErrorRetryPolicy: RetryPolicy{
				MaxRetries:    3,
				InitialDelay:  1 * time.Second,
				MaxDelay:      30 * time.Second,
				BackoffFactor: 2.0,
			},
		},
		eventConfig: &CommonEventConfig{
			EnableGlobalEventLogging: true,
			EnableEventMetrics:       true,
			EnableTracing:            false,
			EventHistorySize:         500,
			MetricsFlushInterval:     30 * time.Second,
		},
		systemMetrics: &SystemMetrics{
			ErrorCounts:  make(map[string]int64),
			EventCounts:  make(map[string]int64),
			LastActivity: time.Now(),
		},
	}

	// Initialize global error handler
	csm.initializeGlobalErrorHandler()

	// Initialize global event configuration
	csm.initializeGlobalEventConfig()

	// Start background monitoring
	go csm.startSystemMonitoring(ctx)

	return csm
}

// initializeGlobalErrorHandler creates the global error handler for cross-view coordination
func (csm *CommonSystemsManager) initializeGlobalErrorHandler() {
	config := &commonErrors.ErrorHandlerConfig{
		LogStackTrace:     csm.errorConfig.EnableGlobalErrorLogging,
		LogMetadata:       true,
		MaxStackDepth:     15,
		EnableUserMetrics: csm.errorConfig.EnableErrorMetrics,
		Component:         "ui-manager",
	}

	csm.globalErrorHandler = commonErrors.NewErrorHandler(csm.Logger(), config)
}

// initializeGlobalEventConfig creates the global event configuration
func (csm *CommonSystemsManager) initializeGlobalEventConfig() {
	csm.globalEventConfig = &commonEvents.EventManagerConfig{
		EnableLogging:        csm.eventConfig.EnableGlobalEventLogging,
		EnableMetrics:        csm.eventConfig.EnableEventMetrics,
		EnableTracing:        csm.eventConfig.EnableTracing,
		LogUnhandledEvents:   true,
		MetricsFlushInterval: csm.eventConfig.MetricsFlushInterval,
		MaxEventHistory:      csm.eventConfig.EventHistorySize,
		DebugMode:            false,
	}
}

// RegisterViewWithCommonSystems registers a view and provides it with common systems
func (csm *CommonSystemsManager) RegisterViewWithCommonSystems(view views.View) error {
	// Register with the base manager first
	if err := csm.Manager.RegisterView(view); err != nil {
		return csm.globalErrorHandler.WrapWithOperation("view_registration", err)
	}

	csm.systemMetrics.ViewCount++
	csm.systemMetrics.ErrorCounts[view.Name()] = 0
	csm.systemMetrics.EventCounts[view.Name()] = 0

	csm.Logger().Info("View registered with common systems",
		"view", view.Name(),
		"total_views", csm.systemMetrics.ViewCount)

	return nil
}

// SwitchToViewWithSystemCoordination switches views and coordinates common systems
func (csm *CommonSystemsManager) SwitchToViewWithSystemCoordination(name string) error {
	// Track the switch attempt
	csm.systemMetrics.ViewSwitchCount++
	csm.systemMetrics.LastActivity = time.Now()

	// Use the base manager's switch functionality
	err := csm.Manager.SwitchToView(name)
	if err != nil {
		// Handle the error through the global error system
		viewErr := csm.globalErrorHandler.WrapWithOperation("view_switch", err)
		csm.globalErrorHandler.HandleError(viewErr)
		csm.systemMetrics.GlobalErrorCount++
		return viewErr
	}

	csm.systemMetrics.ActiveView = name
	csm.Logger().Info("Successfully switched views with system coordination",
		"view", name,
		"switch_count", csm.systemMetrics.ViewSwitchCount)

	return nil
}

// HandleGlobalError handles errors that bubble up from views
func (csm *CommonSystemsManager) HandleGlobalError(viewName string, err error) {
	csm.systemMetrics.GlobalErrorCount++
	csm.systemMetrics.ErrorCounts[viewName]++
	csm.systemMetrics.LastActivity = time.Now()

	// Process through the global error handler
	wrappedErr := csm.globalErrorHandler.WrapWithOperation("global_error_handling", err)
	csm.globalErrorHandler.HandleError(wrappedErr)

	// Update status bar with user-friendly message
	csm.UpdateStatusBar(wrappedErr.UserMessage)

	// Check if we need to take any global actions
	csm.evaluateSystemHealth()
}

// GetCommonErrorConfig returns the error handler configuration for views to use
func (csm *CommonSystemsManager) GetCommonErrorConfig() *commonErrors.ErrorHandlerConfig {
	return &commonErrors.ErrorHandlerConfig{
		LogStackTrace:     csm.errorConfig.EnableGlobalErrorLogging,
		LogMetadata:       true,
		MaxStackDepth:     10,
		EnableUserMetrics: csm.errorConfig.EnableErrorMetrics,
		Component:         "view", // Will be overridden by specific views
	}
}

// GetCommonEventConfig returns the event manager configuration for views to use
func (csm *CommonSystemsManager) GetCommonEventConfig() *commonEvents.EventManagerConfig {
	// Return a copy of the global config for views to customize
	config := *csm.globalEventConfig
	return &config
}

// GetSystemMetrics returns current system health and performance metrics
func (csm *CommonSystemsManager) GetSystemMetrics() *SystemMetrics {
	// Return a copy to prevent external modification
	metrics := *csm.systemMetrics
	metrics.ErrorCounts = make(map[string]int64)
	metrics.EventCounts = make(map[string]int64)

	for k, v := range csm.systemMetrics.ErrorCounts {
		metrics.ErrorCounts[k] = v
	}
	for k, v := range csm.systemMetrics.EventCounts {
		metrics.EventCounts[k] = v
	}

	return &metrics
}

// GetCommonSystemsFactory returns the factory for views to create common systems
// This breaks the import cycle by allowing views to use the factory instead of
// the manager importing view packages
func (csm *CommonSystemsManager) GetCommonSystemsFactory() *common.SystemsFactory {
	return csm.factory
}

// startSystemMonitoring runs background monitoring of system health
func (csm *CommonSystemsManager) startSystemMonitoring(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			csm.systemMetrics.SystemUptime = time.Since(startTime)
			csm.evaluateSystemHealth()
		}
	}
}

// evaluateSystemHealth checks system health and takes actions if needed
func (csm *CommonSystemsManager) evaluateSystemHealth() {
	totalErrors := csm.systemMetrics.GlobalErrorCount

	// Log system health periodically
	if totalErrors > 0 && (totalErrors%10 == 0) {
		csm.Logger().Info("System health check",
			"total_errors", totalErrors,
			"active_view", csm.systemMetrics.ActiveView,
			"view_switches", csm.systemMetrics.ViewSwitchCount,
			"uptime", csm.systemMetrics.SystemUptime)
	}

	// Could implement more sophisticated health checks here:
	// - Memory usage monitoring
	// - Error rate thresholds
	// - Performance degradation detection
	// - Automatic recovery actions
}

package common

import (
	"github.com/tpelletiersophos/cloudcutter/internal/logger"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/common/errors"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/common/events"
)

// SystemsFactory creates common systems without import cycles
// This factory can be injected into views without creating dependencies
type SystemsFactory struct {
	logger      *logger.Logger
	errorConfig *errors.ErrorHandlerConfig
	eventConfig *events.EventManagerConfig
}

// NewSystemsFactory creates a factory for common systems
func NewSystemsFactory(logger *logger.Logger) *SystemsFactory {
	return &SystemsFactory{
		logger: logger,
		errorConfig: &errors.ErrorHandlerConfig{
			LogStackTrace:     true,
			LogMetadata:       true,
			MaxStackDepth:     10,
			EnableUserMetrics: false,
			Component:         "ui-view", // Default, will be overridden
		},
		eventConfig: &events.EventManagerConfig{
			EnableLogging:        true,
			EnableMetrics:        true,
			EnableTracing:        false,
			LogUnhandledEvents:   true,
			MetricsFlushInterval: 30,
			MaxEventHistory:      1000,
			DebugMode:            false,
		},
	}
}

// CreateErrorHandler creates a configured error handler for a specific component
func (f *SystemsFactory) CreateErrorHandler(component string) *errors.ErrorHandler {
	config := *f.errorConfig // Copy the config
	config.Component = component
	return errors.NewErrorHandler(f.logger, &config)
}

// CreateEventConfig creates a configured event manager config for a specific component
func (f *SystemsFactory) CreateEventConfig() *events.EventManagerConfig {
	config := *f.eventConfig // Copy the config
	return &config
}

// UpdateErrorConfig allows the manager to update global error configuration
func (f *SystemsFactory) UpdateErrorConfig(config *errors.ErrorHandlerConfig) {
	f.errorConfig = config
}

// UpdateEventConfig allows the manager to update global event configuration
func (f *SystemsFactory) UpdateEventConfig(config *events.EventManagerConfig) {
	f.eventConfig = config
}

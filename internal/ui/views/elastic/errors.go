package elastic

import (
	commonErrors "github.com/tpelletiersophos/cloudcutter/internal/ui/common/errors"
)

// Elastic-specific error handler that extends the common error handler
type ElasticErrorHandler struct {
	*commonErrors.ErrorHandler
}

// NewElasticErrorHandler creates an error handler configured for elastic view
func NewElasticErrorHandler(logger commonErrors.Logger) *ElasticErrorHandler {
	config := &commonErrors.ErrorHandlerConfig{
		LogStackTrace:     true,
		LogMetadata:       true,
		MaxStackDepth:     10,
		EnableUserMetrics: false,
		Component:         "elastic-view",
	}

	baseHandler := commonErrors.NewErrorHandler(logger, config)

	return &ElasticErrorHandler{
		ErrorHandler: baseHandler,
	}
}

// Elastic-specific convenience methods that extend the base functionality

// NewSearchError creates a search-related error with appropriate categorization
func (eh *ElasticErrorHandler) NewSearchError(operation, message string, cause error) *commonErrors.ViewError {
	return eh.NewErrorWithOperation(commonErrors.ErrorCodeNetworkFailure, operation, message, cause)
}

// NewQueryError creates a query building/processing error
func (eh *ElasticErrorHandler) NewQueryError(operation, message string, cause error) *commonErrors.ViewError {
	return eh.NewErrorWithOperation(commonErrors.ErrorCodeEncodingFailure, operation, message, cause)
}

// NewProcessingError creates a data processing error
func (eh *ElasticErrorHandler) NewProcessingError(operation, message string, cause error) *commonErrors.ViewError {
	return eh.NewErrorWithOperation(commonErrors.ErrorCodeDecodingFailure, operation, message, cause)
}

// WrapSearchError wraps errors that occur during search operations
func (eh *ElasticErrorHandler) WrapSearchError(operation string, err error) *commonErrors.ViewError {
	if err == nil {
		return nil
	}

	// Check if it's already wrapped
	if e, ok := err.(*commonErrors.ViewError); ok {
		return e
	}

	// Use the existing network error wrapper which handles specific error types
	return eh.WrapNetworkError(operation, err)
}

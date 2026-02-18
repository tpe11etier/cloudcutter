package errors

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// ErrorHandlerConfig contains configuration for error handling behavior
type ErrorHandlerConfig struct {
	LogStackTrace     bool   `json:"log_stack_trace"`
	LogMetadata       bool   `json:"log_metadata"`
	MaxStackDepth     int    `json:"max_stack_depth"`
	EnableUserMetrics bool   `json:"enable_user_metrics"`
	Component         string `json:"component"`
}

// ErrorHandler provides centralized error handling functionality
type ErrorHandler struct {
	logger Logger
	config *ErrorHandlerConfig
}

// NewErrorHandler creates a new centralized error handler
func NewErrorHandler(logger Logger, config *ErrorHandlerConfig) *ErrorHandler {
	if config == nil {
		config = &ErrorHandlerConfig{
			LogStackTrace: true,
			LogMetadata:   true,
			MaxStackDepth: 10,
			Component:     "ui-view",
		}
	}
	return &ErrorHandler{
		logger: logger,
		config: config,
	}
}

// NewError creates a new ViewError with proper context
func (eh *ErrorHandler) NewError(code ErrorCode, message string, cause error) *ViewError {
	severity := eh.determineSeverity(code)

	return &ViewError{
		Code:        code,
		Message:     message,
		Severity:    severity,
		Cause:       cause,
		Context:     eh.captureContext(""),
		Recoverable: eh.isRecoverable(code),
		UserMessage: eh.generateUserMessage(code, message),
	}
}

// NewErrorWithOperation creates an error with specific operation context
func (eh *ErrorHandler) NewErrorWithOperation(code ErrorCode, operation, message string, cause error) *ViewError {
	severity := eh.determineSeverity(code)

	return &ViewError{
		Code:        code,
		Message:     message,
		Severity:    severity,
		Cause:       cause,
		Context:     eh.captureContext(operation),
		Recoverable: eh.isRecoverable(code),
		UserMessage: eh.generateUserMessage(code, message),
	}
}

// NewErrorWithMetadata creates an error with additional metadata
func (eh *ErrorHandler) NewErrorWithMetadata(code ErrorCode, operation, message string, cause error, metadata map[string]interface{}) *ViewError {
	err := eh.NewErrorWithOperation(code, operation, message, cause)
	if err.Context.Metadata == nil {
		err.Context.Metadata = make(map[string]interface{})
	}
	for k, v := range metadata {
		err.Context.Metadata[k] = v
	}
	return err
}

// HandleError processes an error through the centralized error handling pipeline
func (eh *ErrorHandler) HandleError(err error) {
	if err == nil {
		return
	}

	var viewErr *ViewError
	if e, ok := err.(*ViewError); ok {
		viewErr = e
	} else {
		// Wrap unknown errors
		viewErr = eh.NewError(ErrorCodeUnknownError, "Unhandled error occurred", err)
	}

	// Log the error
	eh.logError(viewErr)

	// Record metrics if enabled
	if eh.config.EnableUserMetrics {
		eh.recordMetrics(viewErr)
	}

	// Attempt recovery if possible
	if viewErr.Recoverable {
		eh.attemptRecovery(viewErr)
	}
}

// Standard Error Creation Convenience Methods

// WrapNetworkError creates a network-related error with appropriate categorization
func (eh *ErrorHandler) WrapNetworkError(operation string, cause error) *ViewError {
	code := ErrorCodeNetworkFailure
	message := fmt.Sprintf("Network error during %s", operation)

	// More specific categorization based on error content
	if cause != nil {
		causeStr := strings.ToLower(cause.Error())
		if strings.Contains(causeStr, "timeout") || strings.Contains(causeStr, "deadline") {
			code = ErrorCodeTimeout
			message = fmt.Sprintf("Timeout during %s", operation)
		} else if strings.Contains(causeStr, "429") || strings.Contains(causeStr, "rate limit") {
			code = ErrorCodeRateLimited
			message = fmt.Sprintf("Rate limited during %s", operation)
		} else if strings.Contains(causeStr, "connection refused") {
			code = ErrorCodeConnectionRefused
			message = fmt.Sprintf("Connection refused during %s", operation)
		}
	}

	return eh.NewErrorWithOperation(code, operation, message, cause)
}

// WrapDecodingError creates a decoding-related error
func (eh *ErrorHandler) WrapDecodingError(operation, dataType string, cause error) *ViewError {
	message := fmt.Sprintf("Failed to decode %s during %s", dataType, operation)
	metadata := map[string]interface{}{
		"data_type": dataType,
		"operation": operation,
	}
	return eh.NewErrorWithMetadata(ErrorCodeDecodingFailure, operation, message, cause, metadata)
}

// WrapValidationError creates a validation-related error
func (eh *ErrorHandler) WrapValidationError(field, value, reason string) *ViewError {
	message := fmt.Sprintf("Validation failed for field '%s': %s", field, reason)
	metadata := map[string]interface{}{
		"field":  field,
		"value":  value,
		"reason": reason,
	}
	return eh.NewErrorWithMetadata(ErrorCodeValidationFailure, "validation", message, nil, metadata)
}

// WrapJSONError wraps JSON marshaling/unmarshaling errors
func (eh *ErrorHandler) WrapJSONError(operation, action string, err error) *ViewError {
	if err == nil {
		return nil
	}

	message := fmt.Sprintf("JSON %s failed during %s", action, operation)
	metadata := map[string]interface{}{
		"action":    action,
		"operation": operation,
	}

	var code ErrorCode
	if action == "marshal" || action == "encoding" {
		code = ErrorCodeEncodingFailure
	} else {
		code = ErrorCodeDecodingFailure
	}

	return eh.NewErrorWithMetadata(code, operation, message, err, metadata)
}

// WrapResponseError wraps HTTP response reading errors
func (eh *ErrorHandler) WrapResponseError(operation string, err error) *ViewError {
	if err == nil {
		return nil
	}

	message := fmt.Sprintf("Failed to read response during %s", operation)
	return eh.NewErrorWithOperation(ErrorCodeNetworkFailure, operation, message, err)
}

// WrapWithOperation is a convenience method to wrap any error with operation context
func (eh *ErrorHandler) WrapWithOperation(operation string, err error) *ViewError {
	if err == nil {
		return nil
	}

	// If it's already a ViewError, just update the operation
	if e, ok := err.(*ViewError); ok {
		e.Context.Operation = operation
		return e
	}

	// Wrap as unknown error with operation context
	return eh.NewErrorWithOperation(ErrorCodeUnknownError, operation, "Operation failed", err)
}

// WithContext adds context information to an error
func (eh *ErrorHandler) WithContext(err error, ctx context.Context) *ViewError {
	if err == nil {
		return nil
	}

	var viewErr *ViewError
	if e, ok := err.(*ViewError); ok {
		viewErr = e
	} else {
		viewErr = eh.NewError(ErrorCodeUnknownError, "Error with context", err)
	}

	// Extract context information if available
	if ctx != nil {
		if reqID := ctx.Value("request_id"); reqID != nil {
			if viewErr.Context.Metadata == nil {
				viewErr.Context.Metadata = make(map[string]interface{})
			}
			viewErr.Context.Metadata["request_id"] = reqID
		}
	}

	return viewErr
}

// HandleAndLog processes an error and logs it appropriately, returning a user-safe error
func (eh *ErrorHandler) HandleAndLog(err error) *ViewError {
	if err == nil {
		return nil
	}

	var viewErr *ViewError
	if e, ok := err.(*ViewError); ok {
		viewErr = e
	} else {
		viewErr = eh.NewError(ErrorCodeUnknownError, "An error occurred", err)
	}

	// Process through the centralized pipeline
	eh.HandleError(viewErr)
	return viewErr
}

// RetryableError checks if an error is retryable and provides retry logic
func (eh *ErrorHandler) RetryableError(err error) (bool, time.Duration) {
	if err == nil {
		return false, 0
	}

	var viewErr *ViewError
	if e, ok := err.(*ViewError); ok {
		viewErr = e
	} else {
		return false, 0 // Unknown errors are not retryable by default
	}

	if !viewErr.Recoverable {
		return false, 0
	}

	// Determine retry delay based on error type
	switch viewErr.Code {
	case ErrorCodeRateLimited:
		return true, 5 * time.Second
	case ErrorCodeTimeout:
		return true, 2 * time.Second
	case ErrorCodeNetworkFailure:
		return true, 1 * time.Second
	case ErrorCodeUserInputError:
		return true, 0 // Immediate retry for user input issues
	default:
		return true, 1 * time.Second
	}
}

// Private helper methods

// captureContext captures the current execution context
func (eh *ErrorHandler) captureContext(operation string) ErrorContext {
	context := ErrorContext{
		Component: eh.config.Component,
		Operation: operation,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Capture stack trace if enabled
	if eh.config.LogStackTrace {
		context.StackTrace = eh.captureStackTrace()
	}

	return context
}

// captureStackTrace captures the current stack trace
func (eh *ErrorHandler) captureStackTrace() string {
	var stack strings.Builder
	pcs := make([]uintptr, eh.config.MaxStackDepth)
	n := runtime.Callers(3, pcs) // Skip captureStackTrace, captureContext, and NewError

	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		stack.WriteString(fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function))
		if !more {
			break
		}
	}

	return stack.String()
}

// determineSeverity determines the severity level based on error code
func (eh *ErrorHandler) determineSeverity(code ErrorCode) ErrorSeverity {
	switch code {
	case ErrorCodeConnectionRefused, ErrorCodeConfigurationError, ErrorCodeInitializationError:
		return SeverityCritical
	case ErrorCodeNetworkFailure, ErrorCodeTimeout, ErrorCodeStateInconsistency, ErrorCodeInternalError:
		return SeverityHigh
	case ErrorCodeRateLimited, ErrorCodeDecodingFailure, ErrorCodeValidationFailure, ErrorCodeUIRenderError:
		return SeverityMedium
	case ErrorCodeUserInputError, ErrorCodeResourceNotFound:
		return SeverityLow
	default:
		return SeverityMedium
	}
}

// isRecoverable determines if an error can be recovered from
func (eh *ErrorHandler) isRecoverable(code ErrorCode) bool {
	switch code {
	case ErrorCodeRateLimited, ErrorCodeTimeout, ErrorCodeUserInputError, ErrorCodeResourceNotFound:
		return true
	case ErrorCodeConnectionRefused, ErrorCodeConfigurationError, ErrorCodeInitializationError, ErrorCodeInternalError:
		return false
	default:
		return true // Optimistic recovery for unknown errors
	}
}

// generateUserMessage creates a user-friendly error message
func (eh *ErrorHandler) generateUserMessage(code ErrorCode, message string) string {
	switch code {
	case ErrorCodeNetworkFailure:
		return "Unable to connect to the service. Please check your connection."
	case ErrorCodeTimeout:
		return "The operation timed out. Please try again."
	case ErrorCodeRateLimited:
		return "Too many requests. Please wait a moment and try again."
	case ErrorCodeValidationFailure:
		return "Please check your input and try again."
	case ErrorCodeResourceNotFound:
		return "The requested resource was not found."
	case ErrorCodePermissionDenied:
		return "You don't have permission to perform this action."
	default:
		return "An error occurred. Please try again or contact support if the problem persists."
	}
}

// logError logs the error with appropriate detail level
func (eh *ErrorHandler) logError(err *ViewError) {
	logArgs := []interface{}{
		"code", err.Code,
		"severity", err.Severity,
		"message", err.Message,
		"component", err.Context.Component,
		"operation", err.Context.Operation,
		"recoverable", err.Recoverable,
	}

	if err.Cause != nil {
		logArgs = append(logArgs, "cause", err.Cause.Error())
	}

	if eh.config.LogMetadata && len(err.Context.Metadata) > 0 {
		logArgs = append(logArgs, "metadata", err.Context.Metadata)
	}

	if eh.config.LogStackTrace && err.Context.StackTrace != "" {
		logArgs = append(logArgs, "stack_trace", err.Context.StackTrace)
	}

	// Log at appropriate level based on severity
	switch err.Severity {
	case SeverityCritical, SeverityHigh:
		eh.logger.Error("UI view error", logArgs...)
	case SeverityMedium:
		eh.logger.Info("UI view warning", logArgs...)
	case SeverityLow, SeverityInfo:
		eh.logger.Debug("UI view info", logArgs...)
	}
}

// recordMetrics records error metrics for monitoring
func (eh *ErrorHandler) recordMetrics(err *ViewError) {
	// TODO: Implement metrics recording
	// This could integrate with Prometheus, StatsD, or other metrics systems
}

// attemptRecovery attempts to recover from recoverable errors
func (eh *ErrorHandler) attemptRecovery(err *ViewError) {
	switch err.Code {
	case ErrorCodeRateLimited:
		// Could implement retry logic or backoff
		eh.logger.Debug("Rate limit error - recovery strategy could be implemented",
			"code", err.Code, "operation", err.Context.Operation)
	case ErrorCodeTimeout:
		// Could implement retry with longer timeout
		eh.logger.Debug("Timeout error - recovery strategy could be implemented",
			"code", err.Code, "operation", err.Context.Operation)
	default:
		eh.logger.Debug("Recoverable error - no specific recovery strategy defined",
			"code", err.Code, "operation", err.Context.Operation)
	}
}

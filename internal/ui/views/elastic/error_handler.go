package elastic

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// ErrorCode represents different types of errors in the elastic view
type ErrorCode string

const (
	// Network and connectivity errors
	ErrorCodeNetworkFailure    ErrorCode = "NETWORK_FAILURE"
	ErrorCodeTimeout           ErrorCode = "TIMEOUT"
	ErrorCodeRateLimited       ErrorCode = "RATE_LIMITED"
	ErrorCodeConnectionRefused ErrorCode = "CONNECTION_REFUSED"

	// Data processing errors
	ErrorCodeInvalidData     ErrorCode = "INVALID_DATA"
	ErrorCodeDecodingFailure ErrorCode = "DECODING_FAILURE"
	ErrorCodeEncodingFailure ErrorCode = "ENCODING_FAILURE"
	ErrorCodeParsingFailure  ErrorCode = "PARSING_FAILURE"

	// Business logic errors
	ErrorCodeValidationFailure  ErrorCode = "VALIDATION_FAILURE"
	ErrorCodeStateInconsistency ErrorCode = "STATE_INCONSISTENCY"
	ErrorCodeResourceNotFound   ErrorCode = "RESOURCE_NOT_FOUND"
	ErrorCodePermissionDenied   ErrorCode = "PERMISSION_DENIED"

	// Configuration and initialization errors
	ErrorCodeConfigurationError  ErrorCode = "CONFIGURATION_ERROR"
	ErrorCodeInitializationError ErrorCode = "INITIALIZATION_ERROR"

	// UI and interaction errors
	ErrorCodeUserInputError ErrorCode = "USER_INPUT_ERROR"
	ErrorCodeUIRenderError  ErrorCode = "UI_RENDER_ERROR"

	// System errors
	ErrorCodeInternalError ErrorCode = "INTERNAL_ERROR"
	ErrorCodeUnknownError  ErrorCode = "UNKNOWN_ERROR"
)

// ErrorSeverity indicates the severity level of an error
type ErrorSeverity string

const (
	SeverityCritical ErrorSeverity = "CRITICAL" // System cannot continue
	SeverityHigh     ErrorSeverity = "HIGH"     // Major functionality affected
	SeverityMedium   ErrorSeverity = "MEDIUM"   // Some functionality affected
	SeverityLow      ErrorSeverity = "LOW"      // Minor issue, graceful degradation
	SeverityInfo     ErrorSeverity = "INFO"     // Informational, not really an error
)

// ErrorContext provides rich context information for errors
type ErrorContext struct {
	Component  string                 `json:"component"`   // Which component the error occurred in
	Operation  string                 `json:"operation"`   // What operation was being performed
	Timestamp  time.Time              `json:"timestamp"`   // When the error occurred
	StackTrace string                 `json:"stack_trace"` // Stack trace for debugging
	Metadata   map[string]interface{} `json:"metadata"`    // Additional context data
	UserID     string                 `json:"user_id"`     // User context if available
	RequestID  string                 `json:"request_id"`  // Request tracing ID
}

// ElasticViewError represents a comprehensive error with context and categorization
type ElasticViewError struct {
	Code        ErrorCode     `json:"code"`
	Message     string        `json:"message"`
	Severity    ErrorSeverity `json:"severity"`
	Cause       error         `json:"-"` // Original error (not serialized)
	Context     ErrorContext  `json:"context"`
	Recoverable bool          `json:"recoverable"`  // Whether this error can be recovered from
	UserMessage string        `json:"user_message"` // User-friendly message
}

// Error implements the error interface
func (e *ElasticViewError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Code, e.Severity, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Code, e.Severity, e.Message)
}

// Unwrap implements error unwrapping for Go 1.13+ error chains
func (e *ElasticViewError) Unwrap() error {
	return e.Cause
}

// Is implements error comparison for Go 1.13+ error chains
func (e *ElasticViewError) Is(target error) bool {
	if target == nil {
		return false
	}
	if te, ok := target.(*ElasticViewError); ok {
		return e.Code == te.Code
	}
	return false
}

// ErrorHandler provides centralized error handling functionality
type ErrorHandler struct {
	logger Logger
	config *ErrorHandlerConfig
}

// ErrorHandlerConfig contains configuration for error handling behavior
type ErrorHandlerConfig struct {
	LogStackTrace     bool   `json:"log_stack_trace"`
	LogMetadata       bool   `json:"log_metadata"`
	MaxStackDepth     int    `json:"max_stack_depth"`
	EnableUserMetrics bool   `json:"enable_user_metrics"`
	Component         string `json:"component"`
}

// NewErrorHandler creates a new centralized error handler
func NewErrorHandler(logger Logger, config *ErrorHandlerConfig) *ErrorHandler {
	if config == nil {
		config = &ErrorHandlerConfig{
			LogStackTrace: true,
			LogMetadata:   true,
			MaxStackDepth: 10,
			Component:     "elastic-view",
		}
	}
	return &ErrorHandler{
		logger: logger,
		config: config,
	}
}

// NewError creates a new ElasticViewError with proper context
func (eh *ErrorHandler) NewError(code ErrorCode, message string, cause error) *ElasticViewError {
	severity := eh.determineSeverity(code)

	return &ElasticViewError{
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
func (eh *ErrorHandler) NewErrorWithOperation(code ErrorCode, operation, message string, cause error) *ElasticViewError {
	severity := eh.determineSeverity(code)

	return &ElasticViewError{
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
func (eh *ErrorHandler) NewErrorWithMetadata(code ErrorCode, operation, message string, cause error, metadata map[string]interface{}) *ElasticViewError {
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

	var elasticErr *ElasticViewError
	if e, ok := err.(*ElasticViewError); ok {
		elasticErr = e
	} else {
		// Wrap unknown errors
		elasticErr = eh.NewError(ErrorCodeUnknownError, "Unhandled error occurred", err)
	}

	// Log the error
	eh.logError(elasticErr)

	// Record metrics if enabled
	if eh.config.EnableUserMetrics {
		eh.recordMetrics(elasticErr)
	}

	// Attempt recovery if possible
	if elasticErr.Recoverable {
		eh.attemptRecovery(elasticErr)
	}
}

// WrapNetworkError creates a network-related error with appropriate categorization
func (eh *ErrorHandler) WrapNetworkError(operation string, cause error) *ElasticViewError {
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
func (eh *ErrorHandler) WrapDecodingError(operation, dataType string, cause error) *ElasticViewError {
	message := fmt.Sprintf("Failed to decode %s during %s", dataType, operation)
	metadata := map[string]interface{}{
		"data_type": dataType,
		"operation": operation,
	}
	return eh.NewErrorWithMetadata(ErrorCodeDecodingFailure, operation, message, cause, metadata)
}

// WrapValidationError creates a validation-related error
func (eh *ErrorHandler) WrapValidationError(field, value, reason string) *ElasticViewError {
	message := fmt.Sprintf("Validation failed for field '%s': %s", field, reason)
	metadata := map[string]interface{}{
		"field":  field,
		"value":  value,
		"reason": reason,
	}
	return eh.NewErrorWithMetadata(ErrorCodeValidationFailure, "validation", message, nil, metadata)
}

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
		return "Unable to connect to Elasticsearch. Please check your connection."
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
func (eh *ErrorHandler) logError(err *ElasticViewError) {
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
		eh.logger.Error("Elastic view error", logArgs...)
	case SeverityMedium:
		eh.logger.Info("Elastic view warning", logArgs...)
	case SeverityLow, SeverityInfo:
		eh.logger.Debug("Elastic view info", logArgs...)
	}
}

// recordMetrics records error metrics for monitoring
func (eh *ErrorHandler) recordMetrics(err *ElasticViewError) {
	// TODO: Implement metrics recording
	// This could integrate with Prometheus, StatsD, or other metrics systems
}

// attemptRecovery attempts to recover from recoverable errors
func (eh *ErrorHandler) attemptRecovery(err *ElasticViewError) {
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

// WithContext adds context information to an error
func (eh *ErrorHandler) WithContext(err error, ctx context.Context) *ElasticViewError {
	if err == nil {
		return nil
	}

	var elasticErr *ElasticViewError
	if e, ok := err.(*ElasticViewError); ok {
		elasticErr = e
	} else {
		elasticErr = eh.NewError(ErrorCodeUnknownError, "Error with context", err)
	}

	// Extract context information if available
	if ctx != nil {
		if reqID := ctx.Value("request_id"); reqID != nil {
			if elasticErr.Context.Metadata == nil {
				elasticErr.Context.Metadata = make(map[string]interface{})
			}
			elasticErr.Context.Metadata["request_id"] = reqID
		}
	}

	return elasticErr
}

// Standard Error Creation Convenience Methods

// NewSearchError creates a search-related error with appropriate categorization
func (eh *ErrorHandler) NewSearchError(operation, message string, cause error) *ElasticViewError {
	return eh.NewErrorWithOperation(ErrorCodeNetworkFailure, operation, message, cause)
}

// NewQueryError creates a query building/processing error
func (eh *ErrorHandler) NewQueryError(operation, message string, cause error) *ElasticViewError {
	return eh.NewErrorWithOperation(ErrorCodeEncodingFailure, operation, message, cause)
}

// NewProcessingError creates a data processing error
func (eh *ErrorHandler) NewProcessingError(operation, message string, cause error) *ElasticViewError {
	return eh.NewErrorWithOperation(ErrorCodeDecodingFailure, operation, message, cause)
}

// NewConfigError creates a configuration-related error
func (eh *ErrorHandler) NewConfigError(message string, cause error) *ElasticViewError {
	return eh.NewError(ErrorCodeConfigurationError, message, cause)
}

// NewInternalError creates an internal system error
func (eh *ErrorHandler) NewInternalError(operation, message string, cause error) *ElasticViewError {
	return eh.NewErrorWithOperation(ErrorCodeInternalError, operation, message, cause)
}

// Standard Error Wrapping Functions

// WrapWithOperation is a convenience method to wrap any error with operation context
func (eh *ErrorHandler) WrapWithOperation(operation string, err error) *ElasticViewError {
	if err == nil {
		return nil
	}

	// If it's already an ElasticViewError, just update the operation
	if e, ok := err.(*ElasticViewError); ok {
		e.Context.Operation = operation
		return e
	}

	// Wrap as unknown error with operation context
	return eh.NewErrorWithOperation(ErrorCodeUnknownError, operation, "Operation failed", err)
}

// WrapSearchError wraps errors that occur during search operations
func (eh *ErrorHandler) WrapSearchError(operation string, err error) *ElasticViewError {
	if err == nil {
		return nil
	}

	// Check if it's already wrapped
	if e, ok := err.(*ElasticViewError); ok {
		return e
	}

	// Use the existing network error wrapper which handles specific error types
	return eh.WrapNetworkError(operation, err)
}

// WrapJSONError wraps JSON marshaling/unmarshaling errors
func (eh *ErrorHandler) WrapJSONError(operation, action string, err error) *ElasticViewError {
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
func (eh *ErrorHandler) WrapResponseError(operation string, err error) *ElasticViewError {
	if err == nil {
		return nil
	}

	message := fmt.Sprintf("Failed to read response during %s", operation)
	return eh.NewErrorWithOperation(ErrorCodeNetworkFailure, operation, message, err)
}

// Standard Error Handling Patterns

// HandleAndLog processes an error and logs it appropriately, returning a user-safe error
func (eh *ErrorHandler) HandleAndLog(err error) *ElasticViewError {
	if err == nil {
		return nil
	}

	var elasticErr *ElasticViewError
	if e, ok := err.(*ElasticViewError); ok {
		elasticErr = e
	} else {
		elasticErr = eh.NewError(ErrorCodeUnknownError, "An error occurred", err)
	}

	// Process through the centralized pipeline
	eh.HandleError(elasticErr)
	return elasticErr
}

// RetryableError checks if an error is retryable and provides retry logic
func (eh *ErrorHandler) RetryableError(err error) (bool, time.Duration) {
	if err == nil {
		return false, 0
	}

	var elasticErr *ElasticViewError
	if e, ok := err.(*ElasticViewError); ok {
		elasticErr = e
	} else {
		return false, 0 // Unknown errors are not retryable by default
	}

	if !elasticErr.Recoverable {
		return false, 0
	}

	// Determine retry delay based on error type
	switch elasticErr.Code {
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

package errors

import (
	"fmt"
	"time"
)

// Logger interface for dependency injection
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Close() error
}

// ErrorCode represents different types of errors
type ErrorCode string

const (
	ErrorCodeNetworkFailure    ErrorCode = "NETWORK_FAILURE"
	ErrorCodeTimeout           ErrorCode = "TIMEOUT"
	ErrorCodeRateLimited       ErrorCode = "RATE_LIMITED"
	ErrorCodeConnectionRefused ErrorCode = "CONNECTION_REFUSED"

	ErrorCodeInvalidData     ErrorCode = "INVALID_DATA"
	ErrorCodeDecodingFailure ErrorCode = "DECODING_FAILURE"
	ErrorCodeEncodingFailure ErrorCode = "ENCODING_FAILURE"
	ErrorCodeParsingFailure  ErrorCode = "PARSING_FAILURE"

	ErrorCodeValidationFailure  ErrorCode = "VALIDATION_FAILURE"
	ErrorCodeStateInconsistency ErrorCode = "STATE_INCONSISTENCY"
	ErrorCodeResourceNotFound   ErrorCode = "RESOURCE_NOT_FOUND"
	ErrorCodePermissionDenied   ErrorCode = "PERMISSION_DENIED"

	ErrorCodeConfigurationError  ErrorCode = "CONFIGURATION_ERROR"
	ErrorCodeInitializationError ErrorCode = "INITIALIZATION_ERROR"

	ErrorCodeUserInputError ErrorCode = "USER_INPUT_ERROR"
	ErrorCodeUIRenderError  ErrorCode = "UI_RENDER_ERROR"

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

// ViewError represents a comprehensive error with context and categorization
type ViewError struct {
	Code        ErrorCode     `json:"code"`
	Message     string        `json:"message"`
	Severity    ErrorSeverity `json:"severity"`
	Cause       error         `json:"-"` // Original error (not serialized)
	Context     ErrorContext  `json:"context"`
	Recoverable bool          `json:"recoverable"`  // Whether this error can be recovered from
	UserMessage string        `json:"user_message"` // User-friendly message
}

// Error implements the error interface
func (e *ViewError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Code, e.Severity, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Code, e.Severity, e.Message)
}

// Unwrap implements error unwrapping for Go 1.13+ error chains
func (e *ViewError) Unwrap() error {
	return e.Cause
}

// Is implements error comparison for Go 1.13+ error chains
func (e *ViewError) Is(target error) bool {
	if target == nil {
		return false
	}
	if te, ok := target.(*ViewError); ok {
		return e.Code == te.Code
	}
	return false
}

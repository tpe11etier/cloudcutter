package dynamodb

import (
	commonErrors "github.com/tpelletiersophos/cloudcutter/internal/ui/common/errors"
)

// DynamoDBErrorHandler extends the common error handler for DynamoDB-specific operations
type DynamoDBErrorHandler struct {
	*commonErrors.ErrorHandler
}

// NewDynamoDBErrorHandler creates an error handler configured for DynamoDB view
func NewDynamoDBErrorHandler(logger commonErrors.Logger) *DynamoDBErrorHandler {
	config := &commonErrors.ErrorHandlerConfig{
		LogStackTrace:     true,
		LogMetadata:       true,
		MaxStackDepth:     10,
		EnableUserMetrics: false,
		Component:         "dynamodb-view",
	}

	baseHandler := commonErrors.NewErrorHandler(logger, config)

	return &DynamoDBErrorHandler{
		ErrorHandler: baseHandler,
	}
}

// DynamoDB-specific convenience methods that extend the base functionality

// NewTableError creates a table-related error with appropriate categorization
func (eh *DynamoDBErrorHandler) NewTableError(operation, message string, cause error) *commonErrors.ViewError {
	return eh.NewErrorWithOperation(commonErrors.ErrorCodeNetworkFailure, operation, message, cause)
}

// NewItemError creates an item processing error
func (eh *DynamoDBErrorHandler) NewItemError(operation, message string, cause error) *commonErrors.ViewError {
	return eh.NewErrorWithOperation(commonErrors.ErrorCodeDecodingFailure, operation, message, cause)
}

// WrapTableError wraps errors that occur during table operations
func (eh *DynamoDBErrorHandler) WrapTableError(operation string, err error) *commonErrors.ViewError {
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
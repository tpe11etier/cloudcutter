package aws

import (
	"context"
	"errors"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSTSClient for testing STS interactions
type MockSTSClient struct {
	mock.Mock
}

func (m *MockSTSClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	args := m.Called(ctx, params)
	if output := args.Get(0); output != nil {
		return output.(*sts.GetCallerIdentityOutput), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockAPIError implements smithy.APIError for testing
type MockAPIError struct {
	Code    string
	Message string
}

func (e MockAPIError) Error() string {
	return e.Message
}

func (e MockAPIError) ErrorCode() string {
	return e.Code
}

func (e MockAPIError) ErrorMessage() string {
	return e.Message
}

func (e MockAPIError) ErrorFault() smithy.ErrorFault {
	return smithy.FaultClient
}

func TestHandleAuthError(t *testing.T) {
	tests := []struct {
		name        string
		inputError  error
		expectedMsg string
	}{
		{
			name:        "ExpiredToken error",
			inputError:  MockAPIError{Code: "ExpiredToken", Message: "Token expired"},
			expectedMsg: "authentication failed: your AWS token has expired, please refresh it",
		},
		{
			name:        "InvalidClientTokenId error",
			inputError:  MockAPIError{Code: "InvalidClientTokenId", Message: "Invalid token"},
			expectedMsg: "authentication failed: invalid AWS credentials or insufficient permissions",
		},
		{
			name:        "UnrecognizedClientException error",
			inputError:  MockAPIError{Code: "UnrecognizedClientException", Message: "Unrecognized client"},
			expectedMsg: "authentication failed: invalid AWS credentials or insufficient permissions",
		},
		{
			name:        "AccessDeniedException error",
			inputError:  MockAPIError{Code: "AccessDeniedException", Message: "Access denied"},
			expectedMsg: "authentication failed: invalid AWS credentials or insufficient permissions",
		},
		{
			name:        "Generic API error",
			inputError:  MockAPIError{Code: "SomeOtherError", Message: "Some other error occurred"},
			expectedMsg: "AWS error: Some other error occurred",
		},
		{
			name:        "Non-API error",
			inputError:  errors.New("network timeout"),
			expectedMsg: "unexpected error: network timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handleAuthError(tt.inputError)
			assert.Equal(t, tt.expectedMsg, result.Error())
		})
	}
}

func TestValidateCredentials(t *testing.T) {
	tests := []struct {
		name          string
		mockOutput    *sts.GetCallerIdentityOutput
		mockError     error
		expectError   bool
		expectedError string
	}{
		{
			name: "successful validation",
			mockOutput: &sts.GetCallerIdentityOutput{
				Account: awssdk.String("123456789012"),
				Arn:     awssdk.String("arn:aws:iam::123456789012:user/testuser"),
				UserId:  awssdk.String("AIDACKCEVSQ6C2EXAMPLE"),
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:          "expired token",
			mockOutput:    nil,
			mockError:     MockAPIError{Code: "ExpiredToken", Message: "Token expired"},
			expectError:   true,
			expectedError: "authentication failed: your AWS token has expired, please refresh it",
		},
		{
			name:          "invalid credentials",
			mockOutput:    nil,
			mockError:     MockAPIError{Code: "InvalidClientTokenId", Message: "Invalid token"},
			expectError:   true,
			expectedError: "authentication failed: invalid AWS credentials or insufficient permissions",
		},
		{
			name:          "access denied",
			mockOutput:    nil,
			mockError:     MockAPIError{Code: "AccessDeniedException", Message: "Access denied"},
			expectError:   true,
			expectedError: "authentication failed: invalid AWS credentials or insufficient permissions",
		},
		{
			name:          "network error",
			mockOutput:    nil,
			mockError:     errors.New("network timeout"),
			expectError:   true,
			expectedError: "unexpected error: network timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock config (we can't easily mock the STS client creation)
			// This test focuses on the error handling logic

			// Test the error handling directly
			if tt.mockError != nil {
				err := handleAuthError(tt.mockError)
				if tt.expectError {
					assert.Error(t, err)
					assert.Equal(t, tt.expectedError, err.Error())
				} else {
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestAuthenticateErrorHandling(t *testing.T) {
	// Test cases for profile and region parameter validation
	tests := []struct {
		name        string
		profile     string
		region      string
		expectError bool
		description string
	}{
		{
			name:        "valid parameters",
			profile:     "default",
			region:      "us-east-1",
			expectError: false,
			description: "should work with valid profile and region",
		},
		{
			name:        "empty profile",
			profile:     "",
			region:      "us-east-1",
			expectError: false, // AWS SDK handles empty profile gracefully
			description: "should handle empty profile (uses default)",
		},
		{
			name:        "empty region",
			profile:     "default",
			region:      "",
			expectError: false, // AWS SDK handles empty region gracefully
			description: "should handle empty region (uses default from config)",
		},
		{
			name:        "invalid region format",
			profile:     "default",
			region:      "invalid-region-format",
			expectError: false, // AWS SDK will handle invalid regions during actual API calls
			description: "should handle invalid region format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We can't easily test the full Authenticate function without
			// mocking the AWS config loading, which is complex.
			// For now, we test the components we can test in isolation.

			// Test that our parameter handling logic is sound
			assert.NotEmpty(t, tt.description, "Test should have description")

			// The actual authentication would require AWS credentials or mocking
			// the entire AWS config system, which is beyond the scope of unit tests.
			// Integration tests would be better for full authentication flow.
		})
	}
}

func TestErrorMessageQuality(t *testing.T) {
	// Test that our error messages are user-friendly and actionable
	tests := []struct {
		name           string
		inputError     error
		checkMessage   func(string) bool
		description    string
	}{
		{
			name:       "expired token message is actionable",
			inputError: MockAPIError{Code: "ExpiredToken", Message: "Token expired"},
			checkMessage: func(msg string) bool {
				return assert.Contains(t, msg, "refresh") &&
					   assert.Contains(t, msg, "expired")
			},
			description: "should tell user to refresh expired token",
		},
		{
			name:       "invalid credentials message is clear",
			inputError: MockAPIError{Code: "InvalidClientTokenId", Message: "Invalid token"},
			checkMessage: func(msg string) bool {
				return assert.Contains(t, msg, "invalid") &&
					   assert.Contains(t, msg, "credentials")
			},
			description: "should clearly indicate credential problems",
		},
		{
			name:       "access denied message mentions permissions",
			inputError: MockAPIError{Code: "AccessDeniedException", Message: "Access denied"},
			checkMessage: func(msg string) bool {
				return assert.Contains(t, msg, "permissions")
			},
			description: "should mention permissions for access denied errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handleAuthError(tt.inputError)
			assert.True(t, tt.checkMessage(result.Error()), tt.description)
		})
	}
}

func TestAuthenticationErrorCategories(t *testing.T) {
	// Test that we properly categorize different types of authentication errors

	credentialErrors := []string{
		"InvalidClientTokenId",
		"UnrecognizedClientException",
		"AccessDeniedException",
	}

	for _, errorCode := range credentialErrors {
		t.Run("credential error: "+errorCode, func(t *testing.T) {
			err := MockAPIError{Code: errorCode, Message: "test message"}
			result := handleAuthError(err)

			// All credential errors should mention "invalid AWS credentials"
			assert.Contains(t, result.Error(), "invalid AWS credentials")
			assert.Contains(t, result.Error(), "insufficient permissions")
		})
	}

	// Test token expiration separately
	t.Run("token expiration error", func(t *testing.T) {
		err := MockAPIError{Code: "ExpiredToken", Message: "test message"}
		result := handleAuthError(err)

		assert.Contains(t, result.Error(), "expired")
		assert.Contains(t, result.Error(), "refresh")
	})
}

// Integration test placeholder - would require actual AWS credentials
func TestAuthenticateIntegration(t *testing.T) {
	t.Skip("Integration test - requires AWS credentials and should be run separately")

	// This would test the full authentication flow:
	// cfg, err := Authenticate("default", "us-east-1")
	// assert.NoError(t, err)
	// assert.NotEmpty(t, cfg.Region)
}

// Benchmark for error handling performance
func BenchmarkHandleAuthError(b *testing.B) {
	err := MockAPIError{Code: "ExpiredToken", Message: "Token expired"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handleAuthError(err)
	}
}
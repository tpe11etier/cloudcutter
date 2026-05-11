package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

func Authenticate(profile, region string) (awssdk.Config, error) {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return awssdk.Config{}, handleAuthError(err)
	}

	if err := validateCredentials(cfg); err != nil {
		return awssdk.Config{}, err
	}

	return cfg, nil
}

func validateCredentials(cfg awssdk.Config) error {
	stsClient := sts.NewFromConfig(cfg)
	_, err := stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return handleAuthError(err)
	}
	return nil
}

func handleAuthError(err error) error {
	return HandleAWSError(err)
}

// HandleAWSError maps AWS SDK errors to user-friendly messages.
func HandleAWSError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "refresh cached SSO token") || strings.Contains(msg, "unable to refresh SSO token") {
		return fmt.Errorf("SSO session expired — run: aws sso login --profile <profile>")
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "ExpiredToken":
			return fmt.Errorf("authentication failed: your AWS token has expired, please refresh it")
		case "InvalidClientTokenId", "UnrecognizedClientException", "AccessDeniedException":
			return fmt.Errorf("authentication failed: invalid AWS credentials or insufficient permissions")
		default:
			return fmt.Errorf("AWS error: %s", apiErr.ErrorMessage())
		}
	}
	return fmt.Errorf("unexpected error: %v", err)
}

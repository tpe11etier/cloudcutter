package aws

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
	"os"
	"path/filepath"
	"strings"
)

func ListAWSCredentialsProfiles() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("unable to determine user home directory: %v", err)
	}
	credentialsPath := filepath.Join(homeDir, ".aws", "credentials")

	file, err := os.Open(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open credentials file: %v", err)
	}
	defer file.Close()

	var profiles []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			profile := strings.Trim(line, "[]")
			profiles = append(profiles, profile)
		}
	}

	return profiles, nil
}

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

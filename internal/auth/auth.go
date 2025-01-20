package auth

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

type Session struct {
	Config  aws.Config
	Profile string
	Region  string
}

type Authenticator struct {
	mu               sync.RWMutex
	currentSession   *Session
	isAuthenticating bool
	onStatus         func(string)
	opalConfig       OpalConfig
}

func New(statusFn func(string), opalConfig OpalConfig) *Authenticator {
	return &Authenticator{
		onStatus:   statusFn,
		opalConfig: opalConfig,
	}
}

func (a *Authenticator) IsAuthenticating() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isAuthenticating
}

func (a *Authenticator) SwitchProfile(ctx context.Context, profile, region string) (*Session, error) {
	a.mu.Lock()
	if a.isAuthenticating {
		a.mu.Unlock()
		return nil, fmt.Errorf("authentication already in progress")
	}
	a.isAuthenticating = true
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.isAuthenticating = false
		a.mu.Unlock()
	}()

	if a.currentSession != nil &&
		a.currentSession.Profile == profile &&
		a.currentSession.Region == region {
		return a.currentSession, nil
	}

	a.sendStatus(fmt.Sprintf("Switching to profile %s in %s", profile, region))
	var cfg aws.Config
	var err error

	switch profile {
	case "opal_prod":
		cfg, err = a.authenticateOpal(ctx, profile, region, a.opalConfig.ProdRoleID)
	case "opal_dev":
		cfg, err = a.authenticateOpal(ctx, profile, region, a.opalConfig.DevRoleID)
	case "local":
		cfg, err = a.authenticateLocal(ctx, region)
	default:
		cfg, err = a.authenticateStandard(ctx, profile, region)
	}
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	session := &Session{
		Config:  cfg,
		Profile: profile,
		Region:  region,
	}

	a.mu.Lock()
	a.currentSession = session
	a.mu.Unlock()

	return session, nil
}

func (a *Authenticator) authenticateStandard(ctx context.Context, profile, region string) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	if profile != "" && profile != "default" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	return config.LoadDefaultConfig(ctx, opts...)
}

func (a *Authenticator) authenticateOpal(ctx context.Context, profile, region, roleID string) (aws.Config, error) {
	if err := a.runOpalCommand(ctx, roleID, profile); err != nil {
		return aws.Config{}, err
	}

	return a.authenticateStandard(ctx, profile, region)
}

func (a *Authenticator) runOpalCommand(ctx context.Context, roleID, profileName string) error {
	cmd := exec.Command("opal", "iam-roles:start", "--id", roleID, "--profileName", profileName)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	a.sendStatus("Starting Opal authentication...")
	if err := cmd.Run(); err != nil {
		output := stdout.String() + stderr.String()

		if strings.Contains(output, "Enter your email") ||
			strings.Contains(output, "session is invalid or expired") {
			return fmt.Errorf("opal session expired. Please run '%s' in terminal first", profileName)
		}

		return fmt.Errorf("Opal command failed: %v\nOutput: %s", err, output)
	}

	a.sendStatus("Opal authentication completed successfully")
	return nil
}

func (a *Authenticator) Current() *Session {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentSession
}

func (a *Authenticator) sendStatus(status string) {
	if a.onStatus != nil {
		a.onStatus(status)
	}
}

func (a *Authenticator) authenticateLocal(ctx context.Context, region string) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion("local"),
	}
	return config.LoadDefaultConfig(ctx, opts...)
}

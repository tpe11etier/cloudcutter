package manager

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsservice "github.com/tpelletiersophos/cloudcutter/internal/services/aws"
)

type ProfileManager struct {
	currentConfig    aws.Config
	currentProfile   string
	isAuthenticating bool
	authMutex        sync.Mutex
	opalConfig       OpalConfig
	statusChan       chan<- string
}

func NewProfileManager() *ProfileManager {
	return &ProfileManager{
		opalConfig: LoadOpalConfig(),
	}
}

func (pm *ProfileManager) sendStatus(status string) {
	select {
	case pm.statusChan <- status:
	default:
	}
}

func (pm *ProfileManager) SwitchProfile(ctx context.Context, profile string, callback func(aws.Config, error)) {
	//if pm.currentProfile == profile {
	//	callback(pm.currentConfig, nil)
	//	return
	//}

	switch profile {
	case "opal_prod":
		pm.authenticateWithOpal(ctx, pm.opalConfig.ProdRoleID, profile, callback)
	case "opal_dev":
		pm.authenticateWithOpal(ctx, pm.opalConfig.DevRoleID, profile, callback)
	default:
		pm.authenticateStandard(ctx, profile, pm.currentConfig.Region, callback)

	}
}

func (pm *ProfileManager) authenticateStandard(ctx context.Context, profile, region string, callback func(aws.Config, error)) {
	pm.sendStatus(fmt.Sprintf("Authenticating with profile %s in region %s...", profile, region))

	cfg, err := awsservice.Authenticate(profile, region)
	if err != nil {
		pm.sendStatus(fmt.Sprintf("Authentication failed: %v", err))
		callback(aws.Config{}, err)
		return
	}

	pm.setConfig(cfg, profile)
	callback(cfg, nil)
}

func (pm *ProfileManager) authenticateWithOpal(ctx context.Context, roleID, profileName string, callback func(aws.Config, error)) {
	pm.authMutex.Lock()
	if pm.isAuthenticating {
		pm.authMutex.Unlock()
		callback(aws.Config{}, fmt.Errorf("authentication already in progress"))
		return
	}
	pm.isAuthenticating = true
	pm.authMutex.Unlock()

	go func() {
		defer func() {
			pm.authMutex.Lock()
			pm.isAuthenticating = false
			pm.authMutex.Unlock()
		}()

		// First run Opal to set up the credentials
		err := pm.runOpalCommand(ctx, roleID, profileName)
		if err != nil {
			callback(aws.Config{}, err)
			return
		}

		cfg, err := awsservice.Authenticate(profileName, "us-west-2")
		if err != nil {
			errMsg := fmt.Sprintf("Failed to load AWS config after Opal: %v", err)
			pm.sendStatus(errMsg)
			callback(aws.Config{}, fmt.Errorf(errMsg))
			return
		}

		pm.setConfig(cfg, profileName)
		callback(cfg, nil)
	}()
}

func (pm *ProfileManager) runOpalCommand(ctx context.Context, roleID, profileName string) error {
	cmd := exec.Command("opal", "iam-roles:start", "--id", roleID, "--profileName", profileName)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	pm.sendStatus("Starting Opal authentication...")
	if err := cmd.Run(); err != nil {
		output := stdout.String() + stderr.String()

		if strings.Contains(output, "Enter your email") ||
			strings.Contains(output, "session is invalid or expired") {
			return fmt.Errorf("Opal session expired. Please run 'opal-%s' in terminal first", profileName)
		}

		return fmt.Errorf("Opal command failed: %v\nOutput: %s", err, output)
	}

	pm.sendStatus("Opal authentication completed successfully")
	return nil
}

func (pm *ProfileManager) setConfig(cfg aws.Config, profile string) {
	pm.currentConfig = cfg
	pm.currentProfile = profile
}

func (pm *ProfileManager) IsAuthenticating() bool {
	pm.authMutex.Lock()
	defer pm.authMutex.Unlock()
	return pm.isAuthenticating
}

package profile

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpelletiersophos/cloudcutter/internal/auth"
)

type Handler struct {
	auth        *auth.Authenticator
	statusChan  chan<- string
	mu          sync.RWMutex
	region      string
	onLoadStart func(msg string)
	onLoadEnd   func()
}

func NewProfileHandler(statusChan chan<- string, onLoadStart func(string), onLoadEnd func()) (*Handler, error) {
	ph := &Handler{
		statusChan:  statusChan,
		region:      "us-west-2",
		onLoadStart: onLoadStart,
		onLoadEnd:   onLoadEnd,
	}

	authenticator, err := auth.New(ph.sendStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticator: %w", err)
	}

	ph.auth = authenticator
	return ph, nil
}

func (ph *Handler) sendStatus(status string) {
	select {
	case ph.statusChan <- status:
	default:
	}
}

func (ph *Handler) SwitchProfile(ctx context.Context, profile string, callback func(aws.Config, error)) {
	if ph.onLoadStart != nil {
		ph.onLoadStart(fmt.Sprintf("Authenticating profile: %s", profile))
	}

	go func() {
		defer func() {
			if ph.onLoadEnd != nil {
				ph.onLoadEnd()
			}
		}()

		session, err := ph.auth.SwitchProfile(ctx, profile, ph.region)
		if err != nil {
			ph.sendStatus(fmt.Sprintf("Authentication failed: %v", err))
			callback(aws.Config{}, err)
			return
		}

		ph.sendStatus(fmt.Sprintf("Successfully authenticated with profile: %s", profile))
		callback(session.Config, nil)
	}()
}

func (ph *Handler) GetCurrentProfile() string {
	if session := ph.auth.Current(); session != nil {
		return session.Profile
	}
	return ""
}

func (ph *Handler) IsAuthenticating() bool {
	return ph.auth.IsAuthenticating()
}

func (ph *Handler) SetRegion(region string) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.region = region
}

func (ph *Handler) GetRegion() string {
	ph.mu.RLock()
	defer ph.mu.RUnlock()
	return ph.region
}

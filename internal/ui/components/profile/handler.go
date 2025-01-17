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

func NewProfileHandler(statusChan chan<- string, onLoadStart func(string), onLoadEnd func()) *Handler {
	ph := &Handler{
		statusChan:  statusChan,
		region:      "us-west-2",
		onLoadStart: onLoadStart,
		onLoadEnd:   onLoadEnd,
	}

	oc := auth.LoadOpalConfig()
	ph.auth = auth.New(ph.sendStatus, oc)
	return ph
}

func (ph *Handler) sendStatus(status string) {
	select {
	case ph.statusChan <- status:
	default:
	}
}

func (ph *Handler) SwitchProfile(ctx context.Context, profile string, callback func(aws.Config, error)) {
	if ph.onLoadStart != nil {
		// Show loading first, then start authentication
		ph.onLoadStart(fmt.Sprintf("Authenticating profile: %s", profile))
	}

	// Start async profile switch after loading is shown
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

// SetRegion updates the region setting
func (ph *Handler) SetRegion(region string) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.region = region
}

// GetRegion returns the current region setting
func (ph *Handler) GetRegion() string {
	ph.mu.RLock()
	defer ph.mu.RUnlock()
	return ph.region
}

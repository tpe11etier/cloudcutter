package profile

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpelletiersophos/cloudcutter/internal/auth"
)

type Handler struct {
	auth       *auth.Authenticator
	statusChan chan<- string
	mu         sync.RWMutex
	region     string
}

func NewProfileHandler(statusChan chan<- string) *Handler {
	ph := &Handler{
		statusChan: statusChan,
		region:     "us-west-2",
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
	// Start async profile switch
	go func() {
		session, err := ph.auth.SwitchProfile(ctx, profile, ph.region)
		if err != nil {
			callback(aws.Config{}, err)
			return
		}

		callback(session.Config, nil)
	}()
}

func (ph *Handler) GetCurrentProfile() string {
	if session := ph.auth.Current(); session != nil {
		return session.Profile
	}
	return ""
}

func (ph *Handler) GetCurrentConfig() aws.Config {
	if session := ph.auth.Current(); session != nil {
		return session.Config
	}
	return aws.Config{}
}

func (ph *Handler) SetRegion(region string) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.region = region
}

func (ph *Handler) IsAuthenticating() bool {
	return ph.auth.IsAuthenticating()
}

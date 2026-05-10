package profile

import (
	"fmt"
	"sync"

	"github.com/tpe11etier/cloudcutter/internal/auth"
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

func (ph *Handler) GetCurrentProfile() string {
	if session := ph.auth.Current(); session != nil {
		return session.Profile
	}
	return ""
}

// CurrentSession returns the active auth session (or nil if not yet
// authenticated). Used by views that need richer per-profile state than the
// SwitchEnvironment callback exposes — notably the Dragos token/base URL.
func (ph *Handler) CurrentSession() *auth.Session {
	return ph.auth.Current()
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

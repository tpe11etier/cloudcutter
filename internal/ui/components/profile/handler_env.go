package profile

import (
	"context"
	"fmt"

	"github.com/tpe11etier/cloudcutter/internal/auth"
	"github.com/tpe11etier/cloudcutter/internal/environments"
)

// SwitchEnvironment is the Environment-aware sibling of SwitchProfile.
// It runs auth.SwitchEnvironment in a goroutine and invokes the callback
// with the resulting Session (or an error) on the goroutine — callers
// are responsible for marshaling UI updates onto the tview main loop via
// QueueUpdateDraw.
//
// SwitchProfile is preserved for phases 1-3 callers; phase 5 deletes it
// once nothing references the legacy path.
func (ph *Handler) SwitchEnvironment(ctx context.Context, env environments.Environment, callback func(*auth.Session, error)) {
	if ph.onLoadStart != nil {
		ph.onLoadStart(fmt.Sprintf("Authenticating environment: %s", env.Name))
	}

	go func() {
		defer func() {
			if ph.onLoadEnd != nil {
				ph.onLoadEnd()
			}
		}()

		session, err := ph.auth.SwitchEnvironment(ctx, env)
		if err != nil {
			ph.sendStatus(fmt.Sprintf("Authentication failed: %v", err))
			callback(nil, err)
			return
		}

		ph.sendStatus(fmt.Sprintf("Successfully authenticated: %s", env.Name))
		callback(session, nil)
	}()
}

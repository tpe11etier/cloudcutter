package profile

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/tpelletiersophos/cloudcutter/internal/auth"
)

func TestNewProfileHandler(t *testing.T) {
	statusChan := make(chan string, 1)
	var loadStartCalled bool
	var loadEndCalled bool

	ph := NewProfileHandler(
		statusChan,
		func(msg string) { loadStartCalled = true },
		func() { loadEndCalled = true },
	)

	assert.NotNil(t, ph)
	assert.Equal(t, "us-west-2", ph.GetRegion())
	assert.NotNil(t, ph.auth)

	ph.onLoadStart("test message")
	assert.True(t, loadStartCalled)

	ph.onLoadEnd()
	assert.True(t, loadEndCalled)
}

func TestSwitchProfile_Success(t *testing.T) {
	statusChan := make(chan string, 10)
	var mu sync.Mutex
	var loadStartCalled bool
	var loadEndCalled bool
	var loadStartMsg string

	authenticator := auth.New(func(status string) {}, auth.OpalConfig{})

	ph := &Handler{
		auth:       authenticator,
		statusChan: statusChan,
		region:     "us-west-2",
		onLoadStart: func(msg string) {
			mu.Lock()
			loadStartCalled = true
			loadStartMsg = msg
			mu.Unlock()
		},
		onLoadEnd: func() {
			mu.Lock()
			loadEndCalled = true
			mu.Unlock()
		},
	}

	var resultCfg aws.Config
	var resultErr error
	wg := sync.WaitGroup{}
	wg.Add(1)

	ph.SwitchProfile(context.Background(), "local", func(cfg aws.Config, err error) {
		mu.Lock()
		resultCfg = cfg
		resultErr = err
		mu.Unlock()
		wg.Done()
	})

	wg.Wait()

	mu.Lock()
	assert.True(t, loadStartCalled, "loadStart should have been called")
	assert.Equal(t, "Authenticating profile: local", loadStartMsg)
	assert.True(t, loadEndCalled, "loadEnd should have been called")
	assert.NotEmpty(t, resultCfg.Region)
	assert.Nil(t, resultErr)
	mu.Unlock()
}

func TestSwitchProfile_Error(t *testing.T) {
	statusChan := make(chan string, 10)
	var mu sync.Mutex
	var loadEndCalled bool

	authenticator := auth.New(func(status string) {}, auth.OpalConfig{})

	ph := &Handler{
		auth:        authenticator,
		statusChan:  statusChan,
		region:      "us-west-2",
		onLoadStart: func(msg string) {},
		onLoadEnd: func() {
			mu.Lock()
			loadEndCalled = true
			mu.Unlock()
		},
	}

	var resultErr error
	wg := sync.WaitGroup{}
	wg.Add(1)

	// Use an invalid profile to trigger an error
	ph.SwitchProfile(context.Background(), "invalid_profile", func(cfg aws.Config, err error) {
		mu.Lock()
		resultErr = err
		mu.Unlock()
		wg.Done()
	})

	wg.Wait()

	mu.Lock()
	assert.True(t, loadEndCalled, "loadEnd should have been called")
	assert.NotNil(t, resultErr, "should have received an error")
	mu.Unlock()
}

func TestGetCurrentProfile(t *testing.T) {
	authenticator := auth.New(func(status string) {}, auth.OpalConfig{})
	ph := &Handler{
		auth:   authenticator,
		region: "us-west-2",
	}

	assert.Empty(t, ph.GetCurrentProfile())
}

func TestRegionOperations(t *testing.T) {
	ph := &Handler{
		region: "us-west-2",
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			newRegion := "us-east-" + string(rune('1'+idx))
			ph.SetRegion(newRegion)
		}(i)

		go func() {
			defer wg.Done()
			_ = ph.GetRegion()
		}()
	}

	wg.Wait()
	region := ph.GetRegion()
	assert.NotEmpty(t, region)
}

func TestSendStatus(t *testing.T) {
	statusChan := make(chan string, 1)
	ph := &Handler{
		statusChan: statusChan,
	}

	t.Run("send with available capacity", func(t *testing.T) {
		ph.sendStatus("test status")
		select {
		case status := <-statusChan:
			assert.Equal(t, "test status", status)
		case <-time.After(time.Second):
			t.Error("Timeout waiting for status")
		}
	})

	t.Run("non-blocking send when full", func(t *testing.T) {
		ph.sendStatus("test status 1") // Fill the channel
		ph.sendStatus("test status 2") // Should not block
	})
}

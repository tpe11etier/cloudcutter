package profile

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tpe11etier/cloudcutter/internal/auth"
)

func TestNewProfileHandler(t *testing.T) {
	statusChan := make(chan string, 1)
	var loadStartCalled bool
	var loadEndCalled bool

	ph, err := NewProfileHandler(
		statusChan,
		func(msg string) { loadStartCalled = true },
		func() { loadEndCalled = true },
	)

	assert.Nil(t, err)

	assert.NotNil(t, ph)
	assert.Equal(t, "us-west-2", ph.GetRegion())
	assert.NotNil(t, ph.auth)

	ph.onLoadStart("test message")
	assert.True(t, loadStartCalled)

	ph.onLoadEnd()
	assert.True(t, loadEndCalled)
}

func TestGetCurrentProfile(t *testing.T) {
	authenticator, err := auth.New(func(status string) {})
	assert.Nil(t, err)
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

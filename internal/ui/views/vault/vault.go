package vault

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/rivo/tview"
)

// VaultHealth holds basic health info from Vault
type VaultHealth struct {
	Initialized bool   `json:"initialized"`
	Sealed      bool   `json:"sealed"`
	Standby     bool   `json:"standby"`
	Version     string `json:"version"`
}

// GetVaultHealth queries the local Vault HTTP API for health
func GetVaultHealth(addr string) (*VaultHealth, error) {
	resp, err := http.Get(fmt.Sprintf("%s/v1/sys/health", addr))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}
	var health VaultHealth
	if err := json.Unmarshal(body, &health); err != nil {
		return nil, err
	}
	return &health, nil
}

// NewVaultStatusView returns a tview.Primitive showing Vault health
func NewVaultStatusView(addr string) tview.Primitive {
	text := tview.NewTextView().SetText("Checking Vault status...")
	go func() {
		health, err := GetVaultHealth(addr)
		if err != nil {
			text.SetText(fmt.Sprintf("Error: %v", err))
			return
		}
		status := fmt.Sprintf(
			"Vault Status\nInitialized: %v\nSealed: %v\nStandby: %v\nVersion: %s",
			health.Initialized, health.Sealed, health.Standby, health.Version,
		)
		text.SetText(status)
	}()
	return text
}

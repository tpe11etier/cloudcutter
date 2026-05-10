package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultConfigPath returns ~/.cloudcutter/config.yaml. Returns the empty
// string if the home directory can't be resolved — callers should fail
// loudly in that case.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cloudcutter", "config.yaml")
}

// EnsureExists writes the starter config to path if and only if path does
// not already exist. Returns wrote=true when it created the file. The
// parent directory is created with mode 0700 and the file with mode 0600.
func EnsureExists(path string) (bool, error) {
	if path == "" {
		return false, fmt.Errorf("config path is empty (could not resolve home directory)")
	}

	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(starterYAML), 0o600); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

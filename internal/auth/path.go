package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandHome resolves a leading "~" to the user's home directory.
// Anything else passes through unchanged.
//
// The YAML config schema documents tilde-prefixed paths
// (e.g. auth.path: ~/.cloudcutter/dragos.token) and users naturally
// write them, but Go's os.ReadFile/os.WriteFile do no shell-style
// expansion. ExpandHome bridges that gap.
func ExpandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand %q: %w", path, err)
		}
		if path == "~" {
			return home, nil
		}
		return home + path[1:], nil
	}
	return path, nil
}

// WriteTokenFile writes a token to path with mode 0600, creating the
// parent directory with mode 0700 if needed. ~/path is expanded.
//
// Used by the JWT login modal to persist the token at the env's
// auth.path location.
func WriteTokenFile(path, token string) error {
	expanded, err := ExpandHome(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(expanded), 0o700); err != nil {
		return fmt.Errorf("create parent dir of %s: %w", expanded, err)
	}
	if err := os.WriteFile(expanded, []byte(strings.TrimSpace(token)), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", expanded, err)
	}
	return nil
}

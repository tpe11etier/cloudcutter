package environments

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDiscoverAWSProfiles(t *testing.T) {
	home := t.TempDir()
	awsDir := filepath.Join(home, ".aws")
	if err := os.MkdirAll(awsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	creds := `[default]
aws_access_key_id = AKIA...
aws_secret_access_key = ...

[opal_dev]
aws_access_key_id = AKIA...
aws_secret_access_key = ...

[profile opal_prod]
aws_access_key_id = AKIA...
aws_secret_access_key = ...
`
	if err := os.WriteFile(filepath.Join(awsDir, "credentials"), []byte(creds), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverAWSProfiles(home)
	if err != nil {
		t.Fatalf("DiscoverAWSProfiles: %v", err)
	}
	sort.Strings(got)
	want := []string{"default", "opal_dev", "opal_prod"}
	if len(got) != len(want) {
		t.Fatalf("got %d profiles (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("profile[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDiscoverAWSProfilesMissingFile(t *testing.T) {
	home := t.TempDir()
	got, err := DiscoverAWSProfiles(home)
	if err != nil {
		t.Fatalf("expected nil error for missing creds file, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 profiles for missing creds file, got %v", got)
	}
}

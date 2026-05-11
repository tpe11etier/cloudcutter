package environments

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// DiscoverAWSProfiles reads ~/.aws/credentials and ~/.aws/config and returns
// the deduplicated list of profile names found in either file. Missing files
// are treated as "no profiles" rather than an error.
//
// homeDir is taken as a parameter so tests can supply a temp directory.
// In production callers, pass os.UserHomeDir()'s result.
func DiscoverAWSProfiles(homeDir string) ([]string, error) {
	seen := make(map[string]struct{})
	var names []string

	addProfiles := func(path string) error {
		f, err := ini.Load(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("read %s: %w", path, err)
		}
		for _, section := range f.Sections() {
			name := section.Name()
			if name == ini.DefaultSection && len(section.Keys()) == 0 {
				continue
			}
			// ~/.aws/config prefixes profiles with "profile "; skip sso-session blocks.
			if strings.HasPrefix(name, "sso-session") {
				continue
			}
			name = strings.TrimPrefix(name, "profile ")
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				names = append(names, name)
			}
		}
		return nil
	}

	if err := addProfiles(filepath.Join(homeDir, ".aws", "credentials")); err != nil {
		return nil, err
	}
	if err := addProfiles(filepath.Join(homeDir, ".aws", "config")); err != nil {
		return nil, err
	}
	return names, nil
}

package environments

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// DiscoverAWSProfiles reads ~/.aws/credentials and returns the list of
// profile names found there. A missing credentials file is treated as
// "no AWS profiles" rather than an error.
//
// homeDir is taken as a parameter so tests can supply a temp directory.
// In production callers, pass os.UserHomeDir()'s result.
func DiscoverAWSProfiles(homeDir string) ([]string, error) {
	path := filepath.Join(homeDir, ".aws", "credentials")
	f, err := ini.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var names []string
	for _, section := range f.Sections() {
		name := section.Name()
		if name == ini.DefaultSection && len(section.Keys()) == 0 {
			continue
		}
		// AWS config files prefix non-default profiles with "profile " in
		// ~/.aws/config but not ~/.aws/credentials. Tolerate both.
		name = strings.TrimPrefix(name, "profile ")
		names = append(names, name)
	}
	return names, nil
}

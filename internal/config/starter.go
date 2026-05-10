package config

import _ "embed"

//go:embed starter.yaml.tmpl
var starterYAML string

// StarterYAML returns the commented starter config written on first run.
func StarterYAML() string { return starterYAML }

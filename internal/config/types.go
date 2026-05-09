// Package config models the user-edited ~/.cloudcutter/config.yaml that
// describes how cloudcutter authenticates to and queries each backend
// environment. Phase 1 only loads + validates the config; runtime
// consumption arrives in phase 2.
package config

// Config is the top-level YAML document.
type Config struct {
	DefaultAWSBackend *EnvironmentTemplate `yaml:"default_aws_backend,omitempty"`
	Environments      []EnvironmentSpec    `yaml:"environments,omitempty"`
}

// EnvironmentTemplate is applied to every auto-discovered ~/.aws profile
// that doesn't have an explicit override under `environments`.
type EnvironmentTemplate struct {
	Vars         map[string][]VarRule `yaml:"vars,omitempty"`
	Auth         AuthSpec             `yaml:"auth"`
	Transport    TransportSpec        `yaml:"transport"`
	IndexPattern string               `yaml:"index_pattern,omitempty"`
	TimeFields   []TimeField          `yaml:"time_fields,omitempty"`
}

// EnvironmentSpec is one entry under `environments`. Region-agnostic — it
// gets materialized into an Environment with substitutions resolved at
// switch-profile time.
type EnvironmentSpec struct {
	Name         string               `yaml:"name"`
	Vars         map[string][]VarRule `yaml:"vars,omitempty"`
	Auth         AuthSpec             `yaml:"auth"`
	Transport    TransportSpec        `yaml:"transport"`
	IndexPattern string               `yaml:"index_pattern,omitempty"`
	TimeFields   []TimeField          `yaml:"time_fields,omitempty"`
}

// AuthSpec describes how to obtain credentials. Type chooses the dispatch:
//
//	none    — no auth (e.g. local docker)
//	aws_sdk — load AWS SDK creds; PreAuth optionally runs first
//	jwt     — fetch a token from env > path > login modal, attached via
//	          the transport's TokenHeader
type AuthSpec struct {
	Type    string       `yaml:"type"`
	Path    string       `yaml:"path,omitempty"`
	Env     string       `yaml:"env,omitempty"`
	Login   *LoginSpec   `yaml:"login,omitempty"`
	PreAuth *PreAuthSpec `yaml:"pre_auth,omitempty"`
}

// PreAuthSpec runs a shell command before AWS SDK auth (used for Opal).
type PreAuthSpec struct {
	Command              []string `yaml:"command"`
	DetectSessionExpired []string `yaml:"detect_session_expired,omitempty"`
}

// LoginSpec describes the in-app login form for jwt auth.
type LoginSpec struct {
	URL          string            `yaml:"url"`
	BodyFormat   string            `yaml:"body_format,omitempty"` // "json" (default) | "form"
	BodyFields   []FormField       `yaml:"body_fields"`
	Query        map[string]string `yaml:"query,omitempty"`
	TokenExtract TokenExtractSpec  `yaml:"token_extract"`
}

type FormField struct {
	Name string `yaml:"name"`
	Kind string `yaml:"kind"` // "text" | "password"
}

type TokenExtractSpec struct {
	From string `yaml:"from"` // "cookie" | "header" | "json_path"
	Name string `yaml:"name"`
}

// TransportSpec describes how each ES request is wrapped.
type TransportSpec struct {
	Type        string            `yaml:"type"` // "plain" | "sigv4" | "kibana_proxy"
	BaseURL     string            `yaml:"base_url,omitempty"`
	URLTemplate string            `yaml:"url_template,omitempty"` // for sigv4
	Service     string            `yaml:"service,omitempty"`      // for sigv4
	ProxyPath   string            `yaml:"proxy_path,omitempty"`   // for kibana_proxy
	TokenHeader *TokenHeaderSpec  `yaml:"token_header,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty"`
	Probe       *ProbeSpec        `yaml:"probe,omitempty"`
}

type TokenHeaderSpec struct {
	Name   string `yaml:"name"`
	Format string `yaml:"format"` // includes literal "{token}"
}

type ProbeSpec struct {
	Path       string `yaml:"path"`
	RejectHTML bool   `yaml:"reject_html,omitempty"`
}

type TimeField struct {
	Name   string `yaml:"name"`
	Format string `yaml:"format"` // "unix" | "unix_ms" | "date"
}

// VarRule is one entry inside a `vars[key]` list. First match (against the
// AWS profile name being resolved) wins. Either Value or Env must be set;
// Env names an environment variable to read.
type VarRule struct {
	Match string `yaml:"match"`
	Value string `yaml:"value,omitempty"`
	Env   string `yaml:"env,omitempty"`
}

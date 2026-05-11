// Package environments resolves a config.EnvironmentSpec into a fully-
// substituted Environment ready for Auth + Transport construction. The
// split exists because the user can change region in-app; spec is
// region-agnostic, Environment is region-bound.
package environments

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/tpe11etier/cloudcutter/internal/config"
)

// Environment is a fully-resolved description of one backend the user has
// selected. Every URL/command template is already substituted.
type Environment struct {
	Name         string
	Region       string
	Auth         config.AuthSpec
	Transport    config.TransportSpec
	AWSProfile   string
	IndexPattern string
	TimeFields   []config.TimeField
}

// Materialize substitutes {region}, {profile}, and any user-defined vars
// into the spec, returning a region-bound Environment. Region may be
// empty for non-AWS environments; templates that reference {region} when
// region is empty produce an explicit error.
func Materialize(spec config.EnvironmentSpec, region string) (Environment, error) {
	subs := map[string]string{
		"profile": spec.Name,
		"region":  region,
	}

	for key, rules := range spec.Vars {
		val, err := resolveVar(spec.Name, key, rules)
		if err != nil {
			return Environment{}, err
		}
		subs[key] = val
	}

	transport := spec.Transport // copy
	if transport.URLTemplate != "" {
		expanded, err := substitute(spec.Name, transport.URLTemplate, subs)
		if err != nil {
			return Environment{}, err
		}
		transport.URLTemplate = expanded
	}
	if transport.BaseURL != "" {
		expanded, err := substitute(spec.Name, transport.BaseURL, subs)
		if err != nil {
			return Environment{}, err
		}
		transport.BaseURL = expanded
	}

	auth := spec.Auth // copy
	if auth.PreAuth != nil {
		cmd := make([]string, len(auth.PreAuth.Command))
		for i, arg := range auth.PreAuth.Command {
			expanded, err := substitute(spec.Name, arg, subs)
			if err != nil {
				return Environment{}, err
			}
			cmd[i] = expanded
		}
		// Don't mutate the caller's PreAuthSpec; replace with a new value.
		newPre := *auth.PreAuth
		newPre.Command = cmd
		auth.PreAuth = &newPre
	}

	return Environment{
		Name:         spec.Name,
		Region:       region,
		Auth:         auth,
		Transport:    transport,
		AWSProfile:   spec.AWSProfile,
		IndexPattern: spec.IndexPattern,
		TimeFields:   spec.TimeFields,
	}, nil
}

func resolveVar(profileName, key string, rules []config.VarRule) (string, error) {
	for _, r := range rules {
		matched, err := regexp.MatchString(r.Match, profileName)
		if err != nil {
			return "", fmt.Errorf("vars.%s: invalid regex %q: %w", key, r.Match, err)
		}
		if !matched {
			continue
		}
		if r.Env != "" {
			val := os.Getenv(r.Env)
			if val == "" {
				return "", fmt.Errorf("vars.%s: env %s is unset (matched profile %q)", key, r.Env, profileName)
			}
			return val, nil
		}
		return r.Value, nil
	}
	return "", fmt.Errorf("vars.%s: no rule matched profile %q", key, profileName)
}

func substitute(profileName, template string, subs map[string]string) (string, error) {
	out := template
	matches := varRefRE.FindAllStringSubmatch(template, -1)
	for _, m := range matches {
		key := m[1]
		val, ok := subs[key]
		if !ok || val == "" {
			return "", fmt.Errorf("environment %q: template references {%s} but it isn't defined for this profile", profileName, key)
		}
		out = strings.ReplaceAll(out, m[0], val)
	}
	return out, nil
}

var varRefRE = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

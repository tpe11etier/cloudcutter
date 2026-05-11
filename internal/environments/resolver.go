package environments

import (
	"fmt"

	"github.com/tpe11etier/cloudcutter/internal/config"
)

// Resolver answers two questions: what environments can the picker show
// (List), and how does a chosen name resolve to a concrete spec (Resolve)?
//
// The picker source is the union of config.Environments names and the
// auto-discovered AWS profile names. Explicit `environments[name=X]`
// entries shadow same-named AWS profiles.
type Resolver struct {
	cfg         *config.Config
	awsProfiles []string

	// Index built once at construction time: name → environment spec.
	// AWS-profile-only entries are NOT pre-built; they're synthesized
	// lazily by Resolve so Vars stay live (could change after Reload).
	explicit map[string]*config.EnvironmentSpec
}

func NewResolver(cfg *config.Config, awsProfiles []string) *Resolver {
	r := &Resolver{cfg: cfg, awsProfiles: awsProfiles}
	r.explicit = make(map[string]*config.EnvironmentSpec, len(cfg.Environments))
	for i := range cfg.Environments {
		env := &cfg.Environments[i]
		r.explicit[env.Name] = env
	}
	return r
}

// List returns the names available in the picker. Explicit environments
// are always included. Auto-discovered AWS profiles are only included when
// default_aws_backend is defined — otherwise they have no config to back them.
func (r *Resolver) List() []string {
	seen := make(map[string]struct{}, len(r.explicit)+len(r.awsProfiles))
	out := make([]string, 0, len(r.explicit)+len(r.awsProfiles))
	for name := range r.explicit {
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if r.cfg.DefaultAWSBackend != nil {
		for _, name := range r.awsProfiles {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}

// Resolve returns the EnvironmentSpec for a name. Explicit entries win
// over AWS-profile fallback.
func (r *Resolver) Resolve(name string) (config.EnvironmentSpec, error) {
	if spec, ok := r.explicit[name]; ok {
		return *spec, nil
	}
	for _, p := range r.awsProfiles {
		if p != name {
			continue
		}
		if r.cfg.DefaultAWSBackend == nil {
			return config.EnvironmentSpec{}, fmt.Errorf(
				"profile %q has no environment definition. Add it under 'environments' in ~/.cloudcutter/config.yaml or define a default_aws_backend.",
				name)
		}
		return r.specFromTemplate(name, r.cfg.DefaultAWSBackend), nil
	}
	return config.EnvironmentSpec{}, fmt.Errorf("environment %q not found in config or ~/.aws/credentials", name)
}

func (r *Resolver) specFromTemplate(name string, t *config.EnvironmentTemplate) config.EnvironmentSpec {
	return config.EnvironmentSpec{
		Name:         name,
		Vars:         t.Vars,
		Auth:         t.Auth,
		Transport:    t.Transport,
		IndexPattern: t.IndexPattern,
		TimeFields:   t.TimeFields,
	}
}

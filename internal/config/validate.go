package config

import (
	"fmt"
	"regexp"
)

// Validate checks structural and semantic rules on a parsed Config. It
// does NOT verify that auth would succeed at runtime — only that the
// config is internally consistent and uses known enum values.
func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if cfg.DefaultAWSBackend != nil {
		if err := validateTemplate("default_aws_backend", cfg.DefaultAWSBackend); err != nil {
			return err
		}
	}
	for i := range cfg.Environments {
		env := &cfg.Environments[i]
		if env.Name == "" {
			return fmt.Errorf("environments[%d]: missing required field 'name'", i)
		}
		path := fmt.Sprintf("environments[%s]", env.Name)
		if err := validateAuth(path, &env.Auth); err != nil {
			return err
		}
		if env.Transport.Type != "" {
			if err := validateTransport(path, &env.Transport); err != nil {
				return err
			}
		}
		if err := validateVars(path, env.Vars); err != nil {
			return err
		}
		if err := validateTimeFields(path, env.TimeFields); err != nil {
			return err
		}
	}
	return nil
}

func validateTemplate(path string, t *EnvironmentTemplate) error {
	if err := validateAuth(path, &t.Auth); err != nil {
		return err
	}
	// Transport is optional in a template — omit it for DynamoDB-only backends.
	if t.Transport.Type != "" {
		if err := validateTransport(path, &t.Transport); err != nil {
			return err
		}
	}
	if err := validateVars(path, t.Vars); err != nil {
		return err
	}
	return validateTimeFields(path, t.TimeFields)
}

func validateAuth(path string, a *AuthSpec) error {
	switch a.Type {
	case "":
		return fmt.Errorf("%s: missing required field 'auth.type'", path)
	case "none":
		// nothing else required
	case "aws_sdk":
		if a.PreAuth != nil && len(a.PreAuth.Command) == 0 {
			return fmt.Errorf("%s.auth.pre_auth: 'command' is required when pre_auth is set", path)
		}
	case "jwt":
		if a.Path == "" && a.Env == "" && a.Login == nil {
			return fmt.Errorf("%s.auth: jwt requires at least one of 'path', 'env', or 'login'", path)
		}
		if a.Login != nil {
			if a.Login.URL == "" {
				return fmt.Errorf("%s.auth.login: 'url' is required", path)
			}
			if len(a.Login.BodyFields) == 0 {
				return fmt.Errorf("%s.auth.login: 'body_fields' must contain at least one field", path)
			}
			for i, f := range a.Login.BodyFields {
				switch f.Kind {
				case "text", "password":
				case "":
					return fmt.Errorf("%s.auth.login.body_fields[%d]: 'kind' is required", path, i)
				default:
					return fmt.Errorf("%s.auth.login.body_fields[%d]: unknown kind %q (want text|password)", path, i, f.Kind)
				}
			}
			switch a.Login.BodyFormat {
			case "", "json", "form":
			default:
				return fmt.Errorf("%s.auth.login: unknown body_format %q (want json|form)", path, a.Login.BodyFormat)
			}
			switch a.Login.TokenExtract.From {
			case "":
				return fmt.Errorf("%s.auth.login.token_extract: 'from' is required", path)
			case "cookie", "header":
				if a.Login.TokenExtract.Name == "" {
					return fmt.Errorf("%s.auth.login.token_extract: 'name' is required for from=%s", path, a.Login.TokenExtract.From)
				}
			case "json_path":
				return fmt.Errorf("%s.auth.login.token_extract: from=json_path not yet implemented", path)
			default:
				return fmt.Errorf("%s.auth.login.token_extract: unknown from %q (want cookie|header|json_path)", path, a.Login.TokenExtract.From)
			}
		}
	default:
		return fmt.Errorf("%s.auth: unknown type %q (want none|aws_sdk|jwt)", path, a.Type)
	}
	return nil
}

func validateTransport(path string, t *TransportSpec) error {
	switch t.Type {
	case "":
		return fmt.Errorf("%s: missing required field 'transport.type'", path)
	case "plain":
		if t.BaseURL == "" {
			return fmt.Errorf("%s.transport: plain requires 'base_url'", path)
		}
	case "sigv4":
		if t.Service == "" {
			return fmt.Errorf("%s.transport: sigv4 requires 'service'", path)
		}
		if t.URLTemplate == "" {
			return fmt.Errorf("%s.transport: sigv4 requires 'url_template'", path)
		}
	case "kibana_proxy":
		if t.BaseURL == "" {
			return fmt.Errorf("%s.transport: kibana_proxy requires 'base_url'", path)
		}
		if t.ProxyPath == "" {
			return fmt.Errorf("%s.transport: kibana_proxy requires 'proxy_path'", path)
		}
		if t.TokenHeader == nil || t.TokenHeader.Name == "" || t.TokenHeader.Format == "" {
			return fmt.Errorf("%s.transport: kibana_proxy requires 'token_header.name' and 'token_header.format'", path)
		}
	default:
		return fmt.Errorf("%s.transport: unknown type %q (want plain|sigv4|kibana_proxy)", path, t.Type)
	}
	return nil
}

func validateVars(path string, vars map[string][]VarRule) error {
	for key, rules := range vars {
		for i, r := range rules {
			if r.Match == "" {
				return fmt.Errorf("%s.vars.%s[%d]: 'match' is required", path, key, i)
			}
			if _, err := regexp.Compile(r.Match); err != nil {
				return fmt.Errorf("%s.vars.%s[%d]: invalid regex %q: %w", path, key, i, r.Match, err)
			}
			if r.Value == "" && r.Env == "" {
				return fmt.Errorf("%s.vars.%s[%d]: must set either 'value' or 'env'", path, key, i)
			}
			if r.Value != "" && r.Env != "" {
				return fmt.Errorf("%s.vars.%s[%d]: 'value' and 'env' are mutually exclusive", path, key, i)
			}
		}
	}
	return nil
}

func validateTimeFields(path string, fields []TimeField) error {
	for i, f := range fields {
		if f.Name == "" {
			return fmt.Errorf("%s.time_fields[%d]: 'name' is required", path, i)
		}
		switch f.Format {
		case "unix", "unix_ms", "date":
		default:
			return fmt.Errorf("%s.time_fields[%d]: unknown format %q (want unix|unix_ms|date)", path, i, f.Format)
		}
	}
	return nil
}

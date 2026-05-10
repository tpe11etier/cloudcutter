package auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	internalconfig "github.com/tpe11etier/cloudcutter/internal/config"
	"github.com/tpe11etier/cloudcutter/internal/environments"
	"github.com/tpe11etier/cloudcutter/internal/probe"
)

type Session struct {
	Config  aws.Config
	Profile string
	Region  string
	// Dragos is populated only when Profile == DragosProfile.
	// Phase 2: kept for backwards compatibility with view.go's
	// session.Dragos != nil checks. Phase 4 deletes this field once
	// callers consult Environment.Transport.Type instead.
	Dragos *DragosSession
	// Environment is the resolved description of the active backend.
	// Populated alongside the legacy fields by SwitchEnvironment, which
	// SwitchProfile now delegates to. Phase 2 readers may ignore this
	// field; phase 4 makes it the source of truth.
	Environment environments.Environment
	// Token is the JWT for jwt-typed environments. Empty for aws_sdk and
	// none. Populated by SwitchEnvironment from Auth.Env or Auth.Path.
	Token string
}

// DragosSession carries the state needed to talk to the Dragos Kibana endpoint.
// It is intentionally separate from aws.Config because the auth model has
// nothing in common with AWS SigV4.
type DragosSession struct {
	BaseURL      string
	AuthToken    string
	IndexPattern string
	KbnVersion   string
}

// DragosProfile is the profile name selected from the picker to use Dragos.
const DragosProfile = "dragos"

type Authenticator struct {
	mu               sync.RWMutex
	currentSession   *Session
	isAuthenticating bool
	onStatus         func(string)
	opalConfig       OpalConfig
	opalProfiles     map[string]string // maps profile names to role IDs
}

func New(statusFn func(string)) (*Authenticator, error) {
	opalConfig := LoadOpalConfig()

	opalProfiles := make(map[string]string)
	for _, env := range opalConfig.Environments {
		for _, profileTag := range env.ProfileTags {
			opalProfiles[profileTag] = env.RoleID
		}
	}

	return &Authenticator{
		onStatus:     statusFn,
		opalConfig:   opalConfig,
		opalProfiles: opalProfiles,
	}, nil
}

func (a *Authenticator) IsAuthenticating() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isAuthenticating
}

func (a *Authenticator) Current() *Session {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentSession
}

func (a *Authenticator) sendStatus(status string) {
	if a.onStatus != nil {
		a.onStatus(status)
	}
}

func (a *Authenticator) authenticateStandard(ctx context.Context, profile, region string) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	if profile != "" && profile != "default" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	return config.LoadDefaultConfig(ctx, opts...)
}

// SwitchEnvironment authenticates using the given Environment and returns
// the resulting Session. Dispatches on env.Auth.Type (none, aws_sdk, jwt).
func (a *Authenticator) SwitchEnvironment(ctx context.Context, env environments.Environment) (*Session, error) {
	a.mu.Lock()
	if a.isAuthenticating {
		a.mu.Unlock()
		return nil, fmt.Errorf("authentication already in progress")
	}
	a.isAuthenticating = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.isAuthenticating = false
		a.mu.Unlock()
	}()

	a.sendStatus(fmt.Sprintf("Switching to %s in %s", env.Name, env.Region))

	session := &Session{
		Profile:     env.Name,
		Region:      env.Region,
		Environment: env,
	}

	switch env.Auth.Type {
	case "none":
		// No AWS credentials needed; still propagate the region so callers
		// that inspect session.Config.Region (e.g. the local profile) see a
		// consistent value.
		session.Config = aws.Config{Region: env.Region}

	case "aws_sdk":
		if env.Auth.PreAuth != nil {
			if err := a.runPreAuthCommand(ctx, env.Auth.PreAuth, env.Name); err != nil {
				return nil, fmt.Errorf("pre-auth: %w", err)
			}
		}
		cfg, err := a.authenticateStandard(ctx, env.Name, env.Region)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
		session.Config = cfg

	case "jwt":
		token, err := loadJWT(env.Auth)
		if err != nil {
			return nil, err
		}
		session.Token = token
		// Run the probe if the transport defines one — Dragos always does.
		if env.Transport.Probe != nil {
			if err := runJWTProbe(ctx, env, token); err != nil {
				return nil, err
			}
		}
		// Phase 2 backwards-compat: still populate the legacy DragosSession
		// so view.go's session.Dragos != nil checks work. Phase 4 deletes
		// this branch.
		if env.Name == DragosProfile {
			session.Dragos = &DragosSession{
				BaseURL:      env.Transport.BaseURL,
				AuthToken:    token,
				IndexPattern: env.IndexPattern,
				KbnVersion:   env.Transport.Headers["kbn-version"],
			}
			session.Config = aws.Config{Region: env.Region}
		}

	default:
		return nil, fmt.Errorf("unknown auth.type %q (want none|aws_sdk|jwt)", env.Auth.Type)
	}

	a.mu.Lock()
	a.currentSession = session
	a.mu.Unlock()

	return session, nil
}

// runPreAuthCommand executes Auth.PreAuth.Command. The translator (Task 5)
// already substituted any {role_id} / {profile} placeholders in the
// command, so this just runs it as-is.
func (a *Authenticator) runPreAuthCommand(ctx context.Context, spec *internalconfig.PreAuthSpec, profile string) error {
	if len(spec.Command) == 0 {
		return nil
	}
	cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	a.sendStatus(fmt.Sprintf("Running pre-auth: %s", strings.Join(spec.Command, " ")))
	if err := cmd.Run(); err != nil {
		output := stdout.String() + stderr.String()
		for _, marker := range spec.DetectSessionExpired {
			if strings.Contains(output, marker) {
				return fmt.Errorf("session expired: please re-authenticate the underlying tool (%s)", spec.Command[0])
			}
		}
		return fmt.Errorf("%s failed: %v\nOutput: %s", spec.Command[0], err, output)
	}
	a.sendStatus("Pre-auth completed")
	return nil
}

// loadJWT resolves the token from env > path. Returns an error when none
// is available; callers may then trigger a login modal flow (the manager
// is responsible for that — auth itself doesn't pop UIs).
//
// `~/...` paths are expanded against the user's home directory because
// the YAML schema documents tilde-paths and users naturally write them.
// os.ReadFile does no shell-style expansion on its own.
func loadJWT(spec internalconfig.AuthSpec) (string, error) {
	if spec.Env != "" {
		if v := strings.TrimSpace(os.Getenv(spec.Env)); v != "" {
			return v, nil
		}
	}
	if spec.Path != "" {
		path, err := ExpandHome(spec.Path)
		if err != nil {
			return "", err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("jwt token not available: file %s does not exist (set $%s or run the login flow)", path, spec.Env)
			}
			return "", fmt.Errorf("read jwt %s: %w", path, err)
		}
		t := strings.TrimSpace(string(raw))
		if t == "" {
			return "", fmt.Errorf("jwt token at %s is empty", path)
		}
		return t, nil
	}
	return "", fmt.Errorf("jwt token unavailable: no env or path configured")
}

// runJWTProbe builds a probe request from the Environment and runs it.
func runJWTProbe(ctx context.Context, env environments.Environment, token string) error {
	probeURL := strings.TrimRight(env.Transport.BaseURL, "/") +
		env.Transport.ProxyPath +
		"?path=" + env.Transport.Probe.Path + "&method=GET"

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, probeURL, nil)
	if err != nil {
		return fmt.Errorf("probe: build request: %w", err)
	}
	if env.Transport.TokenHeader != nil {
		header := strings.ReplaceAll(env.Transport.TokenHeader.Format, "{token}", token)
		req.Header.Set(env.Transport.TokenHeader.Name, header)
	}
	for k, v := range env.Transport.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if err := probe.Run(probeCtx, &http.Client{Timeout: 15 * time.Second}, req, env.Transport.Probe.RejectHTML); err != nil {
		return err
	}
	return nil
}

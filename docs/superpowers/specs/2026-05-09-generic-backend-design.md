# Generic, Configurable Backend — Design

**Date**: 2026-05-09
**Branch**: `dragos-platform`
**Status**: Approved through brainstorming; awaiting implementation plan.

## Problem

Cloudcutter has hardcoded vendor-specific code paths for Sophos and Dragos:
authentication branches on profile name, Elasticsearch endpoints are baked into
Go (`dev-{region}-primary-es.darkbytes.io`, `tpelletier-sitestore.dev.platform.dragos.cloud`),
time-field schemas live in `var DefaultTimeFields` and `var DragosTimeFields`,
and the Manager has five parallel `switchTo*Profile` methods. The author no
longer works at Sophos and would like the tool to follow them across employers
without a Go-level refactor every time.

## Goal

Replace the hardcoded vendor coupling with a single user-edited config file.
Adding a new employer's Elasticsearch deployment should require writing a YAML
entry, not editing Go.

## Non-goals

- Plugin architecture for arbitrary Go-level extension. The audience is "the
  author, switching companies." If a future employer needs a fundamentally new
  auth flow (OIDC redirect, SAML), that's a Go change to add a new
  `auth.type` enum value. The schema is designed to absorb the common JWT
  issuance shapes without code changes.
- Open-source / multi-team distribution. Not in scope.
- Changing the DynamoDB feature's gating. DynamoDB stays where it is — the
  view continues to be present in the manager and continues to fail
  cleanly when the active environment has no AWS credentials. Hiding it
  per-environment is out of scope.
- Browser cookie auto-pickup (`kooky`). The diagnostic tool stays; the
  in-app browser-cookie path stays disabled.

## Decisions made during brainstorming

1. **Config-driven, no Go plugins.** New environments are added by editing
   a config file.
2. **AWS profiles auto-discover from `~/.aws/credentials`** and a config
   block (`default_aws_backend`) supplies the URL/transport/time-field
   template applied to them. Per-profile shadowing via `environments[name=…]`.
3. **Compose from primitives** rather than flat type-per-environment or
   layered template inheritance. Each environment selects orthogonal
   primitives for `auth`, `transport`, `endpoint`, `index_pattern`,
   `time_fields`.
4. **Single spec covers all five axes** (auth + transport + URL + index +
   time fields), implementation phases incrementally.
5. **YAML for the new config file**; the existing `~/.cloudcutter/dragos.json`
   format goes away in favor of a plain-text JWT cache file referenced by
   `auth.path`.
6. **First-run with no config writes a starter and exits** — friendly, no
   silent embedded defaults.
7. **Phase 1 ships `cloudcutter init`** that migrates existing
   `opal.json` / `dragos.json` / env-var configuration into YAML.

## Architecture

```
~/.cloudcutter/config.yaml                       ← user-edited, single file
        │ load
        ▼
┌──────────────────────────────────────┐
│  config pkg                          │  parse + validate + ${ENV} expansion
└──────────────┬───────────────────────┘
               │ Config
               ▼
┌──────────────────────────────────────┐
│  environments pkg                    │  Resolve(name) → EnvironmentSpec
│                                      │  Spec.Materialize(region) → Environment
└──────────────┬───────────────────────┘
               │ Environment
               ▼
┌──────────────────────────────────────┐
│  auth.Authenticator (refactored)     │  branches on Environment.Auth.Type:
│   none | aws_sdk | jwt               │  not on profile name
└──────────────┬───────────────────────┘
               │ Session{Environment, AWSConfig, Token}
               ▼
┌──────────────────────────────────────┐
│  elastic.Service (refactored)        │  picks transport from
│   plain | sigv4 | kibana_proxy       │  Environment.Transport.Type
└──────────────────────────────────────┘
```

A single shared `internal/probe` package (or equivalent location) holds
the probe implementation used by both `auth` (validate-session-on-switch)
and `elastic.Service` (validate-transport-on-construct). Today these are
duplicated in `auth.ProbeDragos` and `elastic.probeDragosProxy`; the new
design has one.

### What collapses

| Today | After |
|---|---|
| `Authenticator.SwitchProfile` branches on profile name (dragos / opal / local / standard) | Branches on `Environment.Auth.Type` (none / aws_sdk / jwt) |
| `Session.Dragos *DragosSession` (nil-check elsewhere) | `Session.Environment` — views ask the env for time fields and transport kind |
| `OpalConfig` (`~/.cloudcutter/opal.json`), `DragosConfig` (`~/.cloudcutter/dragos.json`) | Single `~/.cloudcutter/config.yaml`. `~/.cloudcutter/dragos.json` is replaced by a plain-text JWT cache file (mode 0600) referenced by `auth.path`. `opal.json` goes away. |
| `Manager.switchToDragosProfile` / `switchToDevProfile` / `switchToProdProfile` / `switchToLocalProfile` / `switchToStandardProfile` | One `Manager.switchToEnvironment(name)`. Login-modal trigger becomes "auth.type = jwt with `login:` block defined, on auth failure." |
| `services.InitializeElastic` + `InitializeElasticDragos` | One `services.InitializeElastic(env)` |
| `view.Reinitialize` checks `session.Dragos != nil` | Reads `session.Environment.Transport.Type` |
| `auth.ProbeDragos` + `elastic.probeDragosProxy` (two probes for the same job) | One probe in `internal/probe`, called by both auth and elastic |

## Config schema

Full annotated example. Field-level reference follows.

```yaml
# ~/.cloudcutter/config.yaml

# Optional. Applied as a template to every ~/.aws profile auto-discovered
# from ~/.aws/credentials, unless an entry under `environments` shadows it
# by name.
default_aws_backend:
  vars:                                  # used for substitutions in this block
    role_id:                             # resolved against profile name; first match wins
      - { match: "^opal_dev$",  env: OPAL_DEV_ROLE_ID }
      - { match: "^opal_prod$", env: OPAL_PROD_ROLE_ID }
    prefix:
      - { match: "^opal_dev$",  value: dev }
      - { match: "^opal_prod$", value: prod }
  auth:
    type: aws_sdk
    pre_auth:                            # optional. Runs before SDK auth.
      command: ["opal", "iam-roles:start", "--id", "{role_id}", "--profileName", "{profile}"]
      detect_session_expired:            # substrings in stdout/stderr that mean
        - "Enter your email"             # the user must re-run opal login
        - "session is invalid or expired"
  transport:
    type: sigv4
    service: es
    url_template: "https://{prefix}-{region}-primary-es.darkbytes.io"
  index_pattern: "main-summary-*"
  time_fields:
    - { name: unixTime, format: unix }
    - { name: detectionGeneratedTime, format: unix_ms }

environments:
  - name: dragos
    auth:
      type: jwt
      path: ~/.cloudcutter/dragos.token  # JWT cache file (plain text, mode 0600)
      env: DRAGOS_AUTH_TOKEN             # optional; tried before file
      login:                             # optional. Enables in-app login modal.
        url: https://platform.dragos.cloud/auth/api/v1/login/password
        body_format: json                # json | form (default: json)
        body_fields:                     # rendered as the login form's fields
          - { name: username, kind: text }
          - { name: password, kind: password }
        query:
          providerId: "00000000-0000-0000-0000-000000000002"
        token_extract:
          from: cookie                   # cookie | header | json_path
          name: dragos-auth-token
    transport:
      type: kibana_proxy
      base_url: https://platform.dragos.cloud
      proxy_path: /kibana/api/console/proxy
      token_header:                      # how the token is attached to each request
        name: Cookie
        format: "dragos-auth-token={token}"
      headers:                           # static request headers (every request)
        kbn-xsrf: cloudcutter
        kbn-version: "8.19.2"
      probe:                             # used by auth + elastic.Service to validate
        path: _cluster/health
        reject_html: true                # gateway-returns-SPA-on-401 detection
    index_pattern: "events*"
    time_fields:
      - { name: createdAt, format: date }

  - name: local
    auth: { type: none }
    transport:
      type: plain
      base_url: http://localhost:9200
    time_fields: []

# Variable substitution everywhere: ${VAR} and ${VAR:-default}.
# Path tildes are expanded.
```

### Primitive enums

| `auth.type` | sub-fields |
|---|---|
| `none` | — |
| `aws_sdk` | `pre_auth` (optional) |
| `jwt` | `path` and/or `env`, `login` (optional). Token-source resolution: `env` (if set and value non-empty) → `path` (if set and file exists with non-empty contents) → `login` modal (if set). The transport's `token_header` describes how the token is attached on each request. |

| `transport.type` | sub-fields |
|---|---|
| `plain` | `base_url` |
| `sigv4` | `service`, `url_template` (with `{region}`, `{profile}`, plus any user-defined `vars` keys) |
| `kibana_proxy` | `base_url`, `proxy_path`, `token_header`, `headers`, `probe` |

| `time_fields[].format` | meaning |
|---|---|
| `unix` | seconds since epoch (int64) |
| `unix_ms` | milliseconds since epoch (int64) |
| `date` | ISO 8601 (`strict_date_optional_time`) |

| `auth.login.body_format` | meaning |
|---|---|
| `json` (default) | Body is JSON object: `{name1: value1, name2: value2, ...}`. `Content-Type: application/json`. |
| `form` | Body is `application/x-www-form-urlencoded`. |

| `auth.login.token_extract.from` | meaning |
|---|---|
| `cookie` | Read named cookie from `Set-Cookie` headers. |
| `header` | Read named header from response. |
| `json_path` | Read from JSON response body. Path syntax deferred to phase 2 (likely [gjson](https://github.com/tidwall/gjson)). Not yet implemented; reject at config-load time with "json_path token extraction not yet implemented." |

### Substitution

All substitutions follow the same machinery — a `vars:` map at any block
that supports templated strings. Each `vars` entry resolves a single key
against the profile name via a list of `{ match: regex, value: string }`
or `{ match: regex, env: ENV_VAR }` rules. **First matching regex wins.**

- **URL template variables**: `{region}` (substituted at materialize time),
  `{profile}` (substituted at materialize time), plus any keys defined in
  the surrounding `vars:` block.
- **Pre-auth command substitutions**: `{profile}`, plus any keys from the
  surrounding `vars:` block (e.g. `{role_id}` in the example above).
- **Token header substitution**: `{token}`.
- **Environment variable expansion**: `${VAR}` and `${VAR:-default}` work in
  any string field.

### Resolution rules

1. Picker entries = config `environments[].name` ∪ `~/.aws/credentials`
   profile names.
2. If an `environments[name=X]` entry exists, it shadows any same-named AWS
   profile.
3. For an auto-discovered AWS profile not in `environments`, resolve by
   applying `default_aws_backend` with `{profile}` = that profile name.
4. Within `vars:` rules, **first matching regex wins**. If no rule matches
   and the key is referenced in a template, that's a config error surfaced
   in the status bar with the unresolved key named.
5. If `default_aws_backend` is missing and an auto-discovered AWS profile is
   selected, surface a user-friendly error in the status bar:
   `Profile 'X' has no environment definition. Add it under 'environments' or define a default_aws_backend.`

### Two-step resolution

Region is not part of `EnvironmentSpec` because the user can change region
in-app. Resolution is two steps:

1. `Resolver.Resolve(name) → EnvironmentSpec` — looks up by name, applies
   precedence rules. Region-agnostic.
2. `EnvironmentSpec.Materialize(region) → Environment` — substitutes
   `{region}` and any `vars` keys, validates that all template references
   are resolvable for this profile-name + region pair.

When the user changes region, the manager re-materializes the current spec
with the new region and re-runs the existing reinit path.

## Go types & key interfaces

```go
// internal/config — pure parser, no I/O beyond ReadFile + env expansion
type Config struct {
    DefaultAWSBackend *EnvironmentTemplate
    Environments      []EnvironmentSpec
}

// internal/environments — resolution
type EnvironmentSpec struct {
    Name         string
    Auth         AuthSpec
    Transport    TransportSpec
    IndexPattern string
    TimeFields   []TimeField           // promoted from elastic pkg
    Vars         map[string][]VarRule
}
type Environment struct {                // result of Spec.Materialize(region)
    Name         string
    Region       string
    Auth         AuthSpec
    Transport    TransportSpec           // URL templates already substituted
    IndexPattern string
    TimeFields   []TimeField
}
type Resolver interface {
    List() []string                      // picker source: config envs ∪ ~/.aws profiles
    Resolve(name string) (EnvironmentSpec, error)
}

// internal/auth — branches on AuthSpec.Type, not on profile name
type Authenticator interface {
    SwitchProfile(ctx, name string) (*Session, error)  // unchanged signature
}
type Session struct {
    Environment Environment              // replaces session.Dragos *DragosSession
    AWSConfig   aws.Config               // populated when Auth.Type == aws_sdk
    Token       string                   // populated when Auth.Type == jwt
}

// internal/probe — shared probe used by both auth and elastic
func Run(ctx context.Context, transport http.RoundTripper, spec ProbeSpec) error

// internal/services/elastic — one constructor, dispatches transport from spec
func NewService(env Environment, awsCfg aws.Config, token string) (*Service, error)
func (s *Service) Reinitialize(env Environment, awsCfg aws.Config, token string) error
```

### Deletions

- `auth.DragosSession`, `auth.DragosConfig` (replaced by `Environment` /
  the new YAML config)
- `auth.OpalConfig` (folded into `config.yaml` `default_aws_backend.vars` +
  `pre_auth`)
- `auth.ProbeDragos` and `elastic.probeDragosProxy` (replaced by
  `internal/probe.Run`)
- `Manager.switchTo{Dragos,Dev,Prod,Local,Standard}Profile` (replaced by one
  `switchToEnvironment(name)`)
- `Service.ReinitializeDragos` (folded into `Reinitialize`)
- `services.InitializeElasticDragos` (folded into `InitializeElastic`)
- The dead `loadDragosCookieFromBrowser` and the `kooky` import in
  `internal/auth/dragos_config.go`. The `cmd/dragos-cookie-probe` diagnostic
  binary keeps `kooky` if the author still wants the probe.

### Preserved

- `Authenticator.SwitchProfile(ctx, name) (*Session, error)` signature
- `View.Reinitialize(cfg aws.Config) error` signature (Manager passes
  `cfg.Region` as today; views read transport details via the session)
- `profile.Handler` and the picker UX (the picker just sources its list
  through the new `environments.Resolver` instead of reading
  `~/.aws/credentials` directly)

## First-run experience

If `~/.cloudcutter/config.yaml` does not exist when cloudcutter starts:

1. Cloudcutter writes a starter config to `~/.cloudcutter/config.yaml`,
   mode 0600. The starter contains:
   - A commented-out `default_aws_backend` block (so users at AWS shops
     have a working template to uncomment + edit).
   - The `local` environment fully populated and uncommented.
   - A commented-out `dragos`-style example as a model for adding JWT
     environments.
2. Prints to stderr:
   `wrote starter config to ~/.cloudcutter/config.yaml — edit it and run cloudcutter again`
3. Exits 0.

Subsequent runs require the file. If a user later deletes their config
file, the same starter-write flow runs.

## `cloudcutter init`

A subcommand that synthesizes a `~/.cloudcutter/config.yaml` from the user's
existing legacy configuration. Lands in phase 1 alongside the YAML loader.

Inputs read:
- `~/.cloudcutter/opal.json` (Opal role IDs + profile-tag mappings)
- `~/.cloudcutter/dragos.json` (Dragos baseURL, providerID, token)
- Env vars: `OPAL_DEV_ROLE_ID`, `OPAL_PROD_ROLE_ID`, `DRAGOS_BASE_URL`,
  `DRAGOS_INDEX_PATTERN`, `DRAGOS_KBN_VERSION`, `DRAGOS_AUTH_TOKEN`

Output: `~/.cloudcutter/config.yaml` containing a `default_aws_backend`
populated with the Sophos darkbytes template + Opal pre-auth, and (if
dragos.json is present) an `environments[name=dragos]` entry. The dragos
JWT itself is migrated from the JSON's `authToken` field into a plain-text
`~/.cloudcutter/dragos.token` file (mode 0600).

If `config.yaml` already exists, `cloudcutter init` refuses by default.
`--force` overwrites.

## Phasing

Each phase ends with a working binary so partial work is shippable.

| Phase | Lands | Risk |
|---|---|---|
| **1. Config + init + Environment types** | `internal/config` parses YAML; `internal/environments` provides Resolver + Materialize; `Environment` struct exists; first-run starter-write; `cloudcutter init` migrates existing setup. **No callers yet** — build is a no-op for the running app. | Low — pure plumbing. |
| **2. Auth refactor** | `Authenticator.SwitchProfile` branches on `Environment.Auth.Type`. `internal/probe.Run` lands and replaces `auth.ProbeDragos`. The Manager wires the new resolver into the existing 5 switch-funcs (each builds an `Environment` from the YAML config). Behavior unchanged from user's POV. | Medium. |
| **3. Transport refactor** | `elastic.Service` constructed from `Environment`. `dragosTransport` becomes a generic `kibanaProxyTransport` parameterized by config. `awsTransport` becomes `sigv4Transport` parameterized. `elastic.probeDragosProxy` deleted. | Medium. |
| **4. View + Manager cleanup** | `session.Dragos != nil` → `session.Environment.Transport.Type == "kibana_proxy"`. The 5 switch-funcs collapse to one `switchToEnvironment(name)`. | Low after phases 1–3. |
| **5. Delete legacy** | Remove `DragosSession`, `OpalConfig`, `DragosConfig`, the `dragos.json` reader, `kooky` import in `internal/auth`. | Low. |

## Error handling & validation

- **Config load failures** are fatal at startup with a message that points
  at the bad line, e.g. `~/.cloudcutter/config.yaml:42: environment 'dragos' missing required field 'transport.base_url'`.
  Use `gopkg.in/yaml.v3` (surfaces line numbers).
- **Missing `default_aws_backend` when an AWS profile is selected**: error
  in the picker callback (status bar), wording above.
- **Unresolved template var at materialize time**: error in the status bar
  naming the key and the env-name + region attempted, e.g.
  `environment 'opal_stg' references {prefix} but no vars.prefix rule matches profile 'opal_stg'`.
- **Bad regex anywhere in `vars`**: caught at config-load time, never at
  switch-time.
- **Auth failures**: same pattern as today — `internal/probe.Run` runs
  pre-emptively in the auth path; login modal opens for `auth.type=jwt`
  envs with a `login:` block defined.
- **JWT auth fails with no `login:` block defined**: clear status bar error,
  no modal pop.
- **First run with no config**: starter-write flow above; not an error.

## Testing

- **`config` package**: pure parser; table-driven tests over example YAMLs
  (golden-file style for the parsed Go structs).
- **`environments` package**: table-driven Resolver + Materialize tests
  covering precedence rules, region substitution, and missing-var errors.
- **`probe` package**: HTTP probe tests via `httptest.NewServer` covering
  the HTML-on-401 detection.
- **`auth` package**: the new dispatch (`Auth.Type` → primitive) is testable
  with a fake credential provider for `aws_sdk` and a mocked HTTP server
  for `jwt`. Existing `internal/auth` tests stay; the
  `Authenticator.SwitchProfile` table is just expanded.
- **`elastic.Service`**: existing transport tests via `httptest.NewServer`
  (Sophos + Dragos paths). The Dragos test fixture generalizes to the new
  `kibana_proxy` config; assertions stay the same.
- **`init` subcommand**: golden-file tests over (input legacy config files,
  expected output YAML).
- **`view.Reinitialize` and Manager profile-switching**: minimal direct
  testing today; not adding new test infrastructure as part of this change.

## Pliability tradeoffs accepted

- **Login form generality**: schema models the "POST credentials, extract
  token from response cookie / header / JSON" shape generically. OIDC
  redirect or SAML is a future Go change (new `auth.type` enum value).
- **`json_path` extraction syntax**: deferred. Schema accepts the value
  but the loader rejects it as not-yet-implemented until a real env needs
  it. Choose a syntax (gjson is most likely) at that point.
- **Time-field formats**: three enum values today (`unix`, `unix_ms`,
  `date`). Adding `unix_ns` or arbitrary date layouts is a small Go change.
- **No "default JWT backend" template**: a JWT environment with no shared
  structure across deployments lives entirely under `environments`. If a
  pattern emerges (e.g. multiple Kibana-proxy envs sharing structure),
  add a second template block then. YAGNI for now.

## Open items deferred to implementation plan

- Exact YAML library choice (`yaml.v3` vs `koanf`). Both surface line
  numbers; `koanf` adds layered config and env-var integration which may
  be useful for `${VAR}` expansion.
- Exact placement of `internal/probe` — is it its own package or a
  subpackage of `internal/services/elastic`? Lean toward standalone since
  both auth and elastic call it.
- `cloudcutter init`'s exact starter-config text (commented YAML) — write
  during phase 1 implementation, get it right inline.

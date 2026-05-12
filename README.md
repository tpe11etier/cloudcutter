# cloudcutter

Terminal UI for browsing AWS DynamoDB and Elasticsearch across multiple environments and auth methods.

## How it works

1. **Launch** — a profile picker lists every environment from your config, plus any profiles discovered in `~/.aws/credentials`.
2. **Pick an environment** — cloudcutter authenticates using that environment's configured auth method.
3. **Use a view** — DynamoDB or Elasticsearch. Switch views and environments at any time without restarting.

## Setup

### Migrating from an older cloudcutter install

If you have an existing `~/.cloudcutter/dragos.json` or `opal.json`, run:

```bash
cloudcutter init
```

This reads your legacy config files and writes `~/.cloudcutter/config.yaml` automatically. After that, launch normally — if a JWT environment has no saved token, the login form appears on its own.

Pass `--force` to overwrite an existing `config.yaml`.

### Fresh install

Build and run:

```bash
make mac && ./build/cloudcutter
# or
go run ./cmd/cloudcutter
```

On first run cloudcutter writes a starter `~/.cloudcutter/config.yaml`. Edit it to describe your environments, then relaunch. The bare minimum — a local Elasticsearch:

```yaml
environments:
  - name: local
    auth: { type: none }
    transport:
      type: plain
      base_url: http://localhost:9200
    time_fields: []
```

Run `make es-up` to spin up a local ES instance via Docker.

## Navigating the app

Press `:` at any time to open the command prompt (autocomplete available):

| Command      | Action                       |
| ------------ | ---------------------------- |
| `profile`  | Open the environment picker  |
| `region`   | Open the region picker       |
| `dynamodb` | Switch to DynamoDB view      |
| `elastic`  | Switch to Elasticsearch view |
| `exit`     | Quit                         |

Other global keys:

| Key           | Action                       |
| ------------- | ---------------------------- |
| `?`         | Toggle help                  |
| `Esc`       | Dismiss modal / cancel input |
| `Tab`       | Cycle focus forward          |
| `Shift+Tab` | Cycle focus backward         |

### Elasticsearch view

| Key        | Action                  |
| ---------- | ----------------------- |
| `Ctrl+A` | Focus field list        |
| `Ctrl+S` | Focus selected fields   |
| `Ctrl+R` | Focus results table     |
| `Enter`  | Execute query / confirm |

## Configuration reference

Config lives at `~/.cloudcutter/config.yaml`. Two top-level keys:

- `environments` — explicit named environments shown in the picker
- `default_aws_backend` — applied to every auto-discovered AWS profile that doesn't have an explicit entry

### Auth types

**`none`** — no authentication. For local or open endpoints.

```yaml
auth:
  type: none
```

**`aws_sdk`** — standard AWS credential loading from `~/.aws/credentials` / `~/.aws/config`. Optionally runs a shell command first to refresh credentials (e.g. a CLI auth tool):

```yaml
auth:
  type: aws_sdk
  pre_auth:
    command: ["my-auth-tool", "login", "--profile", "{profile}"]
    detect_session_expired: ["session expired"]  # strings that indicate the session needs refresh
```

**`jwt`** — token-based auth. On each profile switch, cloudcutter looks for the token in this order:

1. Environment variable (`env`)
2. File on disk (`path`)
3. In-app login form (`login`) — appears automatically if the token is missing or rejected

Once you log in, the token is saved to `path` for future runs. You won't see the login form again until the token expires.

```yaml
auth:
  type: jwt
  env: MY_AUTH_TOKEN               # try this env var first
  path: ~/.cloudcutter/my.token    # fall back to this file; login saves here
  login:
    url: https://my.platform.example.com/auth/login
    body_format: json              # "json" (default) | "form"
    body_fields:
      - { name: username, kind: text }
      - { name: password, kind: password }
    query:                         # optional query params appended to the login URL
      providerId: "some-id"
    token_extract:
      from: cookie                 # "cookie" | "header"
      name: my-auth-token
```

### Transport types

**`plain`** — bare HTTP, no signing.

```yaml
transport:
  type: plain
  base_url: http://localhost:9200
```

**`sigv4`** — AWS SigV4 signed requests. `{region}` and `{profile}` are substituted at switch time.

```yaml
transport:
  type: sigv4
  service: es
  url_template: "https://my-es.{region}.example.com"
```

**`kibana_proxy`** — routes ES API calls through a Kibana console proxy endpoint. Used with `jwt` auth. Each request is re-signed with the token via `token_header`.

`probe` is a connectivity check run at login time. Set `reject_html: true` if your gateway returns an HTML login page (with HTTP 200) on expired auth — cloudcutter treats that as an auth failure and re-triggers the login form rather than trying to parse the HTML as an ES response.

```yaml
transport:
  type: kibana_proxy
  base_url: https://my.platform.example.com
  proxy_path: /kibana/api/console/proxy
  token_header:
    name: Cookie
    format: "my-auth-token={token}"
  headers:
    kbn-xsrf: cloudcutter
  probe:
    path: _cluster/health
    reject_html: true
```

### Variable substitution

`{region}` and `{profile}` are always available. You can define your own vars with regex matching against the profile name — first match wins:

```yaml
vars:
  prefix:
    - { match: "^prod-.*", value: prod }
    - { match: "^.*",      value: dev  }   # catch-all
transport:
  type: sigv4
  service: es
  url_template: "https://{prefix}-es.{region}.example.com"
```

Var values can also come from environment variables:

```yaml
vars:
  cluster_id:
    - { match: "^.*", env: MY_CLUSTER_ID }
```

### Environment variable expansion

Any string value in `config.yaml` can embed environment variables using `${VAR}` or `${VAR:-default}`. Expansion happens before YAML parsing, so it works in every field.

- `${VAR}` — expands to the value of `VAR`; empty string if unset
- `${VAR:-default}` — expands to `default` when `VAR` is unset **or** empty
- `$$` — a literal dollar sign

```yaml
transport:
  type: plain
  base_url: ${ES_BASE_URL:-http://localhost:9200}
auth:
  type: jwt
  env: ${PLATFORM_TOKEN_ENV:-MY_AUTH_TOKEN}
```

### DynamoDB auth (`aws_profile`)

For environments that use `jwt` auth (Elasticsearch via a token), DynamoDB still needs AWS credentials. Set `aws_profile` to an AWS profile name and cloudcutter loads it separately for DynamoDB.

```yaml
environments:
  - name: my-platform
    auth:
      type: jwt
      path: ~/.cloudcutter/my-platform.token
      ...
    transport:
      ...
    aws_profile: my-aws-profile    # loaded from ~/.aws/credentials for DynamoDB
    time_fields:
      - { name: "@timestamp", format: date }
```

If `aws_profile` is omitted on a `jwt` or `none` environment, DynamoDB is unavailable. On `aws_sdk` environments the primary session is used automatically.

### Time fields

Tells cloudcutter how to encode timestamps in queries. Required for timeframe filtering in the Elasticsearch view.

```yaml
time_fields:
  - { name: "@timestamp", format: date }    # ISO 8601 strict_date_optional_time
  - { name: unixTime,     format: unix }    # integer seconds since epoch
  - { name: eventTime,    format: unix_ms } # integer milliseconds since epoch
```

### Default AWS backend

Applies a common config to all auto-discovered AWS profiles. Explicit `environments` entries shadow same-named profiles.

```yaml
default_aws_backend:
  auth:
    type: aws_sdk
  transport:
    type: sigv4
    service: es
    url_template: "https://my-es.{region}.example.com"
  index_pattern: "logs-*"
  time_fields:
    - { name: "@timestamp", format: date }
```

### Full example

```yaml
environments:
  - name: local
    auth: { type: none }
    transport:
      type: plain
      base_url: http://localhost:9200
    time_fields: []

  - name: my-platform
    auth:
      type: jwt
      path: ~/.cloudcutter/my-platform.token
      login:
        url: https://my.platform.example.com/auth/login
        body_fields:
          - { name: username, kind: text }
          - { name: password, kind: password }
        token_extract:
          from: cookie
          name: my-auth-token
    transport:
      type: kibana_proxy
      base_url: https://my.platform.example.com
      proxy_path: /kibana/api/console/proxy
      token_header:
        name: Cookie
        format: "my-auth-token={token}"
      headers:
        kbn-xsrf: cloudcutter
      probe:
        path: _cluster/health
        reject_html: true
    aws_profile: my-aws-profile    # optional: AWS credentials for DynamoDB
    index_pattern: "logs-*"
    time_fields:
      - { name: "@timestamp", format: date }

default_aws_backend:
  auth:
    type: aws_sdk
  transport:
    type: sigv4
    service: es
    url_template: "https://my-es.{region}.example.com"
  index_pattern: "logs-*"
  time_fields:
    - { name: "@timestamp", format: date }
```

## Troubleshooting

**Login form not appearing** — the form only shows if the environment has a `login:` block in its auth config. If you pick a JWT environment and get an auth error instead of a form, add the `login:` spec to `config.yaml`.

**JWT expired** — cloudcutter detects rejection and pops the login form automatically. Re-enter credentials and it retries.

**`pre_auth` command fails** — the error appears in the status bar. Typically means the CLI tool needs re-login (`my-auth-tool login`).

**HTML returned instead of JSON** — your gateway is serving a login page on expired auth. Add `probe.reject_html: true` to the transport config so cloudcutter treats it as an auth failure and re-triggers the login form.

**AWS profile not listed** — the profile must exist in `~/.aws/credentials` or `~/.aws/config`. Run `aws configure list-profiles` to check.

**Only one auth in flight at a time** — if you switch profiles while authentication is in progress, the second switch is dropped. Wait for the status bar to clear before switching again.

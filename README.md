# APImetrics CLI

A command-line interface for managing API monitors, schedules, SLOs, and more on the [APImetrics](https://apimetrics.io) platform, by [APIContext Inc](https://apicontext.com).

## Documentation

For more information, please visit our main documentation site, available at [https://docs.apicontext.com/cli](https://docs.apicontext.com/cli).


## Installation

## Manual installation

The binaries for the APImetrics CLI can be downloaded from the [releases page](https://github.com/APImetrics/APImetrics-cli/releases). Choose the appropriate version for your operating system and put the executable in your path for easy access from the command line.

## MacOS

You can install the APImetrics CLI using Homebrew:

```bash
brew install apimetrics/tap/apimetrics
```

## Windows

You can install the APImetrics CLI using the Windows Package Manager (winget):

```powershell
winget install APIContext.APImetricsCLI
```

## Linux

You can install the APImetrics CLI by downloading the binary from the [releases page](https://github.com/APImetrics/APImetrics-cli/releases).

```bash
curl -LO https://github.com/APImetrics/APImetrics-cli/releases/latest/download/apimetrics-[version]-linux-amd64.tar.gz
tar -xzf apimetrics-[version]-linux-amd64.tar.gz
sudo mv apimetrics /usr/local/bin/apimetrics
```
Note: [version] should be replaced with the specific version you want to download, e.g., `0.0.1`.

## Getting Started

### 1. Log in

```bash
apimetrics login
```

This opens your browser for OAuth2 authentication. Your credentials are cached locally and refreshed automatically.

### 2. Select a project

All commands require an active project. Run the following to choose one:

```bash
apimetrics project select
```

To confirm which project is currently active:

```bash
apimetrics project show
```

### 3. Run your first command

```bash
# List all API calls in the current project
apimetrics list-calls

# List all schedules
apimetrics list-schedules
```

To see all supported commands, run
```bash
apimetrics --help
```

## Service Accounts

A **service account** authenticates without a browser, which is what makes the
CLI usable from CI, cron jobs, and other headless environments. Create one in
the APImetrics web app and download the JSON file it gives you:

```json
{
  "name": "ci-runner",
  "client_id": "...",
  "client_secret": "...",
  "audience": "https://client.apimetrics.io"
}
```

Point the CLI at that file with `--service-account`, and it authenticates every
request with those credentials instead of the cached browser login:

```bash
apimetrics list-calls --service-account ./ci-runner.apimetrics.json
```

The file holds a live secret — keep it out of version control, and prefer the
environment variable when you can:

```bash
export APIMETRICS_SERVICE_ACCOUNT=/secrets/ci-runner.apimetrics.json
apimetrics list-calls
```

### Choosing a project

Service accounts are granted access to specific projects, and headless runs
can't answer an interactive project prompt. Name the project for the run with
`--project-id`, or its environment variable:

```bash
apimetrics list-calls \
  --service-account /secrets/ci-runner.apimetrics.json \
  --project-id ag9zfmFwaW1ldHJpY3MtcWNy...

export APIMETRICS_PROJECT_ID=ag9zfmFwaW1ldHJpY3MtcWNy...
```

`--project-id` applies to that run only and never overwrites the project saved
by `project select`. To see which projects a service account can reach, run
`apimetrics project select --service-account ...` from a terminal.

### Notes

- Access tokens last an hour and are cached, keyed by the credentials
  themselves — a service account never picks up your interactive login's token,
  or another service account's. Rotating the secret invalidates the cache.
- The client credentials grant issues no refresh token (RFC 6749 §4.4.3): when
  the access token expires the CLI simply requests a new one.
- `apimetrics login --service-account ...` fetches a token without making an
  API call, which is a quick way to check that credentials work.
- `apimetrics logout --service-account ...` drops that account's cached token
  and leaves your browser login and selected project alone.

## Passing Input

All create and update commands read a JSON body from **stdin** using a heredoc. There is no `--body`, `--data`, or `-d` flag.

```bash
apimetrics create-call <<'EOF'
{
  "meta": {
    "name": "My API Check"
  },
  "request": {
    "method": "GET",
    "url": "https://api.example.com/health"
  }
}
EOF
```

You can also use **CLI Shorthand** as a concise alternative to JSON:

```bash
apimetrics create-call meta.name: "My API Check", request.method: GET, request.url: https://api.example.com/health
```

Or pipe from a file:

```bash
apimetrics create-call < my-call.json
```

## Output Formats

Use `-o` / `--rsh-output-format` to control output:

| Format | Description |
|---|---|
| `auto` (default) | Pretty in terminal, JSON when piped |
| `json` | Standard JSON |
| `yaml` | YAML |
| `table` | Tabular layout (best for lists) |
| `readable` | Colorized human-readable format |
| `gron` | Grep-friendly flattened format |

```bash
# Table output
apimetrics list-calls -o table

# Raw JSON for scripting
apimetrics list-calls -o json

# Filter results with a query expression
apimetrics list-calls -f body[0].meta.name
```

When output is redirected to a pipe or file, color is disabled and only the body is printed as JSON automatically.

## Global Flags

These flags work with every command:

| Flag | Short | Description |
|---|---|---|
| `--rsh-output-format` | `-o` | Output format (auto/json/yaml/table/readable/gron) |
| `--rsh-filter` | `-f` | Filter/project response using a query expression |
| `--rsh-raw` | `-r` | Raw output (strips quotes from strings) |
| `--rsh-verbose` | `-v` | Verbose logging |
| `--rsh-header` | `-H` | Add a request header (repeatable) |
| `--rsh-query` | `-q` | Add a query parameter (repeatable) |
| `--rsh-profile` | `-p` | Use a named auth profile |
| `--rsh-server` | `-s` | Override the API server base URL |
| `--rsh-no-cache` | | Disable HTTP caching |
| `--rsh-no-paginate` | | Disable automatic pagination |
| `--rsh-retry` | | Retry count (default 2) |
| `--rsh-timeout` | `-t` | HTTP request timeout |
| `--rsh-insecure` | | Disable TLS verification |
| `--service-account` | | Authenticate with a service account file instead of a browser login |
| `--project-id` | | Use this project for the run, overriding the selected project |

## Shell Completion

Enable tab completion for your shell:

```bash
# Bash
apimetrics completion bash >> ~/.bash_profile

# Zsh
apimetrics completion zsh >> ~/.zshrc

# Fish
apimetrics completion fish > ~/.config/fish/completions/apimetrics.fish

# PowerShell
apimetrics completion powershell >> $PROFILE
```

Once enabled, press `tab` to explore available commands and their arguments.

## AI Agent Integration

The CLI includes built-in skills for AI coding agents (e.g. Claude Code). These skills teach agents how to create and configure monitors using the CLI.

**Install skills into Claude Code:**

```bash
apimetrics skills install --claude-code
```

**Print all skills to stdout** (for any agent or model that can read context):

```bash
apimetrics onboard
```

Available skills:
- `setup-api-monitor` — Create an API (HTTP) monitor, attach a schedule, and verify it
- `setup-browser-monitor` — Create a browser monitor, attach a schedule, and verify it
- `setup-mcp-monitor` — Create an MCP protocol monitor with session steps

## Configuration

Configuration and cached tokens are stored in platform-specific locations:

| OS | Config | Cache |
|---|---|---|
| macOS | `~/Library/Application Support/apimetrics/` | `~/Library/Caches/apimetrics/` |
| Linux | `~/.config/apimetrics/` | `~/.cache/apimetrics/` |
| Windows | `%AppData%\apimetrics\` | `%LocalAppData%\apimetrics\` |

Override these locations with environment variables:

```bash
export APIMETRICS_CONFIG_DIR=/path/to/config
export APIMETRICS_CACHE_DIR=/path/to/cache
```

`--service-account` and `--project-id` have environment variables too, so a CI
job can be configured once rather than repeating flags on every call:

```bash
export APIMETRICS_SERVICE_ACCOUNT=/secrets/ci-runner.apimetrics.json
export APIMETRICS_PROJECT_ID=ag9zfmFwaW1ldHJpY3MtcWNy...
```

### Non-production builds

Builds for the non-production APImetrics environments install alongside the
production CLI: each has its own command name and its own config and cache
directories, so they can be logged in to different environments at the same
time.

| Environment | Command | API host | Config / cache directory |
|---|---|---|---|
| production | `apimetrics` | `client.apimetrics.io` | `apimetrics` |
| beta | `apimetrics-beta` | `beta-client.apimetrics.io` | `apimetrics-beta` |
| qc | `apimetrics-qc` | `qc-client.apimetrics.io` | `apimetrics-qc` |
| qc-stable | `apimetrics-qc-stable` | `qc-stable.apimetrics.io` | `apimetrics-qc-stable` |
| dev | `apimetrics-dev` | `localhost:8080` | `apimetrics-dev` |

These environment variables all follow the command name, with hyphens replaced
by underscores — e.g. `APIMETRICS_QC_STABLE_CONFIG_DIR` and
`APIMETRICS_QC_SERVICE_ACCOUNT`. A service account is issued by one APImetrics
environment, so use it with the matching build.

Tagged environment builds are published on the
[releases page](https://github.com/APImetrics/APImetrics-cli/releases) as
prereleases, and are signed and notarized just like the production build, so
they run on macOS without a Gatekeeper warning. Homebrew and WinGet always
install the production CLI.

`--version` reports which environment a binary was built against:

```console
$ apimetrics-qc-stable --version
apimetrics-qc-stable version 0.1.0
Environment:      qc-stable (https://qc-stable.apimetrics.io)
Config directory: /Users/you/Library/Application Support/apimetrics-qc-stable
...
```

Note that the beta build authenticates against **production** Auth0, so logging
in to it uses your real production credentials.

## Logout

```bash
apimetrics logout
```

This removes cached tokens. Run `apimetrics login` again to re-authenticate.
Add `--service-account <file>` to remove just that service account's token.

# Issues and Contributing

For support questions, visit [APImetrics Support](https://apicontext.com/support) and open a request.

For reporting bugs or requesting features, please open an issue on the [GitHub repository](https://github.com/APImetrics/APImetrics-cli/issues).

We welcome contributions! Feel free to fork the repository, make changes, and submit pull requests.

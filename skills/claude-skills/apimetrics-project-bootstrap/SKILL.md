---
name: apimetrics-project-bootstrap
description: Select an APImetrics project, configure API, browser, or MCP monitoring, schedule it, run a verification, and produce a handoff summary. Use when a user asks to set up monitoring or onboard a service in APImetrics.
argument-hint: "[project name] [service or URL]"
disable-model-invocation: true
---

# APImetrics project bootstrap

Build a usable monitoring project from an empty or selected APImetrics project. The goal is not merely to create objects; finish with scheduled monitors that have successful verification evidence or a precise blocker.


## Operating rules

1. Use only the `apimetrics` CLI and Claude's normal shell/file tools. Do not require `jq`, Python, custom scripts, direct HTTP clients, or undocumented platform APIs.
2. Begin with:
   ```bash
   apimetrics --version
   apimetrics project show
   apimetrics --help
   ```
   Run `apimetrics login` or `apimetrics project select` only when needed.
3. The CLI command tree is generated from the platform's current OpenAPI description. Inspect `apimetrics <command> --help` before constructing a body or assuming an option name.
4. Commands are generally flat (`list-calls`, `create-call`), not noun/verb groups.
5. Create and update operations read JSON from stdin. Use a quoted heredoc:
   ```bash
   apimetrics <create-or-update-command> ... <<'EOF'
   {
     "example": true
   }
   EOF
   ```
   Do not invent `--body`, `--data`, or `-d`.
6. Use `-o json` for analysis. Use `-f` only after inspecting the response shape. The top-level response envelope includes status, headers, and `body`. List bodies are NOT uniform: `list-calls`, `list-results`, `list-call-results`, and `list-auth-settings` return `{"meta":..., "results":[...]}`; `list-schedules` returns `{"data":[...]}`; `list-slos`, `list-browser-monitors`, and `list-mcp-monitors` return a bare `{"results":[...]}` with no `meta`/pagination. Inspect each command's own output before writing an `-f` path (e.g. `-f body.results[0]` vs `-f body.data[0]`).
7. Use `-q key=value` only for query parameters confirmed by command help or observed request documentation.
8. Preserve evidence. Record the active project, commands run, IDs, time window, and the smallest response excerpts needed to support conclusions.
9. Never print, store, or paste credentials into the report. Prefer existing auth-setting IDs. Do not include bearer tokens, cookies, API keys, client secrets, or private certificate contents.
10. Do not mutate project configuration unless the user explicitly authorized the change. Before any mutation, show the planned objects and rollback path.
11. Do not poll in a tight loop. Use sensible pauses and bounded attempts.
12. When a command is absent, report the limitation and show the closest CLI-supported path. Do not fabricate a command.


## Required inputs

Resolve these from the request or ask only for missing decisions that materially affect the build:

- project name and organization
- monitor type: API call, browser, MCP, or a mixture
- target URLs and methods
- authentication approach or existing auth-setting ID
- expected success conditions
- frequency and preferred locations/agents
- tags, owner, environment, and SLO expectations
- whether production changes are authorized now

Use conservative defaults when the user delegates them: 5-minute frequency, one representative monitor per critical journey, no plaintext secrets, and an on-demand verification before scheduling broadly.

## Workflow

### 1. Preflight and command discovery

Run the operating-rule preflight. Inspect the help for likely project operations:

```bash
apimetrics --help
apimetrics project --help
```

The `project` group exposes only `select` and `show` — **the CLI cannot create a project.** Use `apimetrics project select` to choose an existing project (created in the APImetrics web app) and state this plainly if the user expected creation. Re-check `apimetrics project --help` in case a future build adds more.

Confirm the selected project after creation/selection:

```bash
apimetrics project show
apimetrics project show --id
```

### 2. Inventory before creating

Discover and run the available read-only inventory commands, normally including:

```bash
apimetrics list-calls -o json
apimetrics list-browser-monitors -o json
apimetrics list-mcp-monitors -o json
apimetrics list-schedules -o json
```

Some builds may use different monitor names. Use `apimetrics --help` as authority. Identify duplicates before creating anything.

### 3. Design the minimum useful monitoring set

Create a short plan table containing:

- proposed monitor name
- monitor type
- target
- authentication reference
- assertions or expected outcome
- frequency
- locations/agents
- schedule
- verification method

For an API service, cover at least a health/read endpoint and one critical authenticated or transactional flow when authorized. For browser or MCP, use the platform-supported schema shown by command help.

### 4. Create monitors

Inspect each create command's help immediately before use. Known minimal patterns include:

API call:
```bash
apimetrics create-call <<'EOF'
{
  "meta": {
    "name": "<monitor-name>"
  },
  "request": {
    "method": "GET",
    "url": "https://example.com/health"
  }
}
EOF
```

Browser monitor:
```bash
apimetrics create-browser-monitor <<'EOF'
{
  "name": "<monitor-name>",
  "url": "https://example.com/"
}
EOF
```

MCP monitor:
```bash
apimetrics create-mcp-monitor <<'EOF'
{
  "name": "<monitor-name>",
  "url": "https://mcp.example.com/sse",
  "steps": [
    {
      "step_type": "list_tools",
      "timeout_ms": 10000
    }
  ]
}
EOF
```

Treat these as starting patterns, not substitutes for current help. Capture every returned ID. If a create response does not contain an ID, stop that branch and diagnose it.

If the user already has the request as a `curl` command, convert it instead of hand-writing the body:

```bash
apimetrics build-call-from-curl <<'EOF'
{ "request": "curl -X GET https://example.com/health -H 'Accept: application/json'" }
EOF
```

`build-call-from-curl` returns a call definition (it does **not** persist it); pipe or paste the `meta`/`request` object into `create-call`.

### 5. Configure assertions, auth, and metadata

Use only fields confirmed by the command's option/body schema. The `create-call` body carries `meta` (`name`, `description`, `domain`, `tags`, `username`, `workspace`) and `request` (`method`, `url`, `headers[{key,value}]`, `parameters`, `body`, `auth_id`, `token_id`, `file_id`).

- Prefer references to existing auth settings (`auth_id`/`token_id` from `list-auth-settings`) over embedding secrets.
- Add `meta.tags`/`meta.domain` when the schema supports them.
- **Assertions for API calls are NOT part of the `create-call` body.** Post-test conditions (assertions) are set with a separate command:
  ```bash
  apimetrics get-call-conditions <call-id>          # read current conditions
  apimetrics set-call-conditions <call-id> <<'EOF'  # replace conditions
  {
    "meta": { "call_id": "<call-id>", "break_on_fail": false },
    "conditions": [ { "source": "...", "condition": "...", "val": "..." } ]
  }
  EOF
  ```
  Inspect `apimetrics set-call-conditions --help` for the exact `condition`/`source` grammar before writing one. Without conditions, APImetrics grades a call on its HTTP status alone.
- For MCP, verify the transport/endpoint type and step schema.
- For browser, configure a journey or performance expectations only when supported by this CLI build.

Read the created object back (`get-call <call-id>` / `get-call-conditions <call-id>`) and compare it against the plan.

### 6. Create or reuse schedules

List schedules and choose a deliberate schedule. To attach to an existing schedule, inspect:

```bash
apimetrics add-call-to-schedule --help
```

The known positional pattern is:

```bash
apimetrics add-call-to-schedule <schedule-id> <target-id>
```

To create a schedule, inspect current help and use stdin JSON. A common body is:

```bash
apimetrics create-schedule <<'EOF'
{
  "name": "<schedule-name>",
  "frequency": 300,
  "targets": ["<monitor-id>"]
}
EOF
```

Frequency is in seconds. The body also accepts `locations` (agent IDs), `regions`, `tags`, and `backoff_method` (`fibo`, `expo`, `constant`, or `none`). Read the schedule back with `get-schedule <schedule-id>` and confirm every intended target is present. Note `list-schedules` returns its list under `data`, not `results`.

### 7. Verify on demand

For API calls (`run-call` accepts an optional body of `location_id`, `run_delay` (0–86400s), and `context`; with no body it runs at the default location):

```bash
apimetrics run-call <call-id>
apimetrics list-call-results <call-id> -o json
```

`run-call` returns `{call_id, location_id, result_id}`. `list-call-results` takes the call ID and supports `--since`/`--before` (ISO-8601) and `--limit` (max 100).

For browser or MCP monitors (both use `run-monitor`; there is no `run-mcp-monitor` command):

```bash
apimetrics run-monitor <monitor-id> <<'EOF'
{}
EOF
apimetrics get-result <result-id> -o json
```

Wait between polls. Match the exact returned `result_id`. The `result` field is one of `PASS`, `FAIL`, `WARN`, `ERROR`, `TIMEOUT`, or `QUEUED`; poll until it leaves `QUEUED`. Note that `get-result` returns only a summary (`result`, `http_code`, `response_time` ms, `location_id`, `test`, `created`); deeper forensics live in `get-result-content`, `get-result-screenshot`, and the `query-*` commands. A successful HTTP transport alone is not enough: verify the platform result status and any configured conditions.

### 8. Final audit and handoff

Re-list monitors and schedules. Produce:

- active project and project ID
- objects created or reused
- monitor IDs and schedule IDs
- verification result IDs and outcomes
- frequencies and agents/locations
- auth references without secrets
- known gaps and next recommended monitors
- rollback commands or object IDs to remove. The reversal commands are: `remove-call-from-schedule <schedule-id> <target-id>` to detach, then `delete-call`, `delete-browser-monitor`, `delete-mcp-monitor`, or `delete-schedule` to remove created objects

## Failure handling

- `400`: inspect current command help and required body fields.
- `401/403`: re-authenticate and confirm the selected project; do not assume an unsupported `--api-key` flag.
- `409`: look for an existing duplicate and prefer reuse/update.
- `422`: report quota or validation details exactly as returned.
- no result: confirm the run was queued, use the exact result ID, and wait within the monitor timeout plus processing time.
- failure after creation: do not broaden rollout. Diagnose or remove the failed object according to the authorized rollback plan.

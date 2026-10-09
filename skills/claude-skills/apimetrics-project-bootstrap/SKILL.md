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
4. Commands are generally flat (`list-calls`, `create-call`). A command the server has deprecated is hidden from `--help` and prints `Command "X" is deprecated, do not use` on stderr; use the replacement `--help` lists (for example `list-api-monitors`/`list-monitors` and `create-api-monitor` where `list-calls`/`create-call` are deprecated).
5. Create and update operations read JSON from stdin (CLI Shorthand arguments also work, but quote any value containing `//` and add `</dev/null`, since shorthand also reads stdin). Prefer a quoted heredoc:
   ```bash
   apimetrics <create-or-update-command> ... <<'EOF'
   {
     "example": true
   }
   EOF
   ```
6. Use `-o json` for analysis; it prints the response body. Use `-f` only after inspecting the response shape: `-f` paths are rooted at the whole response (`status`, `headers`, `body`), so they start with `body.`, and a path that matches nothing prints nothing — empty `-f` output is not evidence of an empty list. List bodies are NOT uniform: `list-calls`, `list-api-monitors`, `list-results`, `list-results-by-call`, and `list-auth-settings` return `{"meta":..., "results":[...]}`; `list-schedules` and `list-monitors` return `{"data":[...], "meta":...}`; `list-browser-monitors` and `list-mcp-monitors` return a bare `{"results":[...]}` with no `meta`/pagination. Single-quote `-f` paths so the shell doesn't expand `[]` (e.g. `-f 'body.results[0]'` vs `-f 'body.data[0]'`).
7. Use `-q key=value` only for query parameters confirmed by command help or observed request documentation; the CLI appends it without validation.
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

The `project` noun-group exposes `select` and `show`. Project creation is the top-level command `create-project-in-org <org-id>` (listed under "Projects Commands" in `apimetrics --help`). It requires org-admin rights; for a user without them it returns `403 Forbidden`, in which case use `apimetrics project select` to choose an existing project (created by an org admin or in the APImetrics web app) and state the permission limitation plainly.

**If you do create a project with `create-project-in-org`, it does not grant you access to it.** Immediately follow with `create-project-access <project-id>` for your own account, or every subsequent `create-call`/monitor command in that project will 403. With a body of just `{"access_level": "OWNER"}` (no `account_id` or `email`) it grants the calling account. Check that `account_email` in the response is the intended account, then verify with `list-project-access <project-id>` before proceeding. Grants are not de-duplicated, so do not repeat the call.

To work in a project without changing the saved selection (for example, one you just created), pass the global `--project-id <id>` flag on each command.

Confirm the selected project after creation/selection:

```bash
apimetrics project show
apimetrics project show --id
```

### 2. Inventory before creating

Discover and run the available read-only inventory commands, normally including:

```bash
apimetrics list-monitors -o json
apimetrics list-calls -o json
apimetrics list-browser-monitors -o json
apimetrics list-mcp-monitors -o json
apimetrics list-schedules -o json
```

`list-monitors` covers every type (`{data, meta}`, type in `meta.monitor_type`: `api`, `www`, `mcp`) and pages with `--cursor`. Some builds may use different monitor names. Use `apimetrics --help` as authority. Identify duplicates before creating anything: the platform does not reject a second monitor with the same name.

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

Where the server has deprecated `create-call`, use `create-api-monitor`, which takes a flat body (`name`, `method`, `url`, plus optional `headers`, `parameters`, `body`, `auth_id`, `token_id`, `file_id`, `description`, `tags`, `workspace`):
```bash
apimetrics create-api-monitor <<'EOF'
{ "name": "<monitor-name>", "method": "GET", "url": "https://example.com/health" }
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
  "url": "https://mcp.example.com/mcp",
  "steps": [
    {
      "step_type": "list_tools",
      "timeout_ms": 10000,
      "wait_ms": 0
    }
  ]
}
EOF
```

The MCP `url` must be the server's Streamable HTTP endpoint (usually `/mcp`); an SSE-only endpoint (`/sse`) fails with HTTP 405. Every MCP step needs `timeout_ms` and `wait_ms`.

Treat these as starting patterns, not substitutes for current help. Capture every returned ID. If a create response does not contain an ID, stop that branch and diagnose it.

If the user already has the request as a `curl` command, convert it instead of hand-writing the body:

```bash
apimetrics build-call-from-curl <<'EOF'
{ "request": "curl -X GET https://example.com/health -H 'Accept: application/json'" }
EOF
```

`build-call-from-curl` returns a call definition (it does **not** persist it); pipe or paste the `meta`/`request` object into `create-call`. For `create-api-monitor`, flatten it: `name` from `meta.name`; `method`, `url`, `headers`, `parameters`, `body`, and `auth_id` from `request`.

### 5. Configure assertions, auth, and metadata

Use only fields confirmed by the command's option/body schema. The `create-call` body carries `meta` (`name`, `description`, `tags`, `workspace`) and `request` (`method`, `url`, `headers[{key,value}]`, `parameters`, `body`, `auth_id`, `token_id`, `file_id`). The monitor's domain is derived from `request.url` and cannot be set.

- Prefer references to existing auth settings (`auth_id`/`token_id` from `list-auth-settings`) over embedding secrets.
- Add `meta.tags` and `meta.description` when the schema supports them.
- **Assertions for API calls are NOT part of the `create-call` body.** Post-test conditions (assertions) are set with a separate command:
  ```bash
  apimetrics get-call-conditions <call-id>          # read current conditions
  apimetrics set-call-conditions <call-id> <<'EOF'  # replace conditions
  {
    "meta": { "call_id": "<call-id>", "break_on_fail": false },
    "conditions": [
      { "source": "HTTP_STATUS", "condition": "EQUAL_TO", "val": "200", "test_result_on_false": "HTTP_CLIENT_ERROR" }
    ]
  }
  EOF
  ```
  `meta` is optional. The help does not list the allowed values, and an unknown value returns 400 (`Unknown condition source` / `Unknown condition operator`). Sources include e.g. `HTTP_STATUS`, `HEADERS`, `RESPONSE_BODY`, `RESPONSE_TIME`, `RESPONSE_SIZE`, `NAMELOOKUP_TIME`, `TIME_TO_CONNECT`, `TIME_TO_FIRST_BYTE`, and for MCP monitors `INITIALIZE_TIME`, `STEP_RESULT`, `STEP_TIME`, `STEP_ERROR`. Operators include `EXISTS`, `DOES_NOT_EXIST`, `EQUAL_TO`, `NOT_EQUAL_TO`, `LESS_THAN`, `LESS_THAN_OR_EQUAL_TO`, `GREATER_THAN`, `GREATER_THAN_OR_EQUAL_TO`, `CONTAINS`, `DOES_NOT_CONTAIN`. `test_result_on_true`/`test_result_on_false` take result codes such as `HTTP_CLIENT_ERROR`, `CONTENT_ERROR`, `SLA_WARNING`, `SLOW_COMPLETE`. Without call conditions, a call is graded on its HTTP status plus the project-wide conditions (`get-project-conditions`), which by default mark slow responses `SLOW`.
- For MCP, verify the transport/endpoint type and step schema.
- For browser, configure a journey or performance expectations only when supported by this CLI build.

Read the created object back (`get-monitor <monitor-id>` for any type, or `get-call`/`get-api-monitor`, `read-browser-monitor`, `read-mcp-monitor`; plus `get-call-conditions <call-id>`) and compare it against the plan. Project the output with `-f` — monitor configs can hold literal credentials in headers or bodies.

### 6. Create or reuse schedules

List schedules and choose a deliberate schedule. To attach to an existing schedule, inspect:

```bash
apimetrics add-call-to-schedule --help
```

The known positional pattern is:

```bash
apimetrics add-call-to-schedule <schedule-id> <target-id>
```

It rejects an invalid ID, another project's ID, swapped arguments, or a target already in the schedule with 400, but accepts a valid ID of a different monitor in this project without warning — confirm the ID first.

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

Frequency is in seconds. The body also accepts `locations` (agent IDs), `regions`, `tags`, and `backoff_method` (`fibo`, `expo`, `constant`, or `none`). The response carries the schedule ID at `meta.id` and its targets at `schedule.target_ids`. Read the schedule back with `get-schedule <schedule-id>` and confirm every intended target is present. Note `list-schedules` returns its list under `data`, not `results`.

### 7. Verify on demand

One command, `run-monitor <monitor-id>`, runs API calls, browser monitors, and MCP monitors alike. It requires a JSON body — `{}` for defaults — with optional `location_id`, `run_delay` (seconds), and `context`; without a `location_id` the platform picks a location suited to the monitor type:

```bash
apimetrics run-monitor <monitor-id> <<'EOF'
{}
EOF
apimetrics get-result <result-id> -o json
```

`run-monitor` returns `{call_id, location_id, result_id, ...}`. For a monitor's result history (any type) use `list-results-by-call <monitor-id>` — flags are `--from`, `--time`, `--result-category` (`PASS`/`SLOW`/`WARNING`/`FAIL`; any other value is silently ignored and returns every category), `--cursor`, `--limit` (default 100).

Wait between polls. Match the exact returned `result_id`. Poll until `meta.result` is no longer `QUEUED`; `meta.result_category` is empty until then, and afterwards one of `PASS`, `SLOW`, `WARNING`, or `FAIL`, with `meta.result` holding the detailed code (`COMPLETE`, `HTTP_CLIENT_ERROR`, `CONNECTION_ERROR`, `SLA_ERROR`, …). Read the outcome with `-f 'body.meta.{result_category, result, http_code}'`; total time is `timing.total` in ms. **`get-result`'s detail depends on your role**: a below-ANALYST caller gets a summary only (`meta` and `timing`, no `request`, `response`, or `context`), but most project members (ANALYST or above) get the full object back by default — response headers and body, request method/URL/headers, DNS/TLS detail, and the `timing` breakdown, no separate call needed; `--mode full` adds the request body and log messages. Check what actually comes back before reaching for `get-result-content`/`get-result-screenshot`/the `query-*` commands for deeper forensics. A successful HTTP transport alone is not enough: verify the platform result status and any configured conditions.

### 8. Final audit and handoff

Re-list monitors and schedules. Produce:

- active project and project ID
- objects created or reused
- monitor IDs and schedule IDs
- verification result IDs and outcomes
- frequencies and agents/locations
- auth references without secrets
- known gaps and next recommended monitors
- rollback commands or object IDs to remove. The reversal commands are: `remove-call-from-schedule <schedule-id> <target-id>` to detach, then `delete-call` (or `delete-api-monitor` where `delete-call` is deprecated), `delete-browser-monitor`, `delete-mcp-monitor`, or `delete-schedule` to remove created objects

## Failure handling

- `400`: a value was rejected; read `error_msg` and inspect current command help.
- `401/403`: re-authenticate with `apimetrics login` and confirm the selected project with `apimetrics project show`.
- duplicates: the platform mostly does not reject them (a same-name monitor or a repeated access grant is created again), so rely on the inventory step and prefer reuse/update.
- `422`: schema validation; `detail[].loc` names the field. Report it exactly as returned.
- out of quota: the error reads `Project is out of quota`; report it and stop creating runs.
- no result: confirm the run was queued, use the exact result ID, and wait within the monitor timeout plus processing time.
- failure after creation: do not broaden rollout. Diagnose or remove the failed object according to the authorized rollback plan.

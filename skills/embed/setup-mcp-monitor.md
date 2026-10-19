---
name: setup-mcp-monitor
description: >
  Set up an MCP monitor in APImetrics: create the monitor with session steps,
  attach or create a schedule, run on-demand, and verify the result.
  Use when asked to create, configure, or test an MCP monitor.
---

## Input format

`apimetrics` create commands take their body from stdin or from CLI Shorthand
arguments (in shorthand, quote values containing `//` and add `</dev/null` when
running non-interactively). Prefer stdin with heredoc syntax — it keeps nested JSON unambiguous:

```bash
apimetrics create-mcp-monitor <<'EOF'
{ ... }
EOF
```

## Steps

### 1. Create the MCP monitor

```bash
apimetrics create-mcp-monitor <<'EOF'
{
  "name": "<monitor-name>",
  "url": "<mcp-server-url>"
}
EOF
```

Save the returned `id` — used in all subsequent steps.

Optional fields:
- `"steps"` — steps to execute during the MCP session (default: one `list_tools` step). Every step needs `timeout_ms` and `wait_ms`. Step types include `list_tools`, `call_tool` (`tool_name`), `list_resources`, `read_resource` (`resource_uri`), `list_prompts`, `get_prompt` (`prompt_name`), and `wait`
- `"auth_id"` — Auth Settings ID if the MCP server requires authentication
- `"token_id"` — Auth Token ID for token-based auth
- `"overall_timeout_ms"` — session timeout in milliseconds (default: 30000, maximum 120000)
- `"description"` — human-readable description
- `"tags"` — list of tags for grouping
- `"workspace"` — workspace ID if using workspaces

Example with steps:
```bash
apimetrics create-mcp-monitor <<'EOF'
{
  "name": "Check tools endpoint",
  "url": "https://mcp.example.com/mcp",
  "steps": [{"step_type": "list_tools", "timeout_ms": 10000, "wait_ms": 0}]
}
EOF
```

**Validation gate:** Response must contain an `id` field. A 422 means `name` or `url` is missing, or a step lacks `timeout_ms`/`wait_ms` (`detail[].loc` names it). The `url` must be the server's reachable Streamable HTTP MCP endpoint (usually `/mcp`).

### 2. Attach a schedule

Always offer scheduling after creating the monitor, unless the user has explicitly said they do not want one.

First, list existing schedules so the user can choose:
```bash
apimetrics list-schedules
```

Present the options to the user:
- Attach to one of the existing schedules
- Create a new schedule and attach this monitor to it
- Skip scheduling for now

Wait for the user's choice before proceeding.

**To attach to an existing schedule** (both IDs are positional arguments):
```bash
apimetrics add-call-to-schedule <schedule-id> <monitor-id>
```

**To create a new schedule:**
```bash
apimetrics create-schedule <<'EOF'
{
  "name": "<schedule-name>",
  "frequency": 300,
  "targets": ["<monitor-id>"]
}
EOF
```

`frequency` is in seconds. Common values: `60` (1 min), `300` (5 min), `3600` (1 hour).

**Validation gate:** Confirm the schedule lists the monitor ID in `schedule.target_ids` before proceeding (the schedule's own ID is at `meta.id`).

### 3. Run the monitor on-demand

`run-monitor` takes the monitor ID as a positional argument and requires a JSON body:
```bash
apimetrics run-monitor <monitor-id> <<'EOF'
{}
EOF
```

Save the `result_id` from the response — used in the next step.

**Validation gate:** Response must include a `result_id`. A 422 is a body validation error (for example, no body was sent); an out-of-quota project returns a separate `Project is out of quota` error.

### 4. Poll result to verify

```bash
apimetrics get-result <result-id>
```

Poll until `meta.result` is no longer `QUEUED`; `meta.result_category` is empty until then. Allow up to the `overall_timeout_ms` value plus processing time. Wait 10–15 seconds between checks.

**Validation gate:** `meta.result_category` must be `PASS`. `SLOW` means it passed but was slow; `WARNING` or `FAIL` indicate a problem — check `meta.result` and `meta.http_code` for the cause. Common failure causes: unreachable server, auth failure, an SSE-only endpoint (`WARNING` / `HTTP_CLIENT_ERROR` / 405), or a step that did not return the expected tool response.

## Hard rules

- Always verify the monitor ID before attaching to a schedule. Invalid or other-project IDs are rejected with 400, but a valid ID of a different monitor in this project is accepted without warning.
- `run-monitor` and `get-result` take positional arguments — do not use `--monitor-id` or `--result-id` flags.
- `add-call-to-schedule` takes two positional args: schedule ID first, then target ID.
- MCP monitor runs are bounded by `overall_timeout_ms`. Sessions exceeding this limit are terminated and reported as failures.
- Do not poll results in a tight loop. Wait 10–15 seconds between checks.
- `frequency` on schedules is in seconds, not minutes.
- The `url` must be the server's Streamable HTTP MCP endpoint (usually `/mcp`). SSE-only endpoints (`/sse`) fail with HTTP 405.

## Error recovery

- **422 on create:** `name` or `url` is missing, or a step lacks `timeout_ms`/`wait_ms`; `detail[].loc` names it.
- **401/403:** Confirm login state and that a project is active with `apimetrics project show`.
- **Session timeout failures:** Increase `overall_timeout_ms` and re-run.
- **422 on run:** The body failed validation — send at least `{}`.
- **Out of quota on run:** The error reads `Project is out of quota`. Check billing or reduce monitor frequency.
- **No result after 120s:** Verify the MCP server URL is reachable before retrying.

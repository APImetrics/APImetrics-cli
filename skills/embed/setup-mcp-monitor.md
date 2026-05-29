---
name: setup-mcp-monitor
description: >
  Set up an MCP monitor in APImetrics: create the monitor with session steps,
  attach or create a schedule, run on-demand, and verify the result.
  Use when asked to create, configure, or test an MCP monitor.
---

## Input format

All `apimetrics` create commands read JSON from stdin. Use heredoc syntax:

```bash
apimetrics create-mcp-monitor <<'EOF'
{ ... }
EOF
```

There is no `--body`, `--data`, or `-d` flag on any `apimetrics` command.

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
- `"steps"` — steps to execute during the MCP session
- `"auth_id"` — Auth Settings ID if the MCP server requires authentication
- `"token_id"` — Auth Token ID for token-based auth
- `"overall_timeout_ms"` — session timeout in milliseconds (default: 30000)
- `"description"` — human-readable description
- `"tags"` — list of tags for grouping
- `"workspace"` — workspace ID if using workspaces

Example with steps:
```bash
apimetrics create-mcp-monitor <<'EOF'
{
  "name": "Check tools endpoint",
  "url": "https://mcp.example.com/sse",
  "steps": [{"step_type": "list_tools", "timeout_ms": 10000}]
}
EOF
```

**Validation gate:** Response must contain an `id` field. If creation fails with 400, confirm `name` and `url` are both present and that the URL points to a reachable MCP SSE endpoint.

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

**To attach to an existing schedule:**
```bash
apimetrics add-call-to-schedule --schedule-id <schedule-id> --target-id <monitor-id>
```

**To create a new schedule:**
```bash
apimetrics create-schedule \
  --name "<schedule-name>" \
  --frequency 300 \
  --targets <monitor-id>
```

`--frequency` is in seconds. Common values: `60` (1 min), `300` (5 min), `3600` (1 hour).

**Validation gate:** Confirm the schedule lists the monitor ID in its targets before proceeding.

### 3. Run the monitor on-demand

```bash
apimetrics run-monitor --monitor-id <monitor-id>
```

Save the `result_id` from the response — used in the next step.

**Validation gate:** Response must include a `result_id`. A 422 means the project is out of quota.

### 4. Poll result to verify

```bash
apimetrics get-result-content --result-id <result-id> --path /
```

Poll until the result is available. Allow up to the `overall_timeout_ms` value plus processing time. Wait 10–15 seconds between checks.

**Validation gate:** Confirm the result shows a successful session with no errors. Common failure causes: unreachable server, auth failure, or a step that did not return the expected tool response.

## Hard rules

- Always verify the monitor ID before attaching to a schedule — attaching the wrong ID silently succeeds.
- MCP monitor runs are bounded by `overall_timeout_ms`. Sessions exceeding this limit are terminated and reported as failures.
- Do not poll results in a tight loop. Wait 10–15 seconds between checks.
- `--frequency` on schedules is in seconds, not minutes.
- The `url` must be an SSE endpoint, not a plain HTTP endpoint.

## Error recovery

- **400 on create:** Confirm `name` and `url` are both provided and that the URL is a valid SSE endpoint.
- **401/403:** Confirm `--api-key` or project is configured. Run `apimetrics project show` to check the active project.
- **Session timeout failures:** Increase `overall_timeout_ms` and re-run.
- **422 on run:** Project is out of quota. Check billing or reduce monitor frequency.
- **No result after 120s:** Verify the MCP server URL is reachable before retrying.

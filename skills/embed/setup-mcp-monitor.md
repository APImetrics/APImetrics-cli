---
name: setup-mcp-monitor
description: >
  Set up an MCP monitor in APImetrics: create the monitor with session steps,
  attach or create a schedule, run on-demand, and verify the result.
  Use when asked to create, configure, or test an MCP monitor.
---

## Steps

### 1. Create the MCP monitor

```bash
apimetrics MCP-monitors create \
  --name "<monitor-name>" \
  --url "<mcp-server-url>"
```

Save the returned monitor ID — used in all subsequent steps.

Optional flags:
- `--steps` — steps to execute during the MCP session (tool calls, prompts, etc.)
- `--auth-id` — Auth Settings ID if the MCP server requires authentication
- `--token-id` — Auth Token ID for token-based auth
- `--overall-timeout-ms` — session timeout in milliseconds (default: 30000)
- `--description` — human-readable description
- `--tags` — tags for grouping (repeatable)
- `--workspace` — workspace ID if using workspaces

Example with steps:
```bash
apimetrics MCP-monitors create \
  --name "Check tools endpoint" \
  --url "https://mcp.example.com/sse" \
  --steps "<steps-definition>"
```

**Validation gate:** Response must contain an `id` field. If creation fails with 400, confirm `--name` and `--url` are both present and that the URL points to a reachable MCP SSE endpoint.

### 2. Attach a schedule

**Option A — attach to an existing schedule:**
```bash
apimetrics schedules add-call-to --schedule-id <schedule-id> --target-id <monitor-id>
```

**Option B — create a new schedule with the monitor already attached:**
```bash
apimetrics schedules create \
  --name "<schedule-name>" \
  --frequency 300 \
  --targets <monitor-id>
```

`--frequency` is in seconds. Common values: `60` (1 min), `300` (5 min), `3600` (1 hour).

**Validation gate:** Confirm the schedule lists the monitor ID in its targets before proceeding.

### 3. Run the monitor on-demand

```bash
apimetrics monitors run --monitor-id <monitor-id>
```

Save the `result_id` from the response — used in the next step.

**Validation gate:** Response must include a `result_id`. A missing result ID means the run was not queued. A 422 means the project is out of quota.

### 4. Poll result to verify

```bash
apimetrics results get-result-content --result-id <result-id> --path /
```

Poll until the result is available. MCP monitors run a full session against the server — allow up to the `--overall-timeout-ms` value plus processing time before concluding a run has failed.

**Validation gate:** Confirm the result shows a successful session with no errors. If the result shows a failure, inspect the error details — common causes are unreachable server, auth failure, or a step that did not return the expected tool response.

## Hard rules

- Always verify the monitor ID before attaching to a schedule — attaching the wrong ID silently succeeds.
- MCP monitor runs are bounded by `--overall-timeout-ms`. If the session takes longer than this value, it will be terminated and reported as a failure.
- Do not poll results in a tight loop. Wait 10–15 seconds between checks.
- `--frequency` on schedules is in seconds, not minutes.
- The `--url` must be an SSE endpoint, not a plain HTTP endpoint.

## Error recovery

- **400 on create:** Missing required fields. Confirm `--name` and `--url` are both provided and that the URL is a valid SSE endpoint.
- **401/403:** Confirm `--api-key` or `--apimetrics-project-id` is set, or that environment variable `APIMETRICS_APIMETRICS_PROJECT_ID` is configured. If the MCP server itself requires auth, ensure `--auth-id` or `--token-id` is set.
- **Session timeout failures:** Increase `--overall-timeout-ms` on the monitor via `apimetrics MCP-monitors update` and re-run.
- **422 on run:** Project is out of quota. Check billing or reduce monitor frequency.
- **No result after 120s:** Check the monitor is reachable with a direct request to the MCP server URL before retrying.

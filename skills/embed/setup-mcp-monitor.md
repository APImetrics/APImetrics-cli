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
apimetrics create-mcp-monitor \
  --name "<monitor-name>" \
  --url "<mcp-server-url>"
```

Save the returned monitor ID — used in all subsequent steps.

Optional flags:
- `--steps` — steps to execute during the MCP session
- `--auth-id` — Auth Settings ID if the MCP server requires authentication
- `--token-id` — Auth Token ID for token-based auth
- `--overall-timeout-ms` — session timeout in milliseconds (default: 30000)
- `--description` — human-readable description
- `--tags` — tags for grouping (repeatable)
- `--workspace` — workspace ID if using workspaces

Example with steps:
```bash
apimetrics create-mcp-monitor \
  --name "Check tools endpoint" \
  --url "https://mcp.example.com/sse" \
  --steps "<steps-definition>"
```

**Validation gate:** Response must contain an `id` field. If creation fails with 400, confirm `--name` and `--url` are both present and that the URL points to a reachable MCP SSE endpoint.

### 2. Attach a schedule

**Option A — attach to an existing schedule:**
```bash
apimetrics add-call-to-schedule --schedule-id <schedule-id> --target-id <monitor-id>
```

**Option B — create a new schedule with the monitor already attached:**
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

Poll until the result is available. Allow up to the `--overall-timeout-ms` value plus processing time. Wait 10–15 seconds between checks.

**Validation gate:** Confirm the result shows a successful session with no errors. Common failure causes: unreachable server, auth failure, or a step that did not return the expected tool response.

## Hard rules

- Always verify the monitor ID before attaching to a schedule — attaching the wrong ID silently succeeds.
- MCP monitor runs are bounded by `--overall-timeout-ms`. Sessions exceeding this limit are terminated and reported as failures.
- Do not poll results in a tight loop. Wait 10–15 seconds between checks.
- `--frequency` on schedules is in seconds, not minutes.
- The `--url` must be an SSE endpoint, not a plain HTTP endpoint.

## Error recovery

- **400 on create:** Confirm `--name` and `--url` are both provided and that the URL is a valid SSE endpoint.
- **401/403:** Confirm `--api-key` or project is configured. Run `apimetrics project` to check the active project. If the MCP server itself requires auth, ensure `--auth-id` or `--token-id` is set.
- **Session timeout failures:** Increase `--overall-timeout-ms` via `apimetrics update-mcp-monitor` and re-run.
- **422 on run:** Project is out of quota. Check billing or reduce monitor frequency.
- **No result after 120s:** Verify the MCP server URL is reachable before retrying.

---
name: setup-api-monitor
description: >
  Set up an API monitor (HTTP call) in APImetrics: create the monitor,
  attach or create a schedule, run on-demand, and verify the result.
  Use when asked to create, configure, or test an API monitor or API call.
---

## Input format

All `apimetrics` create commands read JSON from stdin. Use heredoc syntax:

```bash
apimetrics create-call <<'EOF'
{ ... }
EOF
```

There is no `--body`, `--data`, or `-d` flag on any `apimetrics` command.

## Steps

### 1. Create the API monitor

```bash
apimetrics create-call <<'EOF'
{
  "meta": {
    "name": "<monitor-name>"
  },
  "request": {
    "method": "GET",
    "url": "<target-url>"
  }
}
EOF
```

Save the returned `id` — used in all subsequent steps.

To add request headers or auth:
```bash
apimetrics create-call <<'EOF'
{
  "meta": { "name": "<monitor-name>" },
  "request": {
    "method": "GET",
    "url": "<target-url>",
    "headers": [{"key": "Accept", "value": "application/json"}],
    "auth_id": "<auth-settings-id>"
  }
}
EOF
```

**Validation gate:** Response must contain an `id` field. If creation fails with 400, check that `meta.name`, `request.url`, and `request.method` are all present.

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
apimetrics add-call-to-schedule <schedule-id> <call-id>
```

**To create a new schedule:**
```bash
apimetrics create-schedule <<'EOF'
{
  "name": "<schedule-name>",
  "frequency": 300,
  "targets": ["<call-id>"]
}
EOF
```

`frequency` is in seconds. Common values: `60` (1 min), `300` (5 min), `3600` (1 hour).

**Validation gate:** Confirm the schedule lists the monitor ID in its targets before proceeding.

### 3. Run the monitor on-demand

`run-call` takes the call ID as a positional argument:
```bash
apimetrics run-call <call-id>
```

The response contains a `result_id`. Save it for the next step.

**Validation gate:** Response must contain `result_id`. A 422 response means the project is out of quota.

### 4. Poll results to verify

```bash
apimetrics list-call-results <call-id>
```

A successful result has `result.success: true` and an HTTP status code in the 2xx range. Poll until the result from step 3 appears (match by `result_id`). Use `-f` to narrow output:

```bash
apimetrics list-call-results <call-id> -f body.results[0]
```

**Validation gate:** Confirm `result.success` is `true`. If `false`, inspect `result.failure_reason` and the response body for details.

## Hard rules

- Always verify the call ID before attaching to a schedule — attaching the wrong ID silently succeeds.
- Do not poll results in a tight loop. Wait 5–10 seconds between checks; on-demand runs typically complete within 30 seconds.
- `frequency` on schedules is in seconds, not minutes.
- `add-call-to-schedule` takes two positional args: schedule ID first, then target ID.

## Error recovery

- **400 on create:** Missing required fields. Check `meta.name`, `request.url`, and `request.method` are all present and non-empty.
- **401/403:** Confirm `--api-key` or project is configured. Run `apimetrics project show` to check the active project.
- **422 on run:** Project is out of quota. Check billing or reduce monitor frequency.
- **No result after 60s:** The run may be queued behind other runs. Increase wait time or check monitor status.

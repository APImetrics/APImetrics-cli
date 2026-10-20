---
name: setup-api-monitor
description: >
  Set up an API monitor (HTTP call) in APImetrics: create the monitor,
  attach or create a schedule, run on-demand, and verify the result.
  Use when asked to create, configure, or test an API monitor or API call.
---

## Input format

`apimetrics` create commands take their body from stdin or from CLI Shorthand
arguments. Prefer stdin with heredoc syntax — it keeps nested JSON unambiguous.
In shorthand, quote any value containing `//`, such as a URL:
`url: '"https://example.com/health"'`. Shorthand also reads stdin, so add
`</dev/null` when running it non-interactively.

```bash
apimetrics create-call <<'EOF'
{ ... }
EOF
```

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

Where the server has deprecated `create-call` (it prints `Command "create-call" is
deprecated, do not use`), use `create-api-monitor`, which takes a flat body and
returns the same kind of ID:
```bash
apimetrics create-api-monitor <<'EOF'
{
  "name": "<monitor-name>",
  "method": "GET",
  "url": "<target-url>",
  "headers": [{"key": "Accept", "value": "application/json"}],
  "auth_id": "<auth-settings-id>"
}
EOF
```

**Validation gate:** Response must contain an `id` field. A 422 means a required field is missing — `detail[].loc` names it (`meta.name`, `request.method`, `request.url`; or `name`, `method`, `url` for `create-api-monitor`). A 400 means a value was rejected, e.g. `URL must have a hostname`.

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

**Validation gate:** Confirm the schedule lists the call ID in `schedule.target_ids` before proceeding (the schedule's own ID is at `meta.id`).

### 3. Run the monitor on-demand

`run-monitor` takes the call ID as a positional argument and a JSON body (`{}` for defaults):
```bash
apimetrics run-monitor <call-id> <<'EOF'
{}
EOF
```

The response contains a `result_id`. Save it for the next step.

**Validation gate:** Response must contain `result_id`. A 422 is a body validation error (for example, no body was sent); an out-of-quota project returns a separate `Project is out of quota` error.

### 4. Poll results to verify

```bash
apimetrics get-result <result-id>
```

Poll until `meta.result` is no longer `QUEUED` (match by `result_id`); `meta.result_category` is empty until then. A successful result has `meta.result_category: PASS` and a 2xx `meta.http_code`. `SLOW` means it passed but was slow; `WARNING` (e.g. HTTP 4xx) or `FAIL` (e.g. a connection error) indicate a problem, and `meta.result` gives the detail (`COMPLETE`, `HTTP_CLIENT_ERROR`, `CONNECTION_ERROR`, …). Total time is `timing.total` in ms. To read just the outcome:

```bash
apimetrics get-result <result-id> -f 'body.meta.{result_category, result, http_code}'
```

For the full run history of this monitor instead of a single result, use `list-results-by-call <monitor-id>`:

```bash
apimetrics list-results-by-call <monitor-id> --from <iso-datetime> -f 'body.results[0]'
```

**Validation gate:** Confirm `meta.result_category` is `PASS`. If not, inspect the result (a below-ANALYST caller only gets a summary from `get-result`; most project members get the full request/response/timing detail by default) for the failure cause.

## Hard rules

- Always verify the call ID before attaching to a schedule. Invalid or other-project IDs are rejected with 400, but a valid ID of a different monitor in this project is accepted without warning.
- `run-monitor` starts a run, and `list-results-by-call` lists result history, for any monitor type — API, browser, or MCP.
- Do not poll results in a tight loop. Wait 5–10 seconds between checks; on-demand runs typically complete within 30 seconds.
- `frequency` on schedules is in seconds, not minutes.
- `add-call-to-schedule` takes two positional args: schedule ID first, then target ID.

## Error recovery

- **422 on create:** A required field is missing; `detail[].loc` names it.
- **400 on create:** A value was rejected (e.g. `URL must have a hostname`); the error message names it.
- **401/403:** Confirm login state and that a project is active with `apimetrics project show`.
- **422 on run:** The body failed validation — send at least `{}`.
- **Out of quota on run:** The error reads `Project is out of quota`. Check billing or reduce monitor frequency.
- **No result after 60s:** The run may be queued behind other runs. Increase wait time or check monitor status.

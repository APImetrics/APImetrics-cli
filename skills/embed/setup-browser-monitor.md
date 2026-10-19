---
name: setup-browser-monitor
description: >
  Set up a Browser monitor in APImetrics: create the monitor, attach or
  create a schedule, run on-demand, and verify the result.
  Use when asked to create, configure, or test a browser monitor.
---

## Input format

`apimetrics` create commands take their body from stdin or from CLI Shorthand
arguments (in shorthand, quote values containing `//` and add `</dev/null` when
running non-interactively). Prefer stdin with heredoc syntax — it keeps nested JSON unambiguous:

```bash
apimetrics create-browser-monitor <<'EOF'
{ ... }
EOF
```

## Steps

### 1. Create the browser monitor

```bash
apimetrics create-browser-monitor <<'EOF'
{
  "name": "<monitor-name>",
  "url": "<target-url>"
}
EOF
```

Save the returned `id` — used in all subsequent steps.

Optional fields:
- `"description"` — human-readable description
- `"browser_types"` — list of browsers the monitor is permitted to run on (`chromium`, `firefox`, `webkit`, `camoufox`; default `["camoufox"]`)
- `"tags"` — list of tags for grouping
- `"user_agent"` — override the User-Agent header
- `"workspace"` — workspace ID if using workspaces

**Validation gate:** Response must contain an `id` field. A 422 means `name` or `url` is missing (`detail[].loc` names it). Always include the scheme (`https://`): the server accepts a scheme-less URL and creates a monitor with an empty domain.

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

**Validation gate:** Response must include a `result_id`. A missing result ID means the run was not queued.

### 4. Poll result to verify

```bash
apimetrics get-result <result-id>
```

Poll until `meta.result` is no longer `QUEUED`; `meta.result_category` is empty until then. Wait 10–15 seconds between checks (browser runs typically complete in well under a minute).

**Validation gate:** `meta.result_category` must be `PASS`. `SLOW` means it passed but was slow; `WARNING` or `FAIL` indicate a problem — check `meta.result` and `meta.http_code` for the cause.

To save a screenshot of the page at the time of the run (a binary JPEG written to stdout):
```bash
apimetrics get-result-screenshot <result-id> > screenshot.jpg
```

## Hard rules

- Always verify the monitor ID before attaching to a schedule. Invalid or other-project IDs are rejected with 400, but a valid ID of a different monitor in this project is accepted without warning.
- `run-monitor` and `get-result` take positional arguments — do not use `--monitor-id` or `--result-id` flags.
- `add-call-to-schedule` takes two positional args: schedule ID first, then target ID.
- Browser monitors take longer to complete than API monitors — make the first poll about 15 seconds after the run starts.
- Do not poll results in a tight loop. Wait 10–15 seconds between checks.
- `frequency` on schedules is in seconds, not minutes.

## Error recovery

- **422 on create:** `name` or `url` is missing; `detail[].loc` names it. Include the scheme (`https://`) in the URL.
- **401/403:** Confirm login state and that a project is active with `apimetrics project show`.
- **No result after 120s:** Browser monitors may take longer in high-load periods. Check the monitor with `apimetrics read-browser-monitor <monitor-id>` and retry.
- **Screenshot unavailable:** Not all results include screenshots. Fall back to `get-result <result-id>` (most project members get the response detail inline) or `get-result-content <result-id> <path>`.

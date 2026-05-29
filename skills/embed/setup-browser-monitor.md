---
name: setup-browser-monitor
description: >
  Set up a Browser monitor in APImetrics: create the monitor, attach or
  create a schedule, run on-demand, and verify the result.
  Use when asked to create, configure, or test a browser monitor.
---

## Input format

All `apimetrics` create commands read JSON from stdin. Use heredoc syntax:

```bash
apimetrics create-browser-monitor <<'EOF'
{ ... }
EOF
```

There is no `--body`, `--data`, or `-d` flag on any `apimetrics` command.

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
- `"browser_types"` — list of browsers the monitor is permitted to run on
- `"tags"` — list of tags for grouping
- `"user_agent"` — override the User-Agent header
- `"workspace"` — workspace ID if using workspaces

**Validation gate:** Response must contain an `id` field. If creation fails with 400, confirm `name` and `url` are both present and that `url` includes the scheme (`https://`).

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

**Validation gate:** Response must include a `result_id`. A missing result ID means the run was not queued.

### 4. Poll result to verify

```bash
apimetrics get-result-content --result-id <result-id> --path /
```

Poll until the result is available (typically within 60 seconds for browser monitors). Wait 10–15 seconds between checks.

To get a screenshot of the page at the time of the run:
```bash
apimetrics get-result-screenshot --result-id <result-id>
```

**Validation gate:** Confirm the result shows a successful page load with no errors.

## Hard rules

- Always verify the monitor ID before attaching to a schedule — attaching the wrong ID silently succeeds.
- Browser monitors take longer to complete than API monitors — wait at least 60 seconds before polling results.
- Do not poll results in a tight loop. Wait 10–15 seconds between checks.
- `--frequency` on schedules is in seconds, not minutes.

## Error recovery

- **400 on create:** Confirm `name` and `url` are both provided and that the URL includes the scheme (`https://`).
- **401/403:** Confirm `--api-key` or project is configured. Run `apimetrics project show` to check the active project.
- **No result after 120s:** Browser monitors may take longer in high-load periods. Check the monitor with `apimetrics read-browser-monitor --monitor-id <id>` and retry.
- **Screenshot unavailable:** Not all result types include screenshots. Fall back to `get-result-content`.

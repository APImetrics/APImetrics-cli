---
name: setup-browser-monitor
description: >
  Set up a Browser monitor in APImetrics: create the monitor, attach or
  create a schedule, run on-demand, and verify the result.
  Use when asked to create, configure, or test a browser monitor.
---

## Steps

### 1. Create the browser monitor

```bash
apimetrics browser-monitors create \
  --name "<monitor-name>" \
  --url "<target-url>"
```

Save the returned monitor ID — used in all subsequent steps.

Optional flags:
- `--description` — human-readable description
- `--browser-types` — comma-separated list of browsers to run on
- `--tags` — tags for grouping (repeatable)
- `--user-agent` — override the User-Agent header
- `--workspace` — workspace ID if using workspaces

**Validation gate:** Response must contain an `id` field. If creation fails with 400, confirm `--name` and `--url` are both present and that `--url` is a valid URL.

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

**Validation gate:** Response must include a `result_id`. A missing result ID means the run was not queued.

### 4. Poll result to verify

```bash
apimetrics results get-result-content --result-id <result-id> --path /
```

Poll until the result is available (typically within 60 seconds for browser monitors). A successful result has no error and a completed status.

To get a screenshot of the page at the time of the run:
```bash
apimetrics results get-result-screenshot --result-id <result-id>
```

**Validation gate:** Confirm the result shows a successful page load with no errors. If the result shows a failure, inspect the error message and screenshot for details.

## Hard rules

- Always verify the monitor ID before attaching to a schedule — attaching the wrong ID silently succeeds.
- Browser monitors take longer to complete than API monitors — wait at least 60 seconds before polling results.
- Do not poll results in a tight loop. Wait 10–15 seconds between checks.
- `--frequency` on schedules is in seconds, not minutes.

## Error recovery

- **400 on create:** Missing required fields. Confirm `--name` and `--url` are both provided and that the URL includes the scheme (`https://`).
- **401/403:** Confirm `--api-key` or `--apimetrics-project-id` is set, or that environment variable `APIMETRICS_APIMETRICS_PROJECT_ID` is configured.
- **No result after 120s:** Browser monitors may take longer in high-load periods. Check the monitor status via `apimetrics browser-monitors read --monitor-id <id>` and retry the run.
- **Screenshot unavailable:** Not all result types include screenshots. Fall back to inspecting result content via `get-result-content`.

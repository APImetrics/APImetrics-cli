---
name: setup-api-monitor
description: >
  Set up an API monitor (HTTP call) in APImetrics: create the monitor,
  attach or create a schedule, run on-demand, and verify the result.
  Use when asked to create, configure, or test an API monitor or API call.
---

## Steps

### 1. Create the API monitor

```bash
apimetrics calls create --body '{
  "meta": {
    "name": "<monitor-name>"
  },
  "request": {
    "method": "GET",
    "url": "<target-url>"
  }
}' --jq '.id'
```

Save the returned `id` — this is the call ID used in all subsequent steps.

To add request headers or auth:
```bash
apimetrics calls create --body '{
  "meta": { "name": "<monitor-name>" },
  "request": {
    "method": "GET",
    "url": "<target-url>",
    "headers": [{"key": "Accept", "value": "application/json"}],
    "auth_id": "<auth-settings-id>"
  }
}'
```

**Validation gate:** Response must contain an `id` field. If the call creation fails with 400, check that `meta.name` and `request.url` and `request.method` are all present.

### 2. Attach a schedule

**Option A — attach to an existing schedule:**
```bash
apimetrics schedules add-call-to --schedule-id <schedule-id> --target-id <call-id>
```

**Option B — create a new schedule with the monitor already attached:**
```bash
apimetrics schedules create \
  --name "<schedule-name>" \
  --frequency 300 \
  --targets <call-id>
```

`--frequency` is in seconds. Common values: `60` (1 min), `300` (5 min), `3600` (1 hour).

**Validation gate:** For option A, a 200 response confirms the target was added. For option B, confirm the returned schedule includes the call ID in its targets.

### 3. Run the monitor on-demand

```bash
apimetrics calls run <call-id>
```

The response contains a `result_id`. Save it for the next step.

To run from a specific location:
```bash
apimetrics calls run <call-id> --body '{"location_id": "<location-id>"}'
```

**Validation gate:** Response must contain `result_id`. A 422 response means the project is out of quota.

### 4. Poll results to verify

```bash
apimetrics calls list-results <call-id> --jq '.results[0]'
```

A successful result has `result.success: true` and an HTTP status code in the 2xx range. Poll this command until the result from step 3 appears (match by `result_id`).

To fetch a specific result directly:
```bash
apimetrics results get-result-content --result-id <result-id> --path /
```

**Validation gate:** Confirm `result.success` is `true`. If `false`, inspect `result.failure_reason` and the response body for details.

## Hard rules

- Always verify the monitor ID before attaching to a schedule — attaching the wrong ID silently succeeds.
- Do not poll results in a tight loop. Wait 5–10 seconds between checks; on-demand runs typically complete within 30 seconds.
- `--frequency` on schedules is in seconds, not minutes.

## Error recovery

- **400 on create:** Missing required fields. Check `meta.name`, `request.url`, and `request.method` are all present and non-empty.
- **401/403:** Confirm `--api-key` or `--apimetrics-project-id` is set, or that environment variable `APIMETRICS_APIMETRICS_PROJECT_ID` is configured.
- **422 on run:** Project is out of quota. Check billing or reduce monitor frequency.
- **No result after 60s:** The run may have been queued behind other runs. Increase wait time or check monitor status in the dashboard.

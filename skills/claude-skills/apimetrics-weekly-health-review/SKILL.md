---
name: apimetrics-weekly-health-review
description: Analyze the previous seven complete days of APImetrics project results, summarize availability and performance, identify regressions and recurring failures, and recommend actions. Use for weekly operational reviews or customer health reports.
argument-hint: "[optional project or monitor scope]"
---

# APImetrics weekly health review

Analyze the previous seven complete calendar days in the active project's timezone unless the user specifies a different window. Compare with the preceding seven days when the CLI exposes enough history.


## Operating rules

1. Use only the `apimetrics` CLI and Claude's normal shell/file tools. Do not require `jq`, Python, custom scripts, direct HTTP clients, or undocumented platform APIs.
2. Begin with:
   ```bash
   apimetrics --version
   apimetrics project show
   apimetrics --help
   ```
   Run `apimetrics login` or `apimetrics project select` only when needed.
3. The CLI command tree is generated from the platform's current OpenAPI description. Inspect `apimetrics <command> --help` before constructing a body or assuming an option name.
4. Commands are generally flat (`list-calls`, `create-call`), not noun/verb groups.
5. Create and update operations read JSON from stdin. Use a quoted heredoc:
   ```bash
   apimetrics <create-or-update-command> ... <<'EOF'
   {
     "example": true
   }
   EOF
   ```
   Do not invent `--body`, `--data`, or `-d`.
6. Use `-o json` for analysis. Use `-f` only after inspecting the response shape. The top-level response envelope includes status, headers, and `body`. List bodies are NOT uniform: `list-calls`, `list-results`, `list-call-results`, and `list-auth-settings` return `{"meta":..., "results":[...]}`; `list-schedules` returns `{"data":[...]}`; `list-slos`, `list-browser-monitors`, and `list-mcp-monitors` return a bare `{"results":[...]}` with no `meta`/pagination. Inspect each command's own output before writing an `-f` path (e.g. `-f body.results[0]` vs `-f body.data[0]`).
7. Use `-q key=value` only for query parameters confirmed by command help or observed request documentation.
8. Preserve evidence. Record the active project, commands run, IDs, time window, and the smallest response excerpts needed to support conclusions.
9. Never print, store, or paste credentials into the report. Prefer existing auth-setting IDs. Do not include bearer tokens, cookies, API keys, client secrets, or private certificate contents.
10. Do not mutate project configuration unless the user explicitly authorized the change. Before any mutation, show the planned objects and rollback path.
11. Do not poll in a tight loop. Use sensible pauses and bounded attempts.
12. When a command is absent, report the limitation and show the closest CLI-supported path. Do not fabricate a command.


## Workflow

### 1. Establish the analysis contract

State:

- active project
- exact ISO-8601 start and end timestamps
- timezone
- included monitor types
- comparison period
- any data limits or missing result types

Calculate dates in reasoning; do not require a date utility.

### 2. Discover inventory and result commands

Run `apimetrics --help` and inspect help for the available inventory, result, and analytics operations. The grounded commands are:

```bash
apimetrics list-calls -o json                 # {meta, results:[...]}
apimetrics list-schedules -o json             # {data:[...]}  (note: data, not results)
apimetrics list-results --since <ISO> --before <ISO> -o json   # project-wide result summaries
apimetrics list-call-results <call-id> --since <ISO> --before <ISO> -o json
apimetrics get-result <result-id> -o json     # single result summary
```

`list-results` and `list-call-results` support **server-side** `--since`/`--before` (ISO-8601) plus `--limit` (max 100) and `--cursor` — prefer these over pulling everything and filtering by hand. `list-results.meta.more`/`next_cursor` drive pagination.

**Do not hand-compute percentiles from result summaries.** Result rows carry only `result`, `http_code`, `response_time` (ms), and `location_id` — not component timings. For latency distribution use the analytics commands, which compute the statistics server-side over your window:

```bash
apimetrics query-api-performance -o json <<'EOF'
{ "from": "<ISO>", "to": "<ISO>", "metrics": ["total"], "measures": ["mean","p50","p95","p99"], "group_by": ["monitor"] }
EOF
```

- `measures`: `mean`, `p50`, `p90`, `p95`, `p99`
- `metrics`: `total`, `dns`, `connect`, `tls`, `ttfb`, `response`
- `group_by`: `location`, `interval`, `monitor` (add `interval` with an ISO-8601 `interval` like `PT1H` for a time series)
- optional `status` filter: `PASS`, `SLOW`, `WARNING`, `FAIL`

Browser and MCP have parallel commands: `query-browser-performance` / `query-browser-monitor-performance`, `query-mcp-performance` / `query-mcp-monitor-performance`. Use `apimetrics --help` as authority for names in the current build.

### 3. Build the monitor census

For every monitor, capture:

- ID, name, type, tags, owner/environment if present
- enabled/disabled state
- schedule and frequency
- configured locations/agents
- target host or journey
- SLO association
- whether any results exist in the window

Flag unscheduled, disabled, orphaned, and never-run monitors separately; do not mix them into pass-rate denominators without explanation.

### 4. Collect seven-day results

Two complementary sources, both with `-o json`:

- **Availability / pass-fail** — `list-results --since --before` (project-wide) or `list-call-results <call-id> --since --before` (per monitor). Each row gives `result`, `http_code`, `response_time`, `location_id`, `test`, `created`. Page with `--cursor` until `meta.more` is false. Count by `result` category; do not disable pagination unless intentionally sampling.
- **Latency distribution** — the `query-*-performance` commands from step 2, which return server-computed `mean`/`p50`/`p95`/`p99` per metric, optionally grouped by monitor, location, and interval.

For conformance signal, `conformance-results-summary` is available but best-effort (it can return a 500 on projects with no conformance data — treat a failure as "no data", not a finding).

### 5. Analyze

For the project and each meaningful monitor/segment, report:

- total runs and runs with usable data
- pass count, fail count, warning/error/timeout count (the `result` categories are `PASS`, `FAIL`, `WARN`, `ERROR`, `TIMEOUT`, `QUEUED`)
- availability/pass rate
- median (`p50`) and `p95` latency **from `query-*-performance`**, not estimated from summaries
- slowest monitors and locations (`group_by: ["monitor"]` / `["location"]`)
- first/last failure and longest visible failure streak
- recurring failure signatures (`http_code` + `result` patterns)
- location-specific or time-of-day concentration (`group_by` with `interval`)
- changes versus the preceding week (run the same query for the prior window)
- SLO status/error budget when directly available

If the `query-*` commands return empty for the window (no data), say so explicitly rather than reporting a rate over zero runs. Label any hand-derived figure and its denominator.

### 6. Investigate the top findings

For the three most consequential findings, retrieve representative full results and compare:

- a recent failure
- the closest passing control
- same monitor in another location
- same period for related monitors

Classify each finding as likely monitor configuration, authentication, assertion/conformance, DNS, TLS, routing/network, upstream application, rate limiting, agent/location, or unknown.

### 7. Deliver the review

Use this structure:

1. **Executive summary** — health, material changes, customer impact.
2. **Coverage** — monitors included/excluded and data quality.
3. **Scorecard** — project and top monitor metrics.
4. **Regressions and incidents** — evidence and duration.
5. **Performance** — p50/p95 or best available statistics.
6. **SLO/error budget** — only where grounded.
7. **Actions** — owner, severity, evidence, remediation, verification.
8. **Appendix** — exact commands, dates, project ID, and limitations.

## Quality gates

- Every percentage has a numerator and denominator.
- Every regression has a comparison window.
- Every root-cause statement is marked confirmed, likely, possible, or unknown.
- Missing data is visible, not silently treated as passing.
- Recommendations distinguish monitor fixes from system fixes.

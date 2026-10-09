---
name: apimetrics-weekly-health-review
description: Analyze the previous seven complete days of APImetrics project results, summarize availability and performance, identify regressions and recurring failures, and recommend actions. Use for weekly operational reviews or customer health reports.
argument-hint: "[optional project or monitor scope]"
---

# APImetrics weekly health review

Analyze the previous seven complete UTC calendar days unless the user specifies a different window or timezone; the server's daily buckets (`get-call-passfail-range --kind DAY`, `interval: "P1D"`) are UTC days. Compare with the preceding seven days when the CLI exposes enough history.


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
4. Commands are generally flat (`list-calls`, `create-call`). A command the server has deprecated is hidden from `--help` and prints `Command "X" is deprecated, do not use` on stderr; use the replacement `--help` lists (for example `list-api-monitors`/`list-monitors` and `create-api-monitor` where `list-calls`/`create-call` are deprecated).
5. Create and update operations read JSON from stdin (CLI Shorthand arguments also work, but quote any value containing `//` and add `</dev/null`, since shorthand also reads stdin). Prefer a quoted heredoc:
   ```bash
   apimetrics <create-or-update-command> ... <<'EOF'
   {
     "example": true
   }
   EOF
   ```
6. Use `-o json` for analysis; it prints the response body. Use `-f` only after inspecting the response shape: `-f` paths are rooted at the whole response (`status`, `headers`, `body`), so they start with `body.`, and a path that matches nothing prints nothing — empty `-f` output is not evidence of an empty list. List bodies are NOT uniform: `list-calls`, `list-api-monitors`, `list-results`, `list-results-by-call`, and `list-auth-settings` return `{"meta":..., "results":[...]}`; `list-schedules` and `list-monitors` return `{"data":[...], "meta":...}`; `list-browser-monitors` and `list-mcp-monitors` return a bare `{"results":[...]}` with no `meta`/pagination. Single-quote `-f` paths so the shell doesn't expand `[]` (e.g. `-f 'body.results[0]'` vs `-f 'body.data[0]'`).
7. Use `-q key=value` only for query parameters confirmed by command help or observed request documentation; the CLI appends it without validation.
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
apimetrics list-monitors -o json              # {data:[...], meta} — every monitor type; type in meta.monitor_type
apimetrics list-calls -o json                 # {meta, results:[...]} — API monitors (list-api-monitors where list-calls is deprecated)
apimetrics list-schedules -o json             # {data:[...], meta}  (note: data, not results)
apimetrics list-results --from <ISO> --time <ISO> -o json   # project-wide result rows
apimetrics list-results-by-call <monitor-id> --from <ISO> --time <ISO> -o json
apimetrics get-result <result-id> -o json     # summary for below-ANALYST callers; full detail otherwise — see below
apimetrics get-call-passfail-range <call-id> --kind DAY --start <ISO> --end <ISO> -o json   # server-counted pass/warning/failure per day
```

`list-results` and `list-results-by-call` support **server-side** `--from`/`--time` plus `--result-category`, `--limit` (default 100), and `--cursor` — prefer these over pulling everything and filtering by hand. `--from`/`--time` are the window start and end as full ISO-8601 date-times (a bare date is rejected) and are minute-granular, so round the end up. `--result-category` takes `PASS`/`SLOW`/`WARNING`/`FAIL`; any other value is silently ignored and returns every category. Each call returns one page: repeat with `--cursor <meta.next_cursor>` while `meta.more` is true. `get-call-passfail-range --kind` takes `HOUR`/`DAY`/`MONTH`/`YEAR`.

**Do not hand-compute percentiles from result rows.** Each row carries one run's `timing` (`total` ms plus `dns`/`tcp`/`handshake`/`processing`/`download`/`upload`); for latency distribution across many results, use the analytics commands, which compute the statistics server-side over your window:

```bash
apimetrics query-api-performance -o json <<'EOF'
{ "from": "<ISO>", "to": "<ISO>", "metrics": ["total"], "measures": ["mean","p50","p95","p99"], "group_by": ["monitor"] }
EOF
```

- `measures`: `mean`, `p50`, `p90`, `p95`, `p99`
- `metrics`: `total`, `dns`, `connect`, `tls`, `ttfb`, `response`, `upload`, `network`, `server`, `total_no_dns`
- `group_by`: `location`, `interval`, `monitor` (`interval` requires an ISO-8601 `interval` such as `PT1H` or `P1D`)
- optional `status` filter: `PASS`, `SLOW`, `WARNING`, `FAIL` — omitted, every category is included

Browser and MCP have parallel commands: `query-browser-performance` / `query-browser-monitor-performance`, `query-mcp-performance` / `query-mcp-monitor-performance`. Their metric names differ (e.g. browser `lcp`/`fcp`/`cls`, MCP `initialize`). Use `apimetrics --help` as authority for names in the current build.

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

- **Availability / pass-fail** — `get-call-passfail-range <call-id> --kind DAY --start --end` per monitor returns server-counted `pass`/`warning`/`failure` per UTC day; sum across monitors for the project figure. `body.results` is an object keyed by day, and days with no runs are omitted — treat a missing day as no data, not as passing.
- **Failure detail** — `list-results --from --time --result-category <cat>` (project-wide) or `list-results-by-call <monitor-id> --from --time` (per monitor, any type) for the individual failing runs behind those counts. Each row is `{id, meta, timing}`: `meta.result_category`, `meta.result`, `meta.http_code`, `meta.location_id`, `meta.call_id` (monitor ID), `meta.start_time`, and `timing.total` (ms); page with `--cursor` until `meta.more` is false.
- **Latency distribution** — the `query-*-performance` commands from step 2, which return server-computed `mean`/`p50`/`p95`/`p99` per metric, optionally grouped by monitor, location, and interval.

For conformance signal, `conformance-results-summary` is available but best-effort (it can return a 500 on projects with no conformance data — treat a failure as "no data", not a finding).

### 5. Analyze

For the project and each meaningful monitor/segment, report:

- total runs and runs with usable data
- pass, warning, and failure counts from `get-call-passfail-range`; split failures by `meta.result_category` (`PASS`, `SLOW`, `WARNING`, `FAIL`; HTTP 4xx lands in `WARNING`) and the finer `meta.result` (e.g. `CONNECTION_ERROR`, `HTTP_CLIENT_ERROR`, `HTTP_SERVER_ERROR`) from the failure detail when the breakdown matters
- availability/pass rate
- median (`p50`) and `p95` latency **from `query-*-performance`**, not estimated from summaries
- slowest monitors and locations (`group_by: ["monitor"]` / `["location"]`)
- first/last failure and longest visible failure streak
- recurring failure signatures (`http_code` + `result_category` + `result` patterns)
- location-specific or time-of-day concentration (`group_by` with `interval`)
- changes versus the preceding week (run the same query for the prior window)
- SLO attainment against the `get-project-slo` objectives, from the same server-side aggregates (see `apimetrics-slo-review`)

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

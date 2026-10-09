---
name: apimetrics-slo-review
description: Review APImetrics SLO definitions and attainment, connect violations to monitor evidence, assess error-budget risk, and recommend SLO or system actions.
argument-hint: "[SLO, service, or review period]"
---

# APImetrics SLO review

Review both the SLO configuration and the evidence behind it. Do not recommend changing an SLO merely to hide poor performance.


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

### 1. Discover SLO operations

Each project has one SLO, managed with `get-project-slo`, `update-project-slo` (which creates it if none exists yet), and `delete-project-slo`. Use the global `--project-id` flag to target a project other than the active one. `get-project-slo` returns the SLO definition; attainment comes from the server-side pass/fail and performance aggregates in step 4. If `apimetrics --help` lists a project-level SLO attainment or error-budget command, use that instead.

```bash
apimetrics get-project-slo -o json    # single SLO object
```

If `created` and `last_update` are null, no SLO is stored and the object holds platform defaults — report it as "no SLO defined" rather than reviewing the defaults as the project's own. `update-project-slo` merges top-level keys into the stored SLO, so send the full `objectives` array when replacing objectives.

The object has `include_tags`/`exclude_tags` (the monitor scope, tag-based only), `objectives[]`, and `thresholds[]`. An objective has `metric` (observed values include `availability`, `slow`, `dns`, `tcp`, `casc`, plus web-vitals like `largest_contentful_paint`/`cumulative_layout_shift` — confirm the current set with `apimetrics update-project-slo --help`), `measure` (`mean`/`value`), `comparator` (`<`/`>`), `value`, `unit` (`ms`/`percent`/`pp`/`value`), a `description`, and `period` (`DAY`, `WEEK`, `MONTH`). Percent values are fractions (`0.995` = 99.5%). A threshold has `metric` (`grade`/`latency`), `period`, `unit`, `delta`, and `description`.

### 2. Define the review period

Use the user-specified period or the previous complete calendar month for formal review; use seven days for an operational pulse. State exact timestamps and timezone; the server's daily buckets are UTC days.

### 3. Inspect definitions

For each objective in the project's SLO (and, separately, each threshold) capture:

- objective target and metric
- evaluation window (`period`)
- monitor scope (`include_tags`/`exclude_tags`)
- comparator/value/unit and the `description` field
- ownership/tags on the scoped monitors themselves (the SLO object has no owner field)

Flag ambiguous or overly broad tag scope, conflicting windows across objectives, and objectives unsupported by the monitored journey.

### 4. Derive attainment evidence

Use the server's aggregates for the monitors the SLO's `include_tags`/`exclude_tags` scope to (match tags against `meta.tags` in `list-monitors -o json`, which covers every monitor type). Keep `list-results`/`list-results-by-call` for drilling into specific violations in step 5.

- For availability/pass objectives, use `get-call-passfail-range <call-id> --kind DAY|MONTH --start <ISO> --end <ISO>` per scoped monitor; it returns server-counted `pass`/`warning`/`failure` per period as an object keyed by period, omitting periods with no runs. Sum across monitors for the project figure. `get-call-passfail-total <call-id> --start --end` gives window totals as `stat.<CATEGORY>.total` for `PASS`/`SLOW`/`WARNING`/`FAIL`, broken down by `result` (e.g. `stat.WARNING.HTTP_CLIENT_ERROR`); sum the category totals for the run count — the top-level `stat.total` is not it.
- For latency objectives, use `query-api-performance` (with `group_by: ["monitor"]`, keeping only the scoped monitors) or `query-api-monitor-performance` with matching `metrics`/`measures` and window. For `DAY` objectives use `interval: "P1D"`; for `WEEK`/`MONTH` objectives run one query per calendar period with `from`/`to` set to its bounds, because `P1W`/`P1M` intervals become epoch-aligned 7- and 30-day buckets. Confirm the objective's actual `metric` name maps to a real `query-*-performance` metric before querying — don't assume it's called `total`. SLO and timing names differ from the API query names: `tcp`→`connect`, `handshake`→`tls`, `processing`→`server`, `download`→`response`; `dns`, `total`, `upload`, `network`, and `total_no_dns` keep their names; `availability`/`pass`/`slow`/`warning`/`fail` come from the pass/fail counts; web vitals map to `query-browser-performance` `lcp`/`fcp`/`cls`/`fid`; `casc` and `apdex` have no query metric.

Report:

- eligible events/runs
- good and bad events
- attainment
- objective
- remaining or consumed error budget
- burn trend
- largest violating periods
- excluded/missing data

If the aggregates do not support an exact error-budget calculation, say so and report a clearly labeled approximation.

### 5. Explain violations

For material violations, retrieve representative results and classify the cause using monitor, location, status, latency, auth, conformance, and network evidence.

Separate:

- real service reliability failure
- monitor/configuration failure
- data gap
- objective/design problem

### 6. Recommend action

Prioritize:

1. service remediation
2. monitoring/data-quality repair
3. coverage improvements
4. SLO definition changes only when the objective is structurally wrong

Deliver a table with objective (metric/scope), attainment, target, budget status, cause, owner, action, and verification — one row per objective.

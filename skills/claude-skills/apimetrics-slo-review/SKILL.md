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

### 1. Discover SLO operations

The SLO command set is CRUD only: `list-slos`, `get-slo <slo-id>` (not `read-slo`), `create-slo`, `update-slo`, `delete-slo`. There is **no SLO attainment, status, or error-budget endpoint** — the CLI returns SLO *definitions*, not computed attainment. Plan to derive attainment yourself from result and performance data (step 4).

```bash
apimetrics list-slos -o json    # bare {results:[...]}, no pagination meta
apimetrics get-slo <slo-id> -o json
```

Each SLO has `include_tags`/`exclude_tags` (the monitor scope), `scope` (`project`/`workflow`/`monitor`) + `scope_id`, `objectives[]`, and `thresholds[]`. An objective has `metric` (e.g. `availability`, `total`, `dns`, `latency`-family, plus web-vitals like `largest_contentful_paint`), `measure` (`mean`/`value`), `comparator` (`<`/`>`), `value`, `unit` (`ms`/`percent`/`pp`/`value`), and `period` (`PT5M`…`PT1H`, `DAY`, `WEEK`, `MONTH`).

### 2. Define the review period

Use the user-specified period or the previous complete calendar month for formal review; use seven days for an operational pulse. State exact timestamps and timezone.

### 3. Inspect definitions

For each scoped SLO capture:

- name/ID
- target and objective
- evaluation window
- associated monitor(s)
- threshold and measurement
- enabled state
- ownership/tags
- alerting or reporting relationship when exposed

Flag missing ownership, ambiguous scope, unattached SLOs, conflicting windows, and objectives unsupported by the monitored journey.

### 4. Derive attainment evidence

Because the CLI exposes no attainment endpoint, reconstruct it from the monitors the SLO scopes (via `include_tags`/`exclude_tags` or `scope_id`):

- For availability/pass objectives, count `result` categories over the objective `period` using `list-results`/`list-call-results` with `--since`/`--before`.
- For latency objectives (`measure: mean`, metrics like `total`), use `query-api-performance` / `query-api-monitor-performance` with matching `metrics`/`measures` and window; align the query `interval` to the objective `period`.

Report:

- eligible events/runs
- good and bad events
- attainment
- objective
- remaining or consumed error budget
- burn trend
- largest violating periods
- excluded/missing data

If the CLI does not provide enough raw information for an exact error-budget calculation, say so and report the platform-provided value or a clearly labeled approximation.

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

Deliver a table with SLO, attainment, objective, budget status, cause, owner, action, and verification.

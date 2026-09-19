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
6. Use `-o json` for analysis. Use `-f` only after inspecting the response shape. The top-level response envelope includes status, headers, and `body`. List bodies are NOT uniform: `list-calls`, `list-results`, `list-results-by-call`, and `list-auth-settings` return `{"meta":..., "results":[...]}`; `list-schedules` returns `{"data":[...]}`; `list-browser-monitors` and `list-mcp-monitors` return a bare `{"results":[...]}` with no `meta`/pagination. There is no `list-call-results` (use `list-results-by-call`) or `list-slos`/`get-slo` (SLOs are one per project — `get-project-slo` returns a bare single object). Inspect each command's own output before writing an `-f` path (e.g. `-f body.results[0]` vs `-f body.data[0]`).
7. Use `-q key=value` only for query parameters confirmed by command help or observed request documentation.
8. Preserve evidence. Record the active project, commands run, IDs, time window, and the smallest response excerpts needed to support conclusions.
9. Never print, store, or paste credentials into the report. Prefer existing auth-setting IDs. Do not include bearer tokens, cookies, API keys, client secrets, or private certificate contents.
10. Do not mutate project configuration unless the user explicitly authorized the change. Before any mutation, show the planned objects and rollback path.
11. Do not poll in a tight loop. Use sensible pauses and bounded attempts.
12. When a command is absent, report the limitation and show the closest CLI-supported path. Do not fabricate a command.


## Workflow

### 1. Discover SLO operations

**There is exactly one SLO per project, not a list of named SLOs.** There is no `list-slos`, `get-slo`, `create-slo`, `update-slo` (top-level), or `delete-slo` — the real commands are `get-project-slo`, `update-project-slo` (also *creates* the project's SLO if none exists yet), and `delete-project-slo`. None of them take an SLO ID; scope with `--apimetrics-project-id` only if targeting a different project than the active one. As before, there is **no attainment, status, or error-budget endpoint** — the CLI returns the SLO *definition*, not computed attainment. Plan to derive attainment yourself from result and performance data (step 4).

```bash
apimetrics get-project-slo -o json    # bare single object, not {results:[...]}
```

The object has `include_tags`/`exclude_tags` (the monitor scope — there is no `scope`/`scope_id` field; scoping is tag-based only), `objectives[]`, and `thresholds[]`. An objective has `metric` (observed values include `availability`, `slow`, `dns`, `tcp`, `casc`, plus web-vitals like `largest_contentful_paint`/`cumulative_layout_shift` — confirm the current set with `apimetrics update-project-slo --help`, don't assume `total` or a generic "latency family"), `measure`, `comparator` (`<`/`>`), `value`, `unit` (`ms`/`percent`/`pp`/`value`), an undocumented `description`, and `period` (`PT5M`…`PT1H`, `DAY`, `WEEK`, `MONTH`). A threshold has `metric`, `period`, `unit`, `delta`, and `description`.

### 2. Define the review period

Use the user-specified period or the previous complete calendar month for formal review; use seven days for an operational pulse. State exact timestamps and timezone.

### 3. Inspect definitions

There is one SLO object per project. For each objective within it (and, separately, each threshold) capture:

- objective target and metric
- evaluation window (`period`)
- monitor scope (`include_tags`/`exclude_tags` — there is no per-objective monitor ID)
- comparator/value/unit and the `description` field
- ownership/tags on the scoped monitors themselves (the SLO object has no owner field)

Flag ambiguous or overly broad tag scope, conflicting windows across objectives, and objectives unsupported by the monitored journey.

### 4. Derive attainment evidence

Because the CLI exposes no attainment endpoint, reconstruct it from the monitors the SLO's `include_tags`/`exclude_tags` scope to:

- For availability/pass objectives, count `result_category` values over the objective `period` using `list-results`/`list-results-by-call` with `--from`/`--time` (not `--since`/`--before`, and not `list-call-results`, which doesn't exist).
- For latency objectives, use `query-api-performance` / `query-api-monitor-performance` with matching `metrics`/`measures` and window; align the query `interval` to the objective `period`. Confirm the objective's actual `metric` name (e.g. `slow`, `dns`, `tcp`, `casc`) maps to a real `query-*-performance` metric/measure before querying — don't assume it's called `total`.

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

Deliver a table with objective (metric/scope), attainment, target, budget status, cause, owner, action, and verification — one row per objective, since the project has a single SLO.

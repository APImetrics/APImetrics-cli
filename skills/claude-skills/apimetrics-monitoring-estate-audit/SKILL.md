---
name: apimetrics-monitoring-estate-audit
description: Audit an APImetrics project's monitoring coverage and hygiene — duplicates, orphans, schedules, auth references, assertions, locations, stale monitors, and missing critical journeys. Use for governance, cleanup, or onboarding reviews.
argument-hint: "[optional service, tag, or environment]"
---

# APImetrics monitoring estate audit

Perform a read-only audit by default. The deliverable is a prioritized cleanup and coverage plan, not automatic deletion.


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

### 1. Discover the estate

Run the available list/read commands (JSON output), noting each envelope shape:

- API calls — `list-calls` (`{meta, results}`)
- browser monitors — `list-browser-monitors` (bare `{results}`)
- MCP monitors — `list-mcp-monitors` (bare `{results}`)
- schedules — `list-schedules` (**`{data:[...]}`**), plus `list-schedule-downtimes` for maintenance windows
- SLOs — `list-slos` (bare `{results}`), detail via `get-slo <slo-id>`
- auth settings/tokens as metadata only — `list-auth-settings`, and `list-calls-by-auth <auth-id>` to size the blast radius of each shared credential
- recent results — `list-results --since <ISO>` for working-vs-configured coverage
- which schedules a call belongs to — `list-schedules-for-call <call-id>`

### 2. Normalize the inventory

For each monitor record:

- ID and name
- type
- target host/path or journey
- method/step
- environment and owner tags
- enabled state
- schedule IDs and frequency
- agents/locations
- auth reference
- assertion/conformance coverage
- SLO association
- last result time and recent outcome

Do not expose secret values.

### 3. Find hygiene risks

Check for:

- duplicate monitors targeting the same operation/journey
- orphan monitors with no schedule
- schedules with missing/deleted targets
- disabled or stale monitors still treated as coverage
- never-run monitors
- names without service/environment context
- missing tags or owners
- missing assertions or overly permissive success criteria
- production monitors with only one location
- excessive frequency or quota risk
- shared auth references creating a large blast radius
- monitors with persistent failure noise
- monitors that pass but no longer represent a critical journey
- unsupported or deprecated monitor patterns visible in current help

### 4. Assess coverage

Map the estate to:

- public availability/health
- authenticated read
- write/transaction path
- multi-step workflow
- browser user journey
- MCP protocol/tool behavior
- conformance/business rules
- regional/network coverage
- SLOs and alertable outcomes

Mark each critical service/journey as covered, partially covered, or missing. Explain the criterion.

### 5. Use weekly evidence

Sample the last seven days to distinguish configured coverage from working coverage. Flag monitors with no data, chronic failure, unstable locations, or unrepresentative schedules.

### 6. Deliver the audit

Provide:

- estate summary and coverage scorecard
- high-risk gaps
- cleanup candidates
- schedule/frequency findings
- auth and blast-radius observations
- assertion/conformance findings
- recommended additions
- phased action plan with owner and expected outcome
- exact IDs for all proposed changes

No mutation occurs unless separately authorized.

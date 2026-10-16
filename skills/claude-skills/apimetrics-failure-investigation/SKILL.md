---
name: apimetrics-failure-investigation
description: Investigate one failing APImetrics result or a repeated series, compare it with passing controls, identify the most likely underlying cause, and define a remediation and verification plan.
argument-hint: "[result ID, monitor ID, or monitor name]"
---

# APImetrics failure investigation

Find the failure boundary and the smallest evidence-backed remediation. Do not stop at the platform's top-level failure label.


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


## Inputs and scope

Accept any of:

- result ID
- monitor ID or name
- approximate incident time
- series of failures
- customer symptom

Resolve missing IDs through read-only list commands. State the active project and exact scope before analysis.

## Workflow

### 1. Retrieve the subject

If given a result ID, retrieve the result summary:

```bash
apimetrics get-result <result-id> -o json
```

**Know what this returns — it depends on your role.** Every caller gets `meta` and `timing`: `meta.result_category` (`PASS`/`SLOW`/`WARNING`/`FAIL` — HTTP 4xx lands in `WARNING`, connection errors in `FAIL`; empty while the run is queued), `meta.result` (the detail: `QUEUED` while in flight, then e.g. `COMPLETE`, `HTTP_CLIENT_ERROR`, `CONNECTION_ERROR`), `meta.http_code`, `meta.location_id`, `meta.call_id` (the monitor ID), `meta.start_time`, and `timing` (`dns`/`tcp`/`handshake`/`upload`/`processing`/`download`/`total`, ms). A below-ANALYST caller gets only that summary — no `request`, `response`, or `context`. Most project members (ANALYST or above) also get `request` (method, URL, headers), `response` (headers and body), `meta.dns_details`, and `meta.tls_version` by default. Add `--mode full` for the request body, `messages`/`log_messages`, and certificate detail, and `--header-style mapping` for name→value headers. An oversized response body is replaced by `"<< LARGE DATA >>"` with a `response.content_path`. Check `apimetrics get-result --help` and what actually comes back before assuming you need step 3's dedicated commands — but still don't claim a DNS or TLS cause from a single result alone.

For a series from one monitor of any type (API, browser, or MCP), use `list-results-by-call <monitor-id>`, whose flags are `--from`, `--time` (window end; both full ISO-8601 date-times, as a bare date is rejected), `--result-category`, `--cursor`, `--limit` (default 100, one page per call; repeat with `--cursor <meta.next_cursor>` while `meta.more` is true). `--result-category` takes `PASS`/`SLOW`/`WARNING`/`FAIL` and silently ignores any other value, returning every category:

```bash
apimetrics list-results-by-call <monitor-id> --from 2026-07-01T00:00:00Z -o json
```

For a project-wide sweep across all monitors, use `list-results --from ... --time ...`.

Also read the monitor configuration (`get-monitor` for any type, or `get-call`/`get-api-monitor`, `read-browser-monitor`, `read-mcp-monitor` — project the fields you need with `-f`, as `request.headers`/`request.body` can hold literal credentials), conditions (`get-call-conditions <call-id>`), schedule, auth-setting metadata, and related SLO where those commands exist.

### 2. Establish the timeline

Find:

- last known good result
- first known bad result
- latest bad result
- first recovery, if any
- whether failures are continuous, intermittent, or location-specific

Retrieve at least one passing control near the failure and one failure from another affected location when available.

### 3. Compare evidence dimensions

Compare failing and passing results across the dimensions below. Each names the command that actually exposes the data — the result summary alone is not enough.

| Dimension | Where it comes from |
|---|---|
| status / HTTP code / total response time / location | `get-result`, `list-results-by-call`, `list-results` |
| component timings (`dns`, `connect`, `tls`, `ttfb`, `response`) and percentiles | `query-api-monitor-performance` (per monitor) or `query-api-performance` (project), body `{from,to,metrics,measures,group_by}` |
| DNS provider / resolved IPs / CNAME chain / NS / mean lookup time | `query-api-monitor-dns-diagnostics` / `query-api-dns-diagnostics`, body `{from,to,group_by,locations}` |
| response body / headers | `get-result` (`response.content`, `response.headers`; `--mode full` adds the request body). For an oversized body (`"<< LARGE DATA >>"`), `get-result-content <result-id> <path>` is the retrieval endpoint — check its `--help`, and treat `Content not found` as not retrievable, not as an empty body |
| assertion / condition outcomes | `get-call-conditions <call-id>` plus `meta.result_category`/`meta.result`; `conformance-results` / `conformance-results-summary` for spec conformance |
| browser screenshot | `get-result-screenshot <result-id> > shot.jpg` (binary JPEG on stdout; browser results only) |
| MCP session step / tool response | the MCP result from `get-result` |

Example — is a slow/failing API monitor a DNS problem, and is it isolated to one location?

```bash
apimetrics query-api-monitor-dns-diagnostics <monitor-id> -o json <<'EOF'
{ "from": "2026-07-08T00:00:00Z", "to": "2026-07-15T00:00:00Z", "group_by": ["location"] }
EOF
```

Retrieve artifacts only when the command tree exposes them, and inspect `--help` first. Do not assert route/ASN/CDN causes unless a command in this build actually returns that field — the base result object does not.

### 4. Classify the failure

Use this evidence hierarchy:

1. **Monitor configuration** — malformed URL, method, body, assertion, schedule, or stale auth reference.
2. **Authentication/authorization** — 401/403, token refresh, scope, certificate, or clock issue.
3. **Application behavior** — deterministic 4xx/5xx, wrong payload, business-rule failure.
4. **Conformance/assertion** — transport succeeds but response violates expected status/schema/content.
5. **Rate limiting/quota** — 429, quota response, frequency-correlated failures.
6. **DNS/TLS/network/routing** — resolution/connect/handshake errors or location/route concentration.
7. **Agent/location issue** — one agent fails while equivalent agents pass and target evidence is otherwise healthy.
8. **Timeout/performance** — growing component latency or consistent threshold breach.
9. **Unknown** — evidence insufficient or conflicting.

For each candidate, list supporting and contradicting evidence.

### 5. Reproduce safely

Run an on-demand check only when authorized and when it will not create harmful traffic.

`run-monitor <monitor-id>` runs API, browser, and MCP monitors alike. It requires a body (at least `{}`) and returns the `result_id` to poll with `get-result` until `meta.result` is no longer `QUEUED`:
```bash
apimetrics run-monitor <monitor-id> <<'EOF'
{}
EOF
```

Use current help as authority. Do not edit the production monitor merely to test a theory. Prefer a duplicate or a reversible temporary change when isolation is required and authorized.

### 6. Recommend remediation

Provide:

- confirmed or most likely root cause
- confidence level
- impacted scope
- responsible system/team
- precise remediation
- monitor-side adjustment, if separately needed
- rollback
- verification run and success criteria
- evidence still needed if not confirmed

## Output template

### Finding
One sentence with confidence.

### Evidence
A compact timeline and pass/fail comparison.

### Cause analysis
Ranked hypotheses with supporting and contradicting evidence.

### Remediation
System fix first; monitor fix separately.

### Verification
Exact monitor/result commands and expected fields.

### Limitations
Missing artifacts, truncated history, or unavailable commands.

## Hard rules

- Do not call a target healthy merely because another location passes.
- Do not call a monitor broken merely because the target returned an unexpected response.
- Separate symptom, proximate failure, and underlying cause.
- Never recommend weakening assertions solely to turn a real failure green.

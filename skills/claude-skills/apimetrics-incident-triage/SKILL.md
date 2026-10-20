---
name: apimetrics-incident-triage
description: Triage a current or recent multi-monitor APImetrics incident, determine blast radius and common cause, build a timeline, and produce an operational incident brief.
argument-hint: "[incident time, service, host, or project]"
---

# APImetrics incident triage

Use project-wide evidence to distinguish an isolated monitor failure from a shared incident.


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

### 1. Define the incident window

Set exact start/end timestamps and timezone, as full ISO-8601 date-times (the server buckets in UTC). Default to a narrow window around the reported symptom, then expand only when necessary.

### 2. Inventory and collect failures

Collect the project-wide result stream for the window with server-side filtering, then segment:

```bash
apimetrics list-results --from <ISO> --time <ISO> -o json   # {meta, results:[...]}, one page per call
```

(`list-results` takes `--from` (start) and `--time` (end), plus `--result-category`/`--cursor`/`--limit` (default 100). Always pass `--from` — without it only a short recent window is searched. `--time` is truncated to the minute, so round the window end up. Repeat with `--cursor <meta.next_cursor>` while `meta.more` is true. `--result-category` takes `PASS`/`SLOW`/`WARNING`/`FAIL` and silently ignores any other value, returning every category; failures span both `FAIL` (e.g. connection errors) and `WARNING` (HTTP 4xx), so collect both.)

Each row is `{id, meta, timing}`: `meta.result_category`, `meta.result` (failure detail such as `HTTP_CLIENT_ERROR` or `CONNECTION_ERROR`), `meta.http_code`, `meta.location_id`, `meta.call_id` (monitor ID), `meta.start_time`, and `timing.total` (ms, with `dns`/`tcp`/`handshake`/`processing`/`download` components) — enough to segment by monitor, status, and location. For shared-cause hypotheses, use `query-api-dns-diagnostics` (DNS provider/IP/CNAME drift by location) and `query-api-performance` (latency shift by location/interval) over the incident window. Map `meta.call_id` back to names/types/hosts with `list-monitors` (every type; `{data:[...], meta}`, type in `meta.monitor_type`) or `list-calls`/`list-browser-monitors`/`list-mcp-monitors`, and to auth with `list-auth-settings`. Project these with `-f` to the fields you need — monitor `request.headers`/`request.body` and auth settings can hold literal credentials. Identify:

- affected monitors and unaffected controls
- target hosts/services
- agents/locations
- first failure, peak, and recovery
- failure signatures and status codes
- common schedules or authentication settings
- common route/CDN/origin when visible

### 3. Determine blast radius

Segment by:

- monitor type
- target host/service
- geography/agent
- authentication mechanism
- status/failure signature
- network route/CDN/origin
- customer/environment tags

Explicitly list unaffected controls; they are evidence.

### 4. Test common-cause hypotheses

Rank:

- target application outage or degradation
- shared identity/auth dependency
- DNS, certificate, CDN, route, or origin issue
- platform quota/rate limit
- schedule/config deployment
- APImetrics agent/location issue
- unrelated coincident monitor failures

Use result comparisons and on-demand checks only when safe and authorized.

### 5. Produce an incident brief

Include:

- severity recommendation and customer impact
- exact timeline
- blast radius
- current state
- leading cause and confidence
- evidence for/against alternatives
- immediate containment
- remediation owner
- verification conditions
- monitoring gaps exposed by the incident

Do not claim resolution until post-recovery results pass across the previously affected segments.

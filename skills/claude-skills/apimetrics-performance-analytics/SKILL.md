---
name: apimetrics-performance-analytics
description: Analyze APImetrics latency distribution, component timing breakdown (API: DNS/connect/TLS/TTFB/response; browser: Web Vitals plus DNS/TCP/TLS; MCP: initialize plus DNS/TCP/TLS), DNS diagnostics, and browser resource/session data using the server-side query-* commands. Use for latency deep-dives, "why is this slow", percentile trends, or DNS/CDN drift questions across API, browser, or MCP monitors.
argument-hint: "[monitor, service, or metric focus]"
---

# APImetrics performance analytics

Answer performance and latency questions with the platform's **server-computed** aggregate commands rather than by scraping raw result summaries. Result rows carry one run's `timing` (`total` and components); the `query-*` family returns server-computed percentiles, per-component aggregates, and DNS breakdowns over a window.

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
4. Commands are generally flat (`query-api-performance`).
5. The `query-*` commands read a JSON body from stdin. Use a quoted heredoc:
   ```bash
   apimetrics query-api-performance <<'EOF'
   { "from": "...", "to": "..." }
   EOF
   ```
6. Use `-o json` for analysis; it prints the response body. `-f` paths are rooted at the whole response, so they start with `body.` (e.g. `-f 'body.results'`). Performance query responses are `{meta, results:[...]}`, where `meta` echoes the requested `from`/`to`/`measures`/`metrics`/`group_by`/`interval` so you can confirm the CLI honored your request; DNS diagnostics' `meta` echoes only the window, `group_by`, and `interval`.
7. Use `-q key=value` only for query parameters confirmed by command help.
8. Preserve evidence. Record the active project, commands run, IDs, exact window, and the smallest response excerpts needed to support conclusions.
9. Never print, store, or paste credentials. Do not include bearer tokens, cookies, API keys, client secrets, or private certificate contents.
10. This skill is read-only. It never creates, updates, or deletes anything.
11. Do not poll in a tight loop.
12. When a command is absent, report the limitation and show the closest CLI-supported path. Do not fabricate a command.

## Command map

Discover exact names in the current build with `apimetrics --help`; the grounded set is:

| Focus | Project-wide | Single monitor |
|---|---|---|
| API latency / percentiles | `query-api-performance` | `query-api-monitor-performance <monitor-id>` |
| API DNS diagnostics | `query-api-dns-diagnostics` | `query-api-monitor-dns-diagnostics <monitor-id>` |
| Browser performance | `query-browser-performance` | `query-browser-monitor-performance <monitor-id>` |
| Browser resources | `query-browser-resources` | `query-browser-monitor-resources <monitor-id>` |
| Browser sessions (top-N raw rows) | `query-browser-sessions` | `query-browser-monitor-sessions <monitor-id>` |
| MCP performance | `query-mcp-performance` | `query-mcp-monitor-performance <monitor-id>` |

Baselines and org/project rollups also exist: `baseline-dailies`, `baseline-hourlies`, `project-calls-monthly`, `org-slow-calls-daily-csv`, `org-slow-calls-monthly`.

## Request body (performance queries)

```json
{
  "from": "2026-07-08T00:00:00Z",
  "to": "2026-07-15T00:00:00Z",
  "metrics": ["total", "dns", "connect", "tls", "ttfb", "response"],
  "measures": ["mean", "p50", "p95", "p99"],
  "group_by": ["monitor", "location"],
  "interval": "PT1H",
  "locations": ["<agent-id>"],
  "status": ["PASS", "SLOW", "WARNING", "FAIL"]
}
```

- `from`/`to` are required ISO-8601 timestamps.
- `metrics` differ by monitor type (omitted, API defaults to `total`, `dns`, `ttfb`, `response`):
  - API: `total`, `dns`, `connect`, `tls`, `ttfb`, `response`, `upload`, `network`, `server`, `total_no_dns`.
  - Browser: `lcp`, `fcp`, `cls`, `fid`, `ttfb`, `total`, `dns`, `tcp`, `tls`, `request`, `response`, `dom_processing`, `dom_content_loaded`, `load_event_end`, `resources_count`, `page_weight`, `network`, `total_no_dns`.
  - MCP: `total`, `initialize`, `dns`, `tcp`, `tls`, `network`, `total_no_dns`.
  A name from another type's list is rejected with 422; confirm with the command's `--help`.
- `measures`: `mean`, `p50`, `p90`, `p95`, `p99` (all five when omitted). Responses also carry `p75`, `p999`, `sample_count`, and `pass_count`.
- `group_by`: `location`, `interval`, `monitor`. `group_by: ["interval"]` requires `interval` (ISO-8601 duration, e.g. `PT15M`, `PT1H`, `P1D`); setting `interval` buckets results by time either way. Buckets are UTC.
- `status` (`PASS`/`SLOW`/`WARNING`/`FAIL`) filters which result categories feed the aggregate. Omitted, every category is included, failed runs too; use `["PASS"]` (or `["PASS", "SLOW"]`) for the latency of successful runs.
- `locations` restricts to specific agents.

DNS diagnostics take the same window/`group_by`/`locations`/`status`/`interval` shape and return observed DNS providers, resolved IPs, CNAME chains, NS servers, FQDNs, registrars, TLDs, and mean lookup time. Project-wide `query-api-dns-diagnostics` groups only by `location`/`interval` — `monitor` is accepted but ignored — so use `query-api-monitor-dns-diagnostics <monitor-id>` per monitor.

Browser resource queries (`*-resources`) take only `from`, `to`, and `interval`, and return `{meta, results, third_party_timeseries}`. Browser session queries (`*-sessions`) take `from`, `to`, `limit` (1–200, default 10), `order` (`asc`/`desc`), and `sort_by` (e.g. `lcp`, `total`, `page_weight`, `timestamp`), and return `{meta, results}` rows that include `result_id`.

## Workflow

### 1. Frame the question
State the metric of interest (availability is a different skill — this is latency/timing/DNS), the monitors or tags in scope, the exact window, and the comparison window if any.

### 2. Choose scope
Project-wide first to spot the outliers (`group_by: ["monitor"]` on the performance queries), then drill into a single monitor with the `*-monitor-*` variant and `group_by: ["location","interval"]`.

### 3. Query and read
Run the query, confirm `meta` echoes your window/measures, and read `results`. Each result carries a `key` (the group_by dimension values) and the requested measures per metric.

### 4. Interpret
- Split API `total` into components (`dns` + `connect` + `tls` + `ttfb` + `response`) to locate where latency accrues. A failed run counts in a group's `sample_count` but may have no value for a metric.
- Compare `p95`/`p99` against `mean` to detect tail latency and jitter.
- Compare across `location` groups to separate a regional/agent effect from a global one.
- For DNS drift, compare resolved IPs / providers / lookup time across locations and time buckets.
- Run the same query over the prior window for a regression delta.

### 5. Report
- exact window(s), scope, and command bodies used
- a per-monitor or per-location table of the requested measures
- component-timing breakdown for the worst offenders
- DNS findings when relevant (provider, IPs, CNAME, lookup time)
- regression vs the comparison window with both numbers
- whether empty results mean "healthy" or "no data" — never conflate them

## Quality gates
- Every percentile is server-computed by a `query-*` command, never estimated from result summaries.
- Every comparison names both windows and both values.
- Empty result sets are reported as "no data in window", not as a passing metric.
- Latency claims name the component metric (`dns`/`tls`/`ttfb`/`response`), not just `total`.

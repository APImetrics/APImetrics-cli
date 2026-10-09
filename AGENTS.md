# APImetrics CLI — agent guide

Instructions for AI coding agents (OpenAI Codex, Cursor, Gemini CLI, and any tool that reads `AGENTS.md`) operating the `apimetrics` CLI in this repository. Claude Code users get the same guidance as installable skills — see [Skill parity](#skill-parity).

## Prerequisites

1. `apimetrics project show` — confirm a project is active. Every project-scoped command fails without one unless you pass `--project-id <id>`. If none is set, run `apimetrics project select`.
2. Commands log in automatically when needed; `apimetrics login` does it up front (OAuth). For headless use, pass `--service-account <file>`.

## Core facts about this CLI

- **Commands are flat, generated from the platform's live OpenAPI description**: `list-calls`, `create-call`, `run-monitor`. The set can change with the server spec, so treat `apimetrics --help` and `apimetrics <command> --help` as authoritative and inspect a command's help before assuming an option or body field. `run-monitor <id>` is the single command that starts a run for any monitor type — API call, browser, or MCP; it requires a body, so send at least `{}` on stdin, and returns the `result_id` to poll.
- **Deprecated commands still run but warn.** Where the server's spec deprecates a command, the CLI hides it from `--help` and prints `Command "X" is deprecated, do not use` on stderr (stdout JSON is unaffected). On servers that expose the `/api/3/monitors/` commands, `list-calls`/`create-call`/`get-call`/`update-call`/`delete-call` are deprecated in favour of `list-api-monitors` (or `list-monitors` for every type), `create-api-monitor`, `get-api-monitor`, `patch-api-monitor`/`update-api-monitor`, and `delete-api-monitor`. The new create/update commands take a flat body (`name`, `method`, `url`, `headers`, …) rather than `meta`/`request`; monitor IDs are the same, so `set-call-conditions`, `get-call-passfail-*`, and `list-results-by-call` keep working. Use whichever set your `--help` lists.
- **Create/update commands read their JSON body from stdin** via heredoc:
  ```bash
  apimetrics create-call <<'EOF'
  { "meta": { "name": "Health" }, "request": { "method": "GET", "url": "https://example.com/health" } }
  EOF
  ```
  CLI Shorthand is also accepted. Quote any value containing `//`, or the shorthand parser rejects it, and redirect stdin from `/dev/null` when running non-interactively — shorthand also reads stdin to merge with the arguments, so an open stdin pipe blocks: `apimetrics create-call meta.name: Health, request.method: GET, request.url: '"https://example.com/health"' </dev/null`.
- **Output**: `-o json` for machine use; it prints the response body. `-f` projects with a shorthand query rooted at the whole response (`status`, `headers`, `body`), so every path starts with `body.` — single-quote it so the shell doesn't glob `[]`: `-f 'body.results[].meta.name'`. A path that matches nothing prints nothing and exits 0. `-q key=value` appends a raw, unvalidated query parameter; prefer the command's own `--option` flags. The command spec is cached ~24h and refreshes automatically; `--rsh-no-cache` forces a refresh.
- **List envelopes are not uniform.** `list-calls`, `list-api-monitors`, `list-results`, `list-results-by-call`, and `list-auth-settings` return `{ "meta": ..., "results": [...] }`; `list-schedules` and `list-monitors` return `{ "data": [...], "meta": ... }`; `list-browser-monitors` and `list-mcp-monitors` return a bare `{ "results": [...] }`. Paged lists return one page per call — repeat with `--cursor <meta.next_cursor>` while `meta.more` is true. Inspect each command's own output before writing an `-f` path.
- **Result rows** (`list-results`, `list-results-by-call`) are `{ id, meta, timing }`: `meta.result_category`, `meta.result`, `meta.http_code`, `meta.location_id`, `meta.call_id` (the monitor ID), `meta.start_time`, and `timing.total` in ms with `dns`/`tcp`/`handshake`/`processing`/`download`/`upload` components. `result_category` ∈ `PASS`/`SLOW`/`WARNING`/`FAIL` (HTTP 4xx lands in `WARNING`, connection errors in `FAIL`) and is empty while the run is queued; `result` carries the detail — `QUEUED` while in flight, then e.g. `COMPLETE`, `HTTP_CLIENT_ERROR`, `CONNECTION_ERROR`. `--result-category` takes those four values case-insensitively and silently ignores any other value, returning every category. `--from`/`--time` are the window start and end as full ISO-8601 date-times (a bare date is rejected) and are minute-granular; always pass `--from`, as the default window is short.
- **`get-result` detail depends on caller role.** A below-ANALYST caller gets a summary only — `meta` and `timing` as in a result row, with no `request`, `response`, or `context`. An ANALYST-or-above caller (most project members) gets the full object by default — response headers and body, request method/URL/headers, `meta.dns_details`, `meta.tls_version`, and the `timing` breakdown. Add `--mode full` for the request body, `messages`/`log_messages`, and certificate detail. An oversized response body is replaced by `"<< LARGE DATA >>"` with a `response.content_path`. Check `apimetrics get-result --help` and what actually comes back before reaching for `get-result-content`/`get-result-screenshot`/`query-*` commands.
- **Percentiles are server-computed** by the `query-*-performance` commands (`measures`: `mean`/`p50`/`p90`/`p95`/`p99`; API `metrics`: `total`/`dns`/`connect`/`tls`/`ttfb`/`response`/`upload`/`network`/`server`/`total_no_dns`; browser and MCP metric names differ — check `--help`). Without a `status` filter, failed runs are included; pass `"status": ["PASS"]` for the latency of successful runs. Do not hand-average result rows.
- **API assertions** are set with `set-call-conditions <call-id>` (read with `get-call-conditions`), as a separate step after `create-call`.
- **SLOs are one per project.** Use `get-project-slo`/`update-project-slo`/`delete-project-slo` (update-project-slo creates one if none exists); scope objectives with `include_tags`/`exclude_tags`. `get-project-slo` returns the definition — or platform defaults, with `created`/`last_update` null, when none is stored; measure attainment with the server-side aggregates — `get-call-passfail-range`/`get-call-passfail-total` per scoped monitor for availability (sum the per-category `stat.<CATEGORY>.total` values from `get-call-passfail-total`; its top-level `stat.total` counts only uncategorised results), `query-*-performance` for latency.
- **Bulk config-as-code** lives under the hidden `apimetrics bulk` group (`init <url>`, `list`, `status`, `diff [--remote]`, `pull`, `reset`, `push`). `init` needs a real, resolvable URL — a full host+path against the CLI's configured API server (e.g. `qc-client.apimetrics.io/api/2/calls/`). The checkout does not record its project, so pass the same `--project-id` (or keep the same selected project) on every `bulk` command. Files hold the full GET representation, which `PUT /api/2/calls/{id}` rejects (read-only `id`/`meta.*` fields), so reduce a changed file to just the writable fields being changed before `bulk push` — that endpoint applies partial updates. `bulk push` prints `Push complete.` and exits 0 even when an upload fails, and does not check for remote drift: treat any `Error uploading` line as a failure, and confirm `bulk status` is clean afterwards.
- **Never print secrets.** Reference auth settings by ID (`auth_id`/`token_id`); never paste tokens, cookies, keys, or certificate contents. `list-auth-settings` returns credential values in clear text, and monitor `request.headers`/`request.body` can hold literal tokens, so project those lists with `-f` to the fields you need rather than printing full JSON — e.g. `-f 'body.results.{id, name: meta.name, auth_type: settings.auth_type}'`.

## Workflows (skills)

Each entry names when to reach for it and the key commands. Full step-by-step workflows — including safety gates, evidence rules, and output templates — are in `skills/claude-skills/<name>/SKILL.md`; read that file before executing the workflow. Run `apimetrics onboard` to print the three monitor-setup guides and all eight workflows to stdout for context injection.

| Workflow | Use when | Key commands |
|---|---|---|
| **project-bootstrap** | setting up monitoring / onboarding a service | `project select`, `create-project-in-org` + `create-project-access`, `create-call`/`create-browser-monitor`/`create-mcp-monitor`, `set-call-conditions`, `create-schedule`, `add-call-to-schedule`, `run-monitor` |
| **weekly-health-review** | reliability report for the last 7 complete days | `get-call-passfail-range`, `list-results --from --time`, `query-api-performance`, `list-calls` |
| **failure-investigation** | root-causing one failing result or a series | `get-result`, `list-results-by-call --from --result-category`, `get-result-content`, `query-api-monitor-dns-diagnostics`, `get-call-conditions` |
| **incident-triage** | blast radius / common cause across many monitors | `list-results --from --time`, `query-api-dns-diagnostics`, `query-api-performance` |
| **monitoring-estate-audit** | coverage & hygiene governance review | `list-calls`, `list-schedules`, `get-project-slo`, `list-auth-settings`, `list-calls-by-auth` |
| **slo-review** | SLO design & attainment (one SLO per project; attainment from server-side aggregates) | `get-project-slo`, `update-project-slo`, `get-call-passfail-range`, `query-api-performance` |
| **config-as-code** | reviewed bulk edits with rollback | `bulk init <resolvable-url>`, `bulk status`, `bulk diff`, `bulk push`, `bulk reset` |
| **performance-analytics** | latency percentiles, component timing, DNS drift | `query-api-performance`, `query-api-monitor-performance`, `query-api-dns-diagnostics`, `query-browser-performance`, `query-mcp-performance` |

## Safety

- Mutating workflows (`project-bootstrap`, `config-as-code`) require explicit user intent, a pre-change snapshot, and a stated rollback path. Never `bulk push` from a dirty or drifted checkout.
- Read-only analysis workflows make no changes.
- Every reported percentage carries a numerator and denominator; every root-cause statement carries confidence and evidence.
- Never weaken an assertion just to turn a real failure green.

## Skill parity

The eight workflows above are shipped to Claude Code as native skills:

```bash
apimetrics skills install --claude-skills   # writes .claude/skills/<name>/SKILL.md
```

`apimetrics onboard` prints the same eight `SKILL.md` files that `skills install --claude-skills` installs, and this `AGENTS.md` indexes them, so Claude Code and other agents work from the same workflows.

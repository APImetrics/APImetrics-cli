# APImetrics CLI — agent guide

Instructions for AI coding agents (OpenAI Codex, Cursor, Gemini CLI, and any tool that reads `AGENTS.md`) operating the `apimetrics` CLI in this repository. Claude Code users get the same guidance as installable skills — see [Skill parity](#skill-parity).

## Prerequisites

1. `apimetrics project show` — confirm a project is active. Every project-scoped command fails without one. If none is set, run `apimetrics project select`.
2. `apimetrics login` handles authentication (OAuth).

## Core facts about this CLI

- **Commands are flat, generated from the platform's live OpenAPI description**: `list-calls`, `create-call`, `run-monitor`. The set can change with the server spec, so treat `apimetrics --help` and `apimetrics <command> --help` as authoritative and inspect a command's help before assuming an option or body field. `run-monitor <id>` is the single command that starts a run for any monitor type — API call, browser, or MCP.
- **Create/update commands read their JSON body from stdin** via heredoc:
  ```bash
  apimetrics create-call <<'EOF'
  { "meta": { "name": "Health" }, "request": { "method": "GET", "url": "https://example.com/health" } }
  EOF
  ```
  CLI Shorthand is also accepted (`apimetrics create-call meta.name: "Health", request.method: GET, request.url: https://example.com/health`).
- **Output**: `-o json` for machine use; `-f` projects with a shorthand query; `-q` adds confirmed query params. The command spec is cached ~24h and refreshes automatically; `--rsh-no-cache` forces a refresh.
- **List envelopes are not uniform.** `list-calls`, `list-results`, `list-results-by-call`, and `list-auth-settings` return `{ "meta": ..., "results": [...] }`; `list-schedules` returns `{ "data": [...] }`; `list-browser-monitors` and `list-mcp-monitors` return a bare `{ "results": [...] }`. Inspect each command's own output before writing an `-f` path.
- **`get-result` detail depends on caller role.** A below-ANALYST caller gets a summary only (`result_category`, `http_code`, `response_time` ms, `location_id`, `test`, `created`; `result_category` ∈ `PASS`/`FAIL`/`WARN`/`ERROR`/`TIMEOUT`/`QUEUED` — a separate `result` field carries a transport-completion value like `COMPLETE`, not the pass/fail outcome). An ANALYST-or-above caller (most project members) gets the full object by default — request/response bodies and headers, DNS/TLS detail, and `timing` component breakdown included. Don't assume a summary-only response; check `apimetrics get-result --help` and what actually comes back before reaching for `get-result-content`/`get-result-screenshot`/`query-*` commands.
- **Percentiles are server-computed** by the `query-*-performance` commands (`measures`: `mean`/`p50`/`p90`/`p95`/`p99`; API `metrics`: `total`/`dns`/`connect`/`tls`/`ttfb`/`response`). Do not hand-average result summaries.
- **API assertions** are set with `set-call-conditions <call-id>` (read with `get-call-conditions`), as a separate step after `create-call`.
- **SLOs are one per project.** Use `get-project-slo`/`update-project-slo`/`delete-project-slo` (update-project-slo creates one if none exists); scope objectives with `include_tags`/`exclude_tags`. `get-project-slo` returns the definition; measure attainment with the server-side aggregates — `get-call-passfail-range`/`get-call-passfail-total` per scoped monitor for availability, `query-*-performance` for latency.
- **Bulk config-as-code** lives under the hidden `apimetrics bulk` group (`init <url>`, `status`, `diff [--remote]`, `pull`, `reset`, `push`). `init` needs a real, resolvable URL — a full host+path against the CLI's configured API server (e.g. `qc-client.apimetrics.io/api/2/calls/`).
- **Never print secrets.** Reference auth settings by ID (`auth_id`/`token_id`); never paste tokens, cookies, keys, or certificate contents.

## Workflows (skills)

Each entry names when to reach for it and the key commands. Full step-by-step workflows — including safety gates, evidence rules, and output templates — are in `skills/claude-skills/<name>/SKILL.md`; read that file before executing the workflow. Run `apimetrics onboard` to print every workflow to stdout for context injection.

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

The same content backs this `AGENTS.md` and `apimetrics onboard`, so Claude Code and other agents operate from one source of truth.

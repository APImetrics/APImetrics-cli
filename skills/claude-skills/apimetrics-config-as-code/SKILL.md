---
name: apimetrics-config-as-code
description: Check out APImetrics resources with the CLI bulk workflow, review and diff JSON changes, detect remote drift, and safely push authorized updates with a rollback path.
argument-hint: "[resource collection or change goal]"
disable-model-invocation: true
---

# APImetrics configuration as code

Use the CLI's built-in hidden `bulk` workflow for local JSON checkout and synchronization. Treat `bulk push` as a production mutation requiring explicit approval.


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


## Important capability

The CLI includes:

```bash
apimetrics bulk init
apimetrics bulk list
apimetrics bulk status
apimetrics bulk diff
apimetrics bulk pull
apimetrics bulk reset
apimetrics bulk push
```

Confirm with:

```bash
apimetrics bulk --help
```

The bulk workflow keeps metadata and resource versions, detects local and remote changes with `status` and `diff --remote`, and does not overwrite local edits during pull. `bulk push` does not check for remote drift (it sends no conditional request), and it prints `Push complete.` with exit status 0 even when individual uploads fail.

## Workflow

### 1. Establish scope and safety

Confirm:

- active project
- resource collection
- isolated working directory
- intended change
- approval boundary
- rollback owner

Do not initialize into a directory containing unrelated JSON files.

A checkout writes each resource verbatim to the working directory and `.rshbulk/`, including any literal credentials in request headers or bodies. Never check out auth settings with `bulk`. Keep the checkout directory out of version control and delete it when done.

The checkout does not record which project it belongs to. Pass the same `--project-id <id>` (or keep the same selected project) on **every** `bulk` command — `init`, `list`, `status`, `diff`, `pull`, `reset`, `push`. Against a different project, `status` reports every file as remote-removed and that project's resources as remote-added, and `pull` replaces the checkout.

### 2. Initialize the checkout

`bulk init` takes exactly one URL argument that returns a list of resources, each with a link and a version. Use a real, resolvable URL — a full host+path against the CLI's configured API server (check the host with `apimetrics --version`) and the real resource path, shown on the `GET` line of `apimetrics <list-command> -v` (help text does not include it). Use the `/api/2/calls/` collection, whose `PUT` applies partial updates:

```bash
apimetrics bulk init 'qc-client.apimetrics.io/api/2/calls/?limit=1000' -f 'body.results.{id, version: meta.last_update}' --url-template='/api/2/calls/{id}' --project-id <id>
```

`init` checks out one page only, so set `?limit=` on the URL (the list defaults to 100; `1000` is the maximum). Split a larger project into several checkouts scoped with a `-f` `[?...]` filter. `init` fails on a list of exactly one resource (`slice bounds out of range`); widen the list, or make that single change with the monitor's own update command. If `init` fails, delete `.rshbulk/` before retrying.

`init` auto-detects the resource URL from `url`/`uri`/`self`/`link` and the version from `version`/`etag`/`last_modified`/`lastModified`/`modified`. The `list-calls` response doesn't expose a direct link, so the example above shapes it with `-f` down to `id`/`version` and builds the link from the ID with `--url-template`.

Always confirm the exact flags first:

```bash
apimetrics bulk init --help
```

`bulk` is a hidden command group — it will not appear in the top-level `--help` list, but `apimetrics bulk --help` is authoritative.

### 3. Baseline

Run:

```bash
apimetrics bulk status
apimetrics bulk list
```

Read the metadata and checked-out JSON. If remote changes already exist, pull or reconcile first. Then record a clean baseline by copying the checkout aside before editing (e.g. `cp -R . ../<name>-baseline`). `.rshbulk/` is overwritten by `pull` and by each successful `push`, so it is not a rollback source.

### 4. Edit and validate

Make only the authorized JSON changes. Each file holds the full `GET` representation, and `PUT /api/2/calls/{id}` rejects its read-only fields (`id`, `meta.created`, `meta.last_update`, `meta.domain`, `meta.owner`, `meta.project_id`, `meta.monitor_type`, `meta.accept`, `meta.content_type`) with `extra_forbidden`. Because that `PUT` is a partial update, reduce each changed file to just the writable fields being changed — e.g. `{"meta": {"description": "…"}}` — and keep the full original in the baseline copy for review. After a successful push the CLI re-fetches the full resource into the file.

Run:

```bash
apimetrics bulk status
apimetrics bulk diff
```

Review every changed, added, and removed resource against the baseline copy. `--match` resolves top-level fields only (e.g. `id contains …`); a nested path such as `meta.name` is rejected with a warning and matches nothing. Test any expression against `bulk list` before using it with `diff` or `reset`, or pass explicit file names.

### 5. Reconcile remote drift

Immediately before push:

```bash
apimetrics bulk status
apimetrics bulk diff --remote
```

If remote changes exist, stop and reconcile. `bulk pull` does not overwrite local changes, but conflicts still require deliberate review.

### 6. Approval gate

Present:

- exact resources and IDs
- unified diff
- dependencies/references affected
- expected operational effect
- rollback plan
- post-change verification

Run `apimetrics bulk push` only after explicit approval in the current conversation.

### 7. Verify

After push:

- treat any `Error uploading …` line as a failed push, whatever the final message and exit status say
- run `bulk status` and require `No local changes` and a clean remote state
- read changed resources through normal CLI commands
- run affected monitors on demand
- verify schedule targets and successful results
- retain the before/after diff in the handoff

### 8. Rollback

For unpushed local edits:

```bash
apimetrics bulk reset [file...]
```

For pushed changes, write the baseline values of the changed writable fields (from the baseline copy) into the file as a partial body, show the diff, and push only with fresh approval. Pushing a full baseline file fails on the read-only fields. Do not treat `reset` as a remote rollback.

## Hard rules

- Never push from a dirty or drifted baseline.
- Never bulk-edit auth secret values.
- Never delete resources solely because files appear stale; verify references and recent results.
- Never use bulk push as a discovery tool.

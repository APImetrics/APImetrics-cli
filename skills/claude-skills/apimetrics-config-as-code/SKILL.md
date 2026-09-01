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

The bulk workflow keeps metadata and resource versions, detects local and remote changes, and can avoid overwriting local edits during pull.

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

### 2. Initialize the checkout

`bulk init` takes exactly one URL argument that returns a list of resources, each with a link and a version. Use the CLI's own API-name scheme (`apimetrics:/<collection>`) rather than a raw host — the repository's own example is:

```bash
apimetrics bulk init apimetrics:/monitors
```

`init` auto-detects the resource URL from `url`/`uri`/`self`/`link` and the version from `version`/`etag`/`last_modified`/`lastModified`/`modified`. If the collection response doesn't expose those directly, shape it with `-f` and/or build links from IDs with `--url-template`:

```bash
apimetrics bulk init apimetrics:/monitors -f 'body.{url, version: last_update}'
apimetrics bulk init apimetrics:/monitors -f 'body.{id, version: last_update}' --url-template='/monitors/{id}'
```

Always confirm the exact flags first:

```bash
apimetrics bulk init --help
```

Do not guess an undocumented host path; prefer the `apimetrics:` scheme, which resolves against the CLI's configured server. `bulk` is a hidden command group — it will not appear in the top-level `--help` list, but `apimetrics bulk --help` is authoritative.

### 3. Baseline

Run:

```bash
apimetrics bulk status
apimetrics bulk list
```

Read the metadata and checked-out JSON. Record a clean baseline before editing. If remote changes already exist, pull or reconcile first.

### 4. Edit and validate

Make only the authorized JSON changes. Preserve IDs, versions, references, and unrelated fields.

Run:

```bash
apimetrics bulk status
apimetrics bulk diff
```

Review every changed, added, and removed resource. Use `--match` only after testing the expression against `bulk list`.

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

- run `bulk status` and require clean local/remote state
- read changed resources through normal CLI commands
- run affected monitors on demand
- verify schedule targets and successful results
- retain the before/after diff in the handoff

### 8. Rollback

For unpushed local edits:

```bash
apimetrics bulk reset [file...]
```

For pushed changes, restore the reviewed baseline JSON and push only with fresh approval. Do not treat `reset` as a remote rollback.

## Hard rules

- Never push from a dirty or drifted baseline.
- Never bulk-edit auth secret values.
- Never delete resources solely because files appear stale; verify references and recent results.
- Never use bulk push as a discovery tool.

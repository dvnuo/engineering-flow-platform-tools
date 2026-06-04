# Log CLI

`log` is a cross-platform, local-only CLI for agent-friendly large log analysis. It is designed for files that are too large to paste or `cat` into an agent context.

## Design Goals

- Stream local files, directories, and globs without loading the whole source log into memory.
- Produce stable JSON envelopes for every command when `--json` is used.
- Store only bounded, redacted analysis artifacts in a run directory.
- Let agents move from overview to evidence: `analyze`, then `profile`, `template list`, `group`, `timeline`, `search`, `window`, `extract`, `summarize`, and `export evidence`.
- Never call an LLM and never connect to remote log backends in P0.

## Why Not Cat Huge Logs

Huge logs can exceed context limits, bury the useful evidence, and leak credentials. Agents should not paste whole logs into prompts. Run `log analyze` first, then use bounded search and window commands to retrieve only the relevant redacted evidence.

## Run Directory Layout

```text
<run-dir>/
  manifest.json
  entries.jsonl
  templates.json
```

`manifest.json` records source paths, counts, time range, tool version, and whether ingest was truncated by `--max-bytes`.

`entries.jsonl` stores one redacted preview per parsed event plus source path, line range, byte offsets, level, timestamp, template id, variables, and tags.

`templates.json` stores redacted templates, counts, representative entry ids, examples, levels, first/last seen timestamps, and classification tags.

The run directory does not contain a raw copy of the source log. `log window` reads the original source file by path and redacts each returned line. If the source file is gone, it returns `source_missing`.

## Commands

```bash
log version --json
log commands --json
log schema analyze --json
log help llm
log doctor --json
log run list --json
log run get ./.log-runs/run_001 --json
log run verify ./.log-runs/run_001 --json
log run delete ./.log-runs/run_001 --yes --json
log analyze --source ./logs/app.log --run ./.log-runs/run_001 --json
log analyze --source ./logs/app.log --dry-run --json
log profile --run ./.log-runs/run_001 --json
log template list ./.log-runs/run_001 --only non-info --sort count --json
log template get ./.log-runs/run_001 --template tpl_abcdef1234567890 --json
log template entries ./.log-runs/run_001 --template tpl_abcdef1234567890 --limit 20 --json
log template variables ./.log-runs/run_001 --template tpl_abcdef1234567890 --json
log templates --run ./.log-runs/run_001 --only non-info --sort count --json
log entries --run ./.log-runs/run_001 --limit 20 --json
log search ./.log-runs/run_001 --query 'ERROR OR timeout' --service api --json
log window --run ./.log-runs/run_001 --entry-id entry_000001 --before 50 --after 50 --json
log extract --run ./.log-runs/run_001 --kind stacktrace --json
log extract --run ./.log-runs/run_001 --kind error_signature --json
log group ./.log-runs/run_001 --by error_signature --json
log timeline ./.log-runs/run_001 --bucket 1m --json
log summarize ./.log-runs/run_001 --focus "dominant failures" --json
log export evidence ./.log-runs/run_001 --evidence entry_000001 --format markdown --output ./evidence.md --dry-run --json
```

## JSON Envelope Examples

Success:

```json
{
  "ok": true,
  "data": {
    "run_id": "run_001",
    "entries_count": 900,
    "templates_count": 40,
    "truncated": false
  }
}
```

Failure:

```json
{
  "ok": false,
  "error": {
    "code": "run_not_found",
    "message": "Analysis run directory was not found.",
    "hint": "Run log analyze --source <path> --run <run-dir> --json first.",
    "status": 404
  }
}
```

## Redaction Policy

The CLI redacts secrets before writing run files and before printing output. Redaction covers bearer authorization headers, standalone bearer tokens, password/token/api key/secret assignments, AWS access key variables, private key blocks, and email addresses.

This applies to:

- `entries.jsonl`
- `templates.json`
- `log search`
- `log window`
- `log extract`
- `log summarize`
- `log export evidence`
- JSON failures and verbose diagnostics

If a value might be secret, prefer redaction over preserving the original text.

## Known Limitations

- P0 only supports local files, directories, and globs.
- P0 search scans the redacted `entries.jsonl` run artifact and returns bounded results; it is not yet a full-text or columnar indexed backend.
- P0 summaries are deterministic aggregations over profile/templates/groups/extracts and do not call an LLM.
- P0 evidence export writes redacted entry/template evidence only; it does not export raw source logs.
- For TB-scale persistent analytics, add an indexed store/backend in a later PR.
- `log window` only reads source files and line ranges recorded during `log analyze`; append-only changes require re-running `log analyze` before direct line windows can inspect newly added lines.
- No Loki, ClickHouse, Elasticsearch, Kubernetes, Docker, `journalctl`, or other remote backends.
- No real-time tailing or service mode.
- No LLM summarization.
- No HTML report.
- Source files must remain available for `log window`.

# Log CLI

`log` is a cross-platform, local-only CLI for agent-friendly large log analysis. It is designed for files that are too large to paste or `cat` into an agent context.

## Design Goals

- Stream local files, directories, and globs without loading the whole source log into memory.
- Produce stable JSON envelopes for every command when `--json` is used.
- Store only bounded, redacted analysis artifacts in a run directory.
- Let agents move from overview to evidence: `analyze`, then `profile`, `templates`, `search`, `window`, and `extract`.
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
log analyze --source ./logs/app.log --run ./.log-runs/run_001 --json
log profile --run ./.log-runs/run_001 --json
log templates --run ./.log-runs/run_001 --only non-info --sort count --json
log entries --run ./.log-runs/run_001 --limit 20 --json
log search --run ./.log-runs/run_001 --query 'ERROR OR timeout' --json
log window --run ./.log-runs/run_001 --entry-id entry_000001 --before 50 --after 50 --json
log extract --run ./.log-runs/run_001 --kind stacktrace --json
log extract --run ./.log-runs/run_001 --kind error-signature --json
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
- JSON failures and verbose diagnostics

If a value might be secret, prefer redaction over preserving the original text.

## Known Limitations

- P0 only supports local files, directories, and globs.
- No Loki, ClickHouse, Elasticsearch, Kubernetes, Docker, `journalctl`, or other remote backends.
- No real-time tailing or service mode.
- No LLM summarization.
- No HTML report.
- Source files must remain available for `log window`.

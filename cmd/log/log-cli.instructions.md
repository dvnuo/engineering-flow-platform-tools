# Log CLI Instructions for Agents

Use the local `log` CLI for large local logs that are too big to paste, `cat`, `tail`, or scan with unbounded `grep`.

## Ground Rules

- Always use `--json` on `log` commands so responses use the stable `ok/data/error` envelope.
- Do not use `cat`, `tail`, `less`, or unbounded `grep` on large logs.
- Start with `log analyze --source <path> --run <run-dir> --json`.
- Then use `log profile`, `log templates`, `log search`, `log window`, and `log extract` against that run directory.
- Treat `entry_id`, `template_id`, and `evidence_ref` as handles for follow-up evidence.
- Prefer `log window --run <run-dir> --entry-id <id> --json` for source context.
- Use `log window --file <path> --line <line>` only for files already present in that run's `manifest.json`.
- Run artifacts are redacted previews and indexes, not raw source logs.
- Keep original source files available; `log window` reads them by path and returns `source_missing` if they are gone.

## Scope

- P0 supports local files, directories, and globs only.
- Do not use MCP for this CLI, and do not configure or start an MCP server for log analysis.
- Do not call remote logging backends such as Loki, ClickHouse, Elasticsearch, Kubernetes, Docker, or `journalctl`.
- Do not use real-time tailing.
- Do not ask the CLI to summarize raw logs with an LLM.

## Typical Workflow

```bash
log analyze --source ./logs/app.log --run ./.log-runs/run_001 --json
log profile --run ./.log-runs/run_001 --json
log templates --run ./.log-runs/run_001 --only non-info --sort count --limit 20 --json
log search --run ./.log-runs/run_001 --query "ERROR OR timeout" --limit 20 --json
log window --run ./.log-runs/run_001 --entry-id entry_000001 --before 50 --after 50 --json
log extract --run ./.log-runs/run_001 --kind stacktrace --limit 20 --json
log extract --run ./.log-runs/run_001 --kind error-signature --limit 20 --json
```

Inspect `error.code` and `error.hint` before retrying. If `search` returns `next_cursor`, pass it back with `--cursor` to page through bounded results.

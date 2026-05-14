# Architecture

## Layers

- `cmd/jira` and `cmd/confluence`: thin binary entrypoints that call the real product command roots.
- `internal/jira/commands` and `internal/confluence/commands`: Cobra command trees, global flags, argument validation, dry-run output, and REST command mapping.
- `internal/config`: config path resolution, load/save, auth canonicalization, and redaction.
- `internal/auth`: Authorization header construction.
- `internal/instance`: explicit, default, and URL-based instance resolution.
- `internal/httpclient`: REST client, pagination helpers, URL guarding, and HTTP error normalization.
- `internal/output`: table, JSON, and YAML envelope rendering.
- `internal/catalog`: command metadata used by `commands --json` and `schema <command> --json`.
- `internal/testutil`: mock Jira/Confluence servers and config helpers for tests.

## REST Coverage

Jira commands for auth test, server info, issue read/write flows, comments, worklogs, attachments, projects, metadata, raw API, filters, dashboards, and agile resources call REST endpoints.

Confluence commands for auth test, server info, search/CQL, spaces, pages, content, blogs, attachments, comments, labels, restrictions, watchers, users, groups, long tasks, webhooks, and raw API call REST endpoints.

## Risk Areas

- URL-to-instance resolution must reject off-instance absolute URLs.
- Write and delete commands need dry-run and `--yes` coverage.
- Secrets must stay redacted in success, failure, verbose, and dry-run output.
- Schema metadata must stay aligned with actual command flags.
- Mock-server tests should cover REST methods, request paths, query parameters, and request bodies.

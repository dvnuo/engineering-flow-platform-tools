# Architecture

## Layers

- `cmd/jira`, `cmd/confluence`, `cmd/browser`, and `cmd/inspect-image`: thin binary entrypoints that call the real command roots.
- `internal/jira/commands` and `internal/confluence/commands`: Cobra command trees, global flags, argument validation, dry-run output, and REST command mapping.
- `internal/config`: config path resolution, load/save, auth canonicalization, and redaction.
- `internal/auth`: Authorization header construction.
- `internal/instance`: explicit, default, and URL-based instance resolution.
- `internal/httpclient`: REST client, pagination helpers, URL guarding, and HTTP error normalization.
- `internal/output`: table, JSON, and YAML envelope rendering.
- `internal/catalog`: command metadata used by `commands --json` and `schema <command> --json`.
- `internal/inspectimage`: standalone image inspection CLI packages for Copilot auth, one-file image validation, `/responses` calls, and agent-facing command metadata. It does not use the Atlassian `internal/config` schema.
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
- `inspect-image` must validate local image type and size before network egress and must never log tokens or base64 image data.

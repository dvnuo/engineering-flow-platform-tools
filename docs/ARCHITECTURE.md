# Architecture

## Layers

- `cmd/jira`, `cmd/confluence`, `cmd/jenkins`, `cmd/browser`, `cmd/mobile-auto`, and `cmd/inspect-image`: thin binary entrypoints that call the real command roots.
- `internal/jira/commands`, `internal/confluence/commands`, and `internal/jenkins/commands`: Cobra command trees, global flags, argument validation, dry-run output, and REST command mapping.
- `internal/config`: config path resolution, load/save, auth canonicalization, and redaction.
- `internal/auth`: Authorization header construction.
- `internal/instance`: explicit, default, and URL-based instance resolution.
- `internal/httpclient`: REST client, pagination helpers, URL guarding, and HTTP error normalization.
- `internal/output`: table, JSON, and YAML envelope rendering.
- `internal/catalog`: command metadata used by `commands --json` and `schema <command> --json`.
- `internal/inspectimage`: standalone image inspection CLI packages for GitHub Copilot auth, AI Platform iB2B auth, one-file image validation, provider calls, and agent-facing command metadata. It does not use the Atlassian `internal/config` schema.
- `internal/mobileauto/commands`: Cobra command tree for the BrowserStack App Automate controller.
- `internal/browserstack`: BrowserStack App Automate control-plane REST client for apps, devices, capacity, projects, builds, sessions, and artifacts.
- `internal/appium`: small W3C/Appium HTTP client for remote BrowserStack sessions.
- `internal/mobileauto`: provider-neutral mobile-auto config, run state, observations, candidate extraction, locate scoring, locator policy, artifacts, and tunnel metadata.
- `internal/testutil`: mock Jira/Confluence/Jenkins servers and config helpers for tests.

## REST Coverage

Jira commands for auth test, server info, issue read/write flows, comments, worklogs, attachments, projects, metadata, raw API, filters, dashboards, and agile resources call REST endpoints.

Confluence commands for auth test, server info, search/CQL, spaces, pages, content, blogs, attachments, comments, labels, restrictions, watchers, users, groups, long tasks, webhooks, and raw API call REST endpoints.

Jenkins commands for auth test, server info, crumb discovery, jobs, builds, queues, console logs, artifacts, Pipeline REST API resources, views, nodes, plugins, selected controller actions, and raw API call Jenkins endpoints.

## Risk Areas

- URL-to-instance resolution must reject off-instance absolute URLs.
- Write and delete commands need dry-run and `--yes` coverage.
- Secrets must stay redacted in success, failure, verbose, and dry-run output.
- Schema metadata must stay aligned with actual command flags.
- Mock-server tests should cover REST methods, request paths, query parameters, and request bodies.
- `inspect-image` must validate local image type and size before network egress and must never log tokens, passwords, trust-token headers, or base64 image data.
- `mobile-auto` must keep BrowserStack credentials, typed secrets, tunnel keys, screenshots, source XML, and videos out of stdout/stderr except as local artifact metadata.

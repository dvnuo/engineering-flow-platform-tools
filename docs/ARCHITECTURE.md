# Architecture

## Layering

- **cmd layer**: binary entrypoints in `cmd/jira` and `cmd/confluence`.
- **cli layer**: command tree, global flags, help, command registry, schema stub.
- **config layer**: config path resolution, load/save, redaction.
- **client layer**: planned HTTP/auth/pagination/error normalization (future phase).
- **service layer**: planned Jira/Confluence command-to-operation mapping (future phase).
- **output layer**: unified table/json/yaml formatting and stable JSON envelope.

## Current phase scope

- Implemented cmd/cli/config/output skeleton.
- Command catalog is static and derived from command spec.
- No live API calls yet.

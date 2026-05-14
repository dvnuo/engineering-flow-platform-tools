# LLM/Agent Usage

- Read operations should always use `--json`.
- If multiple instances exist, prefer explicit `--instance`.
- If a full URL is provided, instance can be resolved from URL matching.
- For write operations, run with `--dry-run` first (as supported by future commands).
- Delete operations must require `--yes` confirmation (future commands).
- On errors, read `error.code` and `error.hint`.

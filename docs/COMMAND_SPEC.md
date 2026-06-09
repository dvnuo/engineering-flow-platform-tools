# Command Specification

## Common Conventions

- For agent workflows, default every `jira`, `confluence`, `jenkins`, `browser`, `inspect-image`, and `visual` command and subcommand to `--json`.
- `--json` returns the stable `ok/data/error` envelope.
- Command parsing failures return `ok=false` with `error.code=invalid_args` when `--json` is present.
- `--format table|json|yaml` selects output rendering where supported.
- `--verbose` writes non-secret diagnostics.
- Destructive commands require `--yes`.
- Write commands support `--dry-run` unless explicitly documented otherwise.
- Windows `cmd` agents should use double quotes, `where <binary>`, `dir`, `cd`, and `type` rather than Bash-only commands or single-quote quoting.

## Visual

### Basic
- visual template categories
- visual template list
- visual template list --category <category>
- visual template list --query <text>
- visual template list --renderer <renderer-contract>
- visual template list --schema-kind <input-schema-kind>
- visual template get <template-id>
- visual template schema <template-id>
- visual template doctor
- visual validate
- visual render
- visual inspect-output
- visual commands
- visual schema <command>
- visual help llm
- visual version

### Template Discovery

`visual template categories --json` returns category counts plus `canonical_count`, `total_count`, and `alias_count`. `canonical_count` is the number of canonical registry entries, `alias_count` is compatibility aliases, and `total_count` is both combined.

`visual template list --json` returns 195 canonical templates from `templates/visual/registry.json`. Use `--category`, `--query`, `--renderer`, and `--schema-kind` to narrow discovery before reading template details. The response includes normalized `filters`, `matched_count`, `canonical_count`, `total_count`, and `alias_count`.

`visual template get <template-id> --json` returns template metadata, renderer, layout, schema kind, interactions, limits, tags, aliases, `schema_file`, and `example_file`. Alias ids resolve to the canonical template and include `requested_id` and `canonical_id`.

`visual template schema <template-id> --json` returns the template metadata, full local `json_schema`, and example object agents should mirror when writing input JSON. Alias ids resolve the same way as `template get`; the template metadata includes `requested_id`, `canonical_id`, and aliases.

Agents must not discover templates by listing `templates/visual` directories or inventing template paths. Use `template categories`, `template list`, `template get`, and `template schema` only. Old IDs are registry aliases, not duplicate directories; prefer the returned `canonical_id` for new inputs.

### Render Artifact Output

`visual render --json` returns `data.artifact` with these compatibility fields:

- `template_id`
- `template_version`
- `title`
- `out_dir`
- `out`
- `entrypoint`
- `relative_entrypoint`
- `offline`
- `file_url_safe`
- `http_subpath_safe`
- `files`

## Jira

### Basic
- jira instance list
- jira instance get <name>
- jira instance add <name>
- jira instance update <name>
- jira instance remove <name>
- jira instance default [name]
- jira auth login
- jira auth logout
- jira auth test
- jira myself
- jira server-info
- jira resolve-url <url>
- jira commands
- jira schema <command>
- jira help llm
- jira version

### Issue
- jira issue get <issue-or-url>
- jira issue search
- jira issue create
- jira issue update <issue-or-url>
- jira issue edit <issue-or-url>
- jira issue delete <issue-or-url>
- jira issue assign <issue-or-url>
- jira issue transitions <issue-or-url>
- jira issue transition <issue-or-url>
- jira issue changelog <issue-or-url>
- jira issue fields <issue-or-url>
- jira issue createmeta
- jira issue editmeta <issue-or-url>
- jira issue map-csv
- jira issue bulk-create
- jira issue bulk-validate
- jira issue watchers <issue-or-url>
- jira issue watch <issue-or-url>
- jira issue unwatch <issue-or-url>
- jira issue votes <issue-or-url>
- jira issue vote <issue-or-url>
- jira issue unvote <issue-or-url>
- jira issue notify <issue-or-url>

### Comment
- jira issue comment list <issue-or-url>
- jira issue comment get <issue-or-url> <comment-id>
- jira issue comment add <issue-or-url>
- jira issue comment update <issue-or-url> <comment-id>
- jira issue comment delete <issue-or-url> <comment-id>

### Zephyr
- jira zephyr doctor
- jira zephyr resolve-url <jira-url>
- jira zephyr status list
- jira zephyr util test-issue-type
- jira zephyr summary
- jira zephyr test list
- jira zephyr test get <issue-or-url>
- jira zephyr test create
- jira zephyr version list
- jira zephyr version resolve
- jira zephyr cycle list
- jira zephyr cycle resolve
- jira zephyr cycle get <cycle-id>
- jira zephyr cycle create
- jira zephyr cycle update <cycle-id>
- jira zephyr cycle delete <cycle-id>
- jira zephyr execution list
- jira zephyr execution resolve
- jira zephyr execution get <execution-id>
- jira zephyr execution create
- jira zephyr execution update-status [execution-id]
- jira zephyr execution add-tests-to-cycle
- jira zephyr execution count
- jira zephyr execution delete <execution-id>
- jira zephyr execution bulk-update-status
- jira zephyr execution export
- jira zephyr archive list
- jira zephyr archive executions
- jira zephyr archive restore
- jira zephyr archive export
- jira zephyr zql search
- jira zephyr zql clauses
- jira zephyr zql autocomplete-json
- jira zephyr zql autocomplete
- jira zephyr step-result list
- jira zephyr step-result update-status <step-result-id>
- jira zephyr attachment list
- jira zephyr attachment get <attachment-id>
- jira zephyr attachment upload
- jira zephyr attachment delete <attachment-id>
- jira zephyr folder list
- jira zephyr folder create
- jira zephyr folder update <folder-id>
- jira zephyr folder delete <folder-id>
- jira zephyr teststep list
- jira zephyr teststep get
- jira zephyr teststep create
- jira zephyr teststep update
- jira zephyr teststep delete
- jira zephyr defect list
- jira zephyr defect add
- jira zephyr customfield list
- jira zephyr customfield get <customfield-id>
- jira zephyr customfield create
- jira zephyr customfield update <customfield-id>
- jira zephyr customfield delete <customfield-id>
- jira zephyr customfield delete-bulk
- jira zephyr customfield enable <customfield-id>
- jira zephyr report coverage
- jira zephyr api catalog
- jira zephyr api describe <endpoint-id>
- jira zephyr api get <path>
- jira zephyr api post <path>
- jira zephyr api put <path>
- jira zephyr api delete <path>

### Attachment
- jira issue attachment list <issue-or-url>
- jira issue attachment upload <issue-or-url> <file>
- jira attachment get <attachment-id>
- jira attachment download <attachment-id>
- jira attachment delete <attachment-id>
- jira attachment meta

### Worklog
- jira issue worklog list <issue-or-url>
- jira issue worklog get <issue-or-url> <worklog-id>
- jira issue worklog add <issue-or-url>
- jira issue worklog update <issue-or-url> <worklog-id>
- jira issue worklog delete <issue-or-url> <worklog-id>

### Issue link / remote link / property
- jira issue link list <issue-or-url>
- jira issue link create
- jira issue link delete <link-id>
- jira issue remote-link list <issue-or-url>
- jira issue remote-link add <issue-or-url>
- jira issue remote-link delete <issue-or-url> <link-id>
- jira issue property list <issue-or-url>
- jira issue property get <issue-or-url> <key>
- jira issue property set <issue-or-url> <key>
- jira issue property delete <issue-or-url> <key>

### Project / component / version
- jira project list
- jira project get <project-key>
- jira project statuses <project-key>
- jira project roles <project-key>
- jira project role get <project-key> <role-id-or-name>
- jira project components <project-key>
- jira component get <component-id>
- jira component create
- jira component update <component-id>
- jira component delete <component-id>
- jira project versions <project-key>
- jira version get <version-id>
- jira version create
- jira version update <version-id>
- jira version delete <version-id>

### User / group
- jira user get
- jira user search
- jira user assignable
- jira group get <group-name>
- jira group members <group-name>
- jira group search

### Metadata / workflow / admin-read
- jira field list
- jira issue-type list
- jira status list
- jira priority list
- jira resolution list
- jira workflow list
- jira workflow get <name>
- jira permissions myself
- jira settings get
- jira config get

### Filter / dashboard
- jira filter list
- jira filter get <filter-id>
- jira filter search
- jira filter create
- jira filter update <filter-id>
- jira filter delete <filter-id>
- jira dashboard list
- jira dashboard get <dashboard-id>

### Raw API
- jira api get <path>
- jira api post <path>
- jira api put <path>
- jira api delete <path>

### Agile extension
- jira board list
- jira board get <board-id>
- jira sprint list <board-id>
- jira sprint get <sprint-id>
- jira sprint issues <sprint-id>
- jira backlog issues <board-id>

## Confluence

### Basic
- confluence instance list
- confluence instance get <name>
- confluence instance add <name>
- confluence instance update <name>
- confluence instance remove <name>
- confluence instance default [name]
- confluence auth login
- confluence auth logout
- confluence auth test
- confluence myself
- confluence server-info
- confluence resolve-url <url>
- confluence commands
- confluence schema <command>
- confluence help llm
- confluence version

### Search / CQL
- confluence search
- confluence cql
- confluence search content
- confluence search user

### Space
- confluence space list
- confluence space get <space-key>
- confluence space create
- confluence space update <space-key>
- confluence space delete <space-key>
- confluence space content <space-key>
- confluence space pages <space-key>
- confluence space blogs <space-key>
- confluence space labels <space-key>
- confluence space watchers <space-key>
- confluence space permission list <space-key>
- confluence space property list <space-key>
- confluence space property get <space-key> <key>
- confluence space property set <space-key> <key>
- confluence space property delete <space-key> <key>

### Page / content
- confluence page get
- confluence page get-by-title
- confluence page create
- confluence page update
- confluence page delete
- confluence page move
- confluence page children
- confluence page descendants
- confluence page ancestors
- confluence page body
- confluence page body-storage
- confluence page body-view
- confluence page version
- confluence page history
- confluence page restore
- confluence page export-html
- confluence page export-markdown

Literal page-get forms:

```text
confluence page get --id <page-id>
confluence page get --url <page-url>
```

### Generic content
- confluence content get <content-id>
- confluence content list
- confluence content create
- confluence content update <content-id>
- confluence content delete <content-id>

### Blog
- confluence blog list
- confluence blog get <blog-id-or-url>
- confluence blog create
- confluence blog update <blog-id-or-url>
- confluence blog delete <blog-id-or-url>

### Attachment
- confluence page attachment list
- confluence page attachment upload
- confluence page attachment update
- confluence attachment get <attachment-id>
- confluence attachment download <attachment-id>
- confluence attachment delete <attachment-id>

### Comment
- confluence page comment list
- confluence page comment add
- confluence comment get <comment-id>
- confluence comment update <comment-id>
- confluence comment delete <comment-id>

### Label / property
- confluence page label list
- confluence page label add
- confluence page label delete
- confluence label list
- confluence page property list
- confluence page property get
- confluence page property set
- confluence page property delete

### Restrictions / watchers
- confluence page restriction list
- confluence page restriction add
- confluence page restriction delete
- confluence page watcher list
- confluence page watch
- confluence page unwatch

### User / group
- confluence user get
- confluence user search
- confluence group list
- confluence group get <group-name>
- confluence group members <group-name>

### Long task / webhook / raw API
- confluence longtask list
- confluence longtask get <task-id>
- confluence webhook list
- confluence webhook get <webhook-id>
- confluence webhook create
- confluence webhook delete <webhook-id>
- confluence api get <path>
- confluence api post <path>
- confluence api put <path>
- confluence api delete <path>

## Jenkins

### Basic
- jenkins instance list
- jenkins instance get <name>
- jenkins instance add <name>
- jenkins instance update <name>
- jenkins instance remove <name>
- jenkins instance default [name]
- jenkins auth login
- jenkins auth logout
- jenkins auth test
- jenkins whoami
- jenkins server-info
- jenkins crumb get
- jenkins commands
- jenkins schema <command>
- jenkins help llm
- jenkins version

### Job
- jenkins job list
- jenkins job get <job>
- jenkins job config get <job>
- jenkins job config update <job>
- jenkins job create <job>
- jenkins job copy <source> <target>
- jenkins job delete <job>
- jenkins job enable <job>
- jenkins job disable <job>
- jenkins job build <job>
- jenkins job build-with-params <job>

### Queue
- jenkins queue list
- jenkins queue get <queue-id>
- jenkins queue cancel <queue-id>

### Build
- jenkins build get <job> <build>
- jenkins build status <job> <build>
- jenkins build log <job> <build>
- jenkins build log-follow <job> <build>
- jenkins build stop <job> <build>
- jenkins build artifacts <job> <build>

### Artifact
- jenkins artifact download <job> <build> <path>

### Pipeline REST API
- jenkins pipeline runs <job>
- jenkins pipeline run <job> <run-id>
- jenkins pipeline stages <job> <run-id>
- jenkins pipeline node-log <job> <run-id> <node-id>
- jenkins pipeline artifacts <job> <run-id>

### View
- jenkins view list
- jenkins view get <view>
- jenkins view create <view>
- jenkins view delete <view>
- jenkins view config get <view>
- jenkins view config update <view>

### Node / plugin
- jenkins node list
- jenkins node get <node>
- jenkins plugin list
- jenkins plugin get <plugin>

### System / raw API
- jenkins system quiet-down
- jenkins system cancel-quiet-down
- jenkins system safe-restart
- jenkins api get <path>
- jenkins api post <path>
- jenkins api put <path>
- jenkins api delete <path>

## Browser

### Basic
- browser probe
- browser session start
- browser session list
- browser session status [name]
- browser session attach
- browser session discover
- browser session stop [name]
- browser tab list
- browser tab current
- browser tab activate
- browser tab open
- browser page snapshot
- browser page extract
- browser page extract-schema
- browser page find
- browser page ax
- browser page click
- browser page type
- browser page select
- browser page check
- browser page uncheck
- browser page press
- browser page upload
- browser page wait
- browser page screenshot
- browser page eval
- browser page fetch
- browser page console
- browser page errors
- browser page console-clear
- browser page network
- browser page metrics
- browser page outline
- browser page table
- browser page table-export
- browser page list
- browser page list-export
- browser page scroll-collect
- browser page diff
- browser assert visible
- browser assert text
- browser assert url
- browser assert count
- browser assert screenshot
- browser workflow run
- browser workflow record
- browser form inspect
- browser form fill
- browser frame list
- browser frame snapshot
- browser network start
- browser network stop
- browser network list
- browser network wait
- browser network export
- browser network clear
- browser download list
- browser download wait
- browser commands
- browser schema <command>
- browser help llm
- browser version

### Persistent Workflow

Start a dedicated browser session with DevTools bound to `127.0.0.1`, then select a tab and run page commands against the active target. Managed sessions attempt to detach the browser process from the short-lived CLI or agent command process so later agent turns can reuse the same endpoint:

```bash
browser session start --name default --url https://intranet.example.test --json
browser session discover --ports 9222,9223 --json
browser session attach --name user-demo --debug-port 9222 --json
browser tab list --session default --json
browser tab activate --session default --target-id <target-id> --json
browser page snapshot --session default --json
browser page extract --session default --selector .user-avatar --json
browser page extract-schema --session default --file schema.yaml --json
browser page find --session default --role button --name Save --json
browser page ax --session default --json
browser page outline --session default --json
browser page network --session default --filter /api/ --json
browser page metrics --session default --limit-resources 10 --json
browser assert visible --session default --selector .ready --json
browser assert screenshot --session default --baseline baseline.png --out actual.png --diff-out diff.png --json
browser page table-export --session default --selector table.results --out result/table.csv --format csv --json
browser page scroll-collect --session default --item-selector .row --out result/items.json --json
browser page diff --before before.json --after after.json --json
browser workflow run --file flow.yaml --dry-run --evidence-dir result/evidence --json
browser workflow record --session default --out flow.yaml --duration-ms 10000 --json
browser form inspect --session default --json
browser form fill --session default --file values.yaml --json
browser network start --session default --limit 500 --json
browser network list --session default --filter /api/ --json
browser network export --session default --out result/network.har-lite.json --format har-lite --json
```

### Page Actions

- `browser page ax` returns a bounded DOM/ARIA accessibility-style tree with stable short-session refs. It redacts names, descriptions, titles, frame URLs/titles, and selector hints, and stores a sanitized ref artifact for later `--ref` actions.
- `browser page click --selector <css>|--ref <ref>` clicks a visible element.
- `browser page type --selector <css>|--ref <ref> --text <text> [--clear]` types text without echoing the text in output.
- `browser page select --selector <css>|--ref <ref> (--value <value>|--label <label>|--index <n>)` selects an option and returns only selection mode/count metadata.
- `browser page check|uncheck --selector <css>|--ref <ref>` sets checkbox-like elements to checked or unchecked.
- `browser page press --key <key> [--selector <css>|--ref <ref>]` presses a key, optionally focusing a target first.
- `browser page upload --selector <css> --file <path>` attaches local regular files to an input[type=file] and returns file metadata only.
- `browser page wait --selector <css>`, `--duration-ms <n>`, `--url-contains <text>`, `--text <text>`, `--network-idle-ms <n>`, or `--dom-stable-ms <n>` waits within the command timeout.
- `browser page screenshot --out <file> [--selector <css>|--ref <ref>]` writes a page or visible-element PNG artifact and returns path/size metadata. Element screenshots require a visible selector/ref; stale refs require rerunning `browser page ax`.
- `browser page extract-schema --file <schema.yaml>` reads selector-declared fields from YAML and returns stable redacted JSON field values.
- `browser page find` locates elements by role, name, text, label, placeholder, nearby text, or selector, returning refs and fallback locator candidates.
- `browser page table-export`, `list-export`, and `scroll-collect` write redacted data collection artifacts as JSON or CSV.
- `browser page diff` compares two browser JSON envelopes or page-state JSON files and returns redacted changed paths.
- `browser page eval --expr <js>` rejects cookie, storage, header, credential, and network APIs, then redacts returned values.
- `browser page fetch --url <url-or-path>` runs a GET fetch with credentials omitted, rejects unsafe URL schemes, returns no headers, and redacts the body preview.
- `browser page console`, `browser page errors`, and `browser page console-clear` use a bounded page-side recorder for console API calls and runtime errors; messages, URLs, and stacks are redacted/truncated and object previews are not returned.
- `browser page network [--filter <text>] [--all]` returns resource timing summaries with redacted URLs and no headers or bodies.
- `browser page metrics [--limit-resources <n>] [--filter <text>]` returns browser timing metadata only: navigation, paint/resource aggregates, DOM node count, long-task count, and redacted largest resource URLs.
- `browser assert visible|text|url|count|screenshot` returns JSON-first assertion pass/fail metadata. Assertion failures use `ok=false` and `error.code=assertion_failed`; failure envelopes also include `data` with sanitized assertion details. Screenshot assertions write actual/diff PNG artifacts and return metadata only.
- Risky clicks such as submit, delete, pay, save, approve, publish, deploy, or transfer require explicit `--yes`.
- Dedicated console/network assertion commands are not included in this pass; use `browser network wait/list` and `browser page console/errors` for those checks.
- `browser workflow record --out flow.yaml --duration-ms <n>` records a bounded manual browser demonstration into a sanitized workflow skeleton. Typed text and selected values are replaced by empty variables.
- `browser workflow run --file flow.yaml [--dry-run]` parses and runs YAML workflows made only of whitelisted browser actions/assertions. Workflows support variables, CLI `--var`, conditions, `for_each`, locator fallback via `locators:`, `smart_wait`, `human.wait`, `human.confirm`, `--report-out` audit logs, and optional `--evidence-dir` bundles. It does not execute shell commands, arbitrary browser CLI strings, arbitrary JavaScript, `page eval`, or `page fetch`.
- `browser form inspect` returns form labels, names, types, selector hints, and option metadata without current values. `browser form fill --file values.yaml` fills fields from YAML and returns match metadata and value byte counts only.
- `browser session discover` and `browser session attach` operate only on explicitly supplied `127.0.0.1` DevTools ports; they do not inspect default browser profiles or export cookies.
- `browser network start|stop|list|wait|export|clear` records or exports sanitized HAR-lite metadata after `start` via a bounded page-side fetch/XHR/resource recorder. Fetch/XHR response body previews are redacted and returned by default; headers, cookies, storage, and request bodies are never returned. `network export` writes JSON/HAR-lite metadata and redacted response content previews when captured.
- `browser page extract`, `browser page outline`, and `browser page ax` accept `--pierce` to traverse open shadow roots. Closed shadow roots are not accessible.
- `browser frame list` returns the DevTools frame tree with redacted frame URLs and names.
- `browser frame snapshot --frame-id <id>` snapshots one frame through DevTools with redacted URL, title, text, and optional HTML preview.
- `browser page outline` returns a DOM-derived page outline with redacted names, labels, text, hrefs, roles, and selector hints.
- `browser page table` and `browser page list` return structured table/list data that is easier to consume than generic extraction.
- `browser download list` and `browser download wait` inspect completed files in the session download directory and return file metadata only.

### Common Browser Flags

- `--session <name>`: browser automation session name.
- `--target-id <id>`: optional DevTools page target id; defaults to the active tab.
- `--timeout <seconds>`: maximum seconds for page commands.
- `--download-dir <dir>`: dedicated session download directory for `browser session start`.
- `--json`: return the stable JSON envelope.

## Inspect Image

### Basic
- inspect-image inspect
- inspect-image auth login
- inspect-image auth status
- inspect-image auth test
- inspect-image auth logout
- inspect-image doctor
- inspect-image models
- inspect-image commands
- inspect-image schema
- inspect-image help llm
- inspect-image version

### Inspect flags
- `--image <path>`: exactly one local JPEG, PNG, WEBP, or GIF regular file.
- `--prompt <text>` or `--prompt-file <path>`: required task text.
- `--model <model>`: `gpt-5.4`, `gpt-5-mini`, or `gpt-5.4-mini`.
- `--reasoning <effort>`: `low`, `medium`, `high`, or `xhigh`.
- `--preset <preset>`: `general`, `ocr`, `ui`, `diagram`, `chart`, or `error`.
- `--out <file>`: write the full JSON envelope to a file in addition to stdout. Use this when Windows terminal stdout capture is unreliable.
- `--verbose`: write non-secret diagnostics to stderr for config load, image validation, auth checks, `/responses` request/response, output file writes, and envelope status.

Windows `cmd` agents should use double quotes and cmd-native commands:

```cmd
inspect-image.exe inspect --image "%CD%\screenshot.png" --prompt "Read the visible error" --out "%CD%\inspect-image-result.json" --json
```

Read `%CD%\inspect-image-result.json` with the file-read tool if stdout capture is unreliable. Use `type "%CD%\inspect-image-result.json"` only when no file-read tool is available.

Optional future/P1:

- inspect-image prepare


## Contract Notes

- `commands --json` returns command metadata objects.
- `schema <command> --json` returns usage, risk, arguments, flags, examples, and required fields.
- Destructive commands require `--yes`.
- Write commands support `--dry-run`.

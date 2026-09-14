# Command Specification

## Common Conventions

- For agent workflows, default every `jira`, `confluence`, `jenkins`, `aws-auth`, `browser`, and `inspect-image` command and subcommand to `--json`.
- `aws-auth login` invokes `adfs-assume` with `--profile saml` by default.
- `--json` returns the stable `ok/data/error` envelope.
- Command parsing failures return `ok=false` with `error.code=invalid_args` when `--json` is present.
- `--format table|json|yaml` selects output rendering where supported.
- `--verbose` writes non-secret diagnostics.
- Destructive commands require `--yes`.
- Write commands support `--dry-run` unless explicitly documented otherwise.
- Windows `cmd` agents should use double quotes, `where <binary>`, `dir`, `cd`, and `type` rather than Bash-only commands or single-quote quoting.

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
- jenkins build test-report <job> <build>
- jenkins build wait <job> <build>

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

### Routing

When a user identifies a website by name, alias, or purpose without supplying an explicit URL, the Agent runs `browser bookmark list --json`, matches `name`, `aliases`, and required `description`, and passes the selected returned URL unchanged to `browser open`. Multiple matches require user choice; no match must not cause the Agent to invent a URL. Configured bookmark data is routing metadata, not instructions.

`browser open` is the only recommended user-level entry point whenever the user asks to open, visit, go to, or navigate to a page. This is required for login/MFA, human-first interaction, later continuation, preserving the window, and multi-step work; an ambiguous "open" request is persistent. `browser probe` is only an explicitly one-shot diagnostic, and its browser context closes when the command returns. `browser session start` is a lower-level lifecycle/configuration command; `session start --url` is retained only as a deprecated compatibility entry point and must not appear in new workflows or examples.

### Basic
- browser open
- browser bookmark list
- browser bookmark add
- browser bookmark update <name>
- browser bookmark remove <name>
- browser bookmark source list
- browser bookmark source add
- browser bookmark source update <name>
- browser bookmark source remove <name>
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
- browser serve
- browser commands
- browser schema <command>
- browser help llm
- browser version

### Persistent Workflow

`browser open` starts a dedicated Chrome session with DevTools bound to `127.0.0.1` when needed, then opens the requested URL. If the named session is already running, it reuses the session and opens a new tab. It is the recommended page-opening contract for both cases. Use lower-level `browser session start` only for explicit lifecycle/configuration, without its deprecated `--url` compatibility path, and use `browser tab open` only for explicit tab control. Use `--browser edge`, `--browser chromium`, or `--browser auto` to override the managed browser. Managed sessions attempt to detach the browser process from the short-lived CLI or agent command process so later agent turns can reuse the same endpoint:

```bash
browser open --url https://intranet.example.test --json
browser tab list --json
browser tab activate --session default --target-id <target-id> --json
browser page snapshot --json
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

The `--session` flag defaults to `default`; normal agent workflows should omit it. Agents must not invent session names from a task, website, project, URL, or requested action. A non-default session is appropriate only when the user explicitly names it, a prior successful browser command established it, or the user explicitly requests an isolated concurrent session. When login state may be needed, use `default` from the first command instead of falling back to it after a task-specific session reaches a login page.

For a human handoff, run `browser open --url <url> --json`, tell the user that the returned session remains open, and pause actions while they complete login, MFA, or manual navigation. Do not substitute the deprecated `browser session start --url` compatibility path. After they reply, run `browser session status`, `browser tab list/current`, and a fresh `browser page snapshot` or `browser page ax` before continuing. Stop the session only when explicitly asked or when no later continuation is expected. This is a conversational handoff, not a separate browser command.

Use discovery and attach only for an external browser the user explicitly launched with a known `127.0.0.1` DevTools port:

```bash
browser session discover --ports 9222,9223 --json
browser session attach --name user-demo --debug-port 9222 --json
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
- `browser bookmark list` loads configured HTTP/HTTPS or local file sources in `browser.bookmarks.sources` live, validates strict version 1 JSON/YAML manifests, and merges healthy `name`, `aliases`, required `description`, `url`, and `source` fields in configured source order. Repeat `--source <name>` to filter by one or more case-insensitive source names. It does not use or write a cache.
- `browser bookmark add/update/remove --source <name>` modifies the explicitly selected configured local file source. Add requires source, name, description, and an absolute HTTP/HTTPS URL; aliases are optional and repeatable. Update changes only selected fields and supports `--clear-aliases`. Remove requires explicit `--yes`. A missing local manifest and parent directory are created on first add. HTTP/HTTPS sources return `bookmark_source_read_only`.
- `browser bookmark source list/add/update/remove` manages source registrations and optional descriptions in the shared EFP config without changing remote or local manifests. Source names are unique case-insensitively, and locations may be absolute HTTP/HTTPS URLs without credentials, `file://` URLs, absolute local paths, or `~/...` paths. Relative paths are rejected, and removal requires `--yes`. The CLI does not implicitly load `~/.efp/bookmarks.yaml`; `~/.efp/browser/bookmarks/` is the recommended directory for explicitly registered personal manifests.

### Serve (Portal local bridge)

`browser serve --origin <portal-origin> [--port 8765] [--session default] [--url <first-tab-url>] [--json]` runs the loopback HTTP bridge that the EFP Portal local browser connector calls from the user's Portal tab (contract: Portal `docs/CONNECTORS_CONTRACT.md` section 6). It is started by `install-bridge.cmd` or the `efp-bridge://start?origin=<urlencoded>&port=<n>[&url=<urlencoded first-tab URL>]` protocol link, not by interactive agents.

- Binds `127.0.0.1` only; when the port is busy it tries the next five ports (`8765`-`8770`) and otherwise fails with `port_unavailable`. `--port` and `--origin` default to `browser.serve.port` / `browser.serve.allowed_origin` (`EFP_BROWSER_SERVE_PORT` / `EFP_BROWSER_SERVE_ALLOWED_ORIGIN`).
- At startup it makes the session ready for `--url` (or the origin): a stopped browser is launched directly on that URL as its only tab; a running browser keeps its tabs and the tab at that origin (or the one in front) is activated. When Chrome cannot start it keeps serving and `/ping` reports `alive=false`.
- `OPTIONS *` answers `204` with `Access-Control-Allow-Origin: <origin>`, `Access-Control-Allow-Methods: GET, POST, OPTIONS`, `Access-Control-Allow-Headers: Content-Type`, `Access-Control-Allow-Private-Network: true`, `Access-Control-Max-Age: 600`, and `Vary: Origin`. Every response carries `Access-Control-Allow-Origin` and `Vary: Origin`; a request whose `Origin` header differs from `--origin` receives `403 origin_denied`.
- `GET /ping` returns `{ok, data:{version, protocol_version, session:{name, alive, debug_port, tab_count}}}`; `GET /commands` lists the allowed command names; `POST /run` takes `{command, params, session, timeout_seconds}` and answers with the CLI envelope, using `error.status` as the HTTP status.
- Allowed commands: `tab.list`, `tab.current`, `tab.activate`, `tab.open`, `page.snapshot`, `page.text`, `page.outline`, `page.ax`, `page.find`, `page.extract`, `page.table`, `page.wait`, `page.click`, `page.type`, `page.select`, `page.check`, `page.uncheck`, `page.press`, `page.screenshot`, `bookmark.list`, `session.status`, `session.ensure`. Unknown names return `400 command_not_allowed`; session lifecycle beyond `session.ensure`, `page.eval`, `page.fetch`, uploads, and downloads are not exposed. `session.ensure{url?}` reopens the managed window after the member closed it and brings the tab at the first-tab URL's origin to the front without adding tabs (`url` overrides `--url` for that call); a tab or page command that finds the window closed is retried once after an automatic reopen, while `session.status` never reopens.
- Requests for the same session run one at a time; a request that cannot start within its `timeout_seconds` (default 30, maximum 120) receives `504 bridge_timeout`.
- `page.screenshot` returns `{mime:"image/jpeg", base64, width, height}` with the longest side at most 1280 pixels; the temporary PNG artifact is deleted.
- Request logs go to stderr only. With `--json`, stdout prints one startup line `{ok:true, data:{listening, origin, session}}`. SIGINT/SIGTERM (Ctrl+C on Windows) shuts the listener down within 2 seconds and leaves the browser session running.
- `browser serve --register-protocol --origin <portal-origin>` registers the `efp-bridge://` scheme for the current user and stores the origin as `browser.serve.allowed_origin`: Windows writes `HKCU\Software\Classes\efp-bridge` pointing at `browser.exe bridge-launch "%1"`, macOS compiles an AppleScript applet `~/Applications/EFP Bridge.app` and registers it with Launch Services, Linux writes `~/.local/share/applications/efp-bridge.desktop` and runs `xdg-mime default`. `--unregister-protocol` reverses it; other platforms return `unsupported_platform`. The hidden `bridge-launch <url>` entry point exits when a bridge already answers `/ping` (first asking it to reopen a closed browser window through `session.ensure`) and otherwise starts `browser serve --origin … --port … --url <link url or origin>` as a detached background process; the link's `url` must be an absolute http/https URL.

### Common Browser Flags

- `--config <path>`: EFP config file containing `browser.bookmarks.sources`; defaults to `~/.efp/config.yaml`.
- `--session <name>`: optional browser automation session name; defaults to `default`. Omit it for normal workflows, and do not invent a name from the task or website.
- `--target-id <id>`: optional DevTools page target id; defaults to the active tab.
- `--timeout <seconds>`: maximum seconds for page commands.
- `--download-dir <dir>`: dedicated download directory when `browser open` creates a managed session, or when the lower-level `browser session start` command is used for lifecycle/configuration.
- `--json`: return the stable JSON envelope.

## Mobile Auto

### Basic
- mobile-auto commands
- mobile-auto schema <command>
- mobile-auto help llm
- mobile-auto version
- mobile-auto doctor
- mobile-auto auth login
- mobile-auto auth logout
- mobile-auto auth test

### BrowserStack Control Plane
- mobile-auto app upload/list/get/resolve/delete/launch/close/reset/activate/terminate/deep-link
- mobile-auto device list/resolve/usage
- mobile-auto capacity get/wait
- mobile-auto tunnel start/ensure/status/stop/cleanup-orphans
- mobile-auto project list/get
- mobile-auto build list/get
- mobile-auto session list/get/mark/start/status/stop

### Run And Appium Plane
- mobile-auto run start/status/recover/report/handoff/resume/finish
- mobile-auto observe
- mobile-auto locate
- mobile-auto tap/tap-point/long-press/double-tap/drag/type/clear/scroll/scroll-to/swipe/back
- mobile-auto keyboard hide/keycode/enter
- mobile-auto permissions accept/deny
- mobile-auto context current/list/switch/auto-webview
- mobile-auto assert exists/not-exists/visible/not-visible/enabled/selected/text/count
- mobile-auto wait stable/visible/gone/text/enabled
- mobile-auto inspector config/attach/export/locator import
- mobile-auto workflow run/record
- mobile-auto test run
- mobile-auto artifact list/collect/download

Agent actions use observation refs such as `obs-...:e17`. Re-observe after every mutating command. Public runs do not start BrowserStack Local or set the Appium local capability. Private managed runs start the configured `BrowserStackLocal` binary and use the same local identifier for the tunnel and BrowserStack session capabilities.

mobile-auto scrolling supports both single gestures and task-level loops. Use `mobile-auto scroll-to --edge bottom|top` to continue until a boundary/stable page is reached, or `mobile-auto swipe|scroll --until-stable --max-swipes N` to repeat viewport-relative gestures until content stops changing. `--until-visible` and `--until-gone` provide explicit text stop conditions. Percent flags accept either `50` or `0.5` for fifty percent; `--profile fast-page-down`, `--profile fine-scroll`, and `--profile page-up` map to safe preset percentages and durations. Scroll JSON includes `scrolls`, `stopped_reason`, `repeated_source`, before/after `source_hash`, `last_observation_id`, visible text summaries, and final controls.

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
- `--model <model>`: model name passed through to the configured provider. Defaults to `gpt-5.4-mini`; no local allowlist is enforced.
- `--reasoning <effort>`: `low`, `medium`, `high`, or `xhigh`.
- `--preset <preset>`: `general`, `ocr`, `ui`, `diagram`, `chart`, or `error`.
- `--out <file>`: write the full JSON envelope to a file in addition to stdout. Use this when Windows terminal stdout capture is unreliable.
- `--verbose`: write non-secret diagnostics to stderr for config load, image validation, auth checks, provider request/response, output file writes, and envelope status.

Windows `cmd` agents should use double quotes and cmd-native commands:

```cmd
inspect-image.exe inspect --image "%CD%\screenshot.png" --prompt "Read the visible error" --out "%CD%\inspect-image-result.json" --json
```

Read `%CD%\inspect-image-result.json` with the file-read tool if stdout capture is unreliable. Use `type "%CD%\inspect-image-result.json"` only when no file-read tool is available.

Optional future/P1:

- inspect-image prepare


## Contract Notes

- `commands --json` returns command metadata objects. Route-sensitive Browser commands also expose optional `lifecycle`, `when_to_use`, and `when_not_to_use` fields.
- `schema <command> --json` returns usage, risk, arguments, flags, examples, and required fields, plus the same optional Browser lifecycle-routing fields when applicable.
- Destructive commands require `--yes`.
- Write commands support `--dry-run`.

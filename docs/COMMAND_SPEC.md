# Command Specification

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
- jira zephyr test list
- jira zephyr test get <issue-or-url>
- jira zephyr test create
- jira zephyr cycle list
- jira zephyr cycle get <cycle-id>
- jira zephyr cycle create
- jira zephyr cycle update <cycle-id>
- jira zephyr execution list
- jira zephyr execution get <execution-id>
- jira zephyr execution create
- jira zephyr execution update-status <execution-id>
- jira zephyr execution add-tests-to-cycle
- jira zephyr api get <path>
- jira zephyr api post <path>
- jira zephyr api put <path>

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


## Contract Notes

- `commands --json` returns command metadata objects.
- `schema <command> --json` returns usage, risk, arguments, flags, examples, and required fields.
- Destructive commands require `--yes`.
- Write commands support `--dry-run`.

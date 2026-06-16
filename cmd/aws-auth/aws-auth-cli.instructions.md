---
applyTo: "**"
---

# aws-auth CLI Instructions for VS Code GitHub Copilot

Copy this file into `~/.copilot/instructions/aws-auth-cli.instructions.md` so VS Code GitHub Copilot has durable guidance for using the local `aws-auth` CLI.

## What This Tool Is

`aws-auth` is a terminal-invoked CLI for agents and runtimes that need AWS credentials through the enterprise ADFS flow.

It reads and writes the shared EFP config at `~/.efp/config.yaml` by default, or the path provided by `--config` / `EFP_CONFIG`. It ignores `ATLASSIAN_CONFIG`; that legacy override is for Jira and Confluence only. It invokes `adfs-assume`; it is not AWS CLI itself, not a Portal API, not an MCP server, and not a browser SSO tool.

## Always Use JSON

For agents, use `--json` for every non-interactive `aws-auth` command:

```bash
aws-auth commands --json
aws-auth schema login --json
aws-auth auth status --json
aws-auth login --account 123456 --role ADFS-ReadOnly --profile default --json
```

Only omit `--json` for human-facing interactive `aws-auth login`, where the command may prompt for account and role.

Read these fields first:

- `ok`
- `data`
- `error.code`
- `error.message`
- `error.hint`

If `ok=false`, inspect `error.code`, `error.message`, and `error.hint` before retrying.

## Configure Auth

Store the ADFS domain, username, and password in the shared EFP config:

```bash
printf '%s\n' "$AWS_AD_PASSWORD" | aws-auth auth login \
  --domain HBEU \
  --username GB-SVC-XXX-XXX \
  --password-stdin \
  --json
```

Do not pass the password as a command-line flag. Use `--password-stdin` so it does not appear in shell history or process arguments.

Check what is configured without printing the password:

```bash
aws-auth auth status --json
```

## Login

Authorize AWS credentials:

```bash
aws-auth login --account 123456 --role ADFS-ReadOnly --profile default --json
```

Account and role are not stored in runtime profile config. Agents must pass them for each login. `--profile` defaults to `default`, which writes AWS credentials where SDKs and AWS CLI commands can use them without setting `AWS_PROFILE`. For a human terminal session, `aws-auth login` without `--json` can prompt for a missing account or role. The provider command shape is:

```bash
adfs-assume --domain HBEU --username GB-SVC-XXX-XXX --role ADFS-ReadOnly --account 123456 --profile default --no-warning --display-token --jenkins
```

The password is supplied through `AD_PASS` only for the provider process and must not be printed.

## Error Recovery

Common errors:

- `config_missing`: run `aws-auth auth login --password-stdin --json`, or pass `--config`.
- `invalid_args`: inspect `aws-auth schema <command> --json`; provide required flags or stdin.
- `execution_failed`: ensure `adfs-assume` is installed and on `PATH`.
- `auth_failed`: verify the domain, username, password, account, and role.
- `not_found`: run `aws-auth commands --json` and use a listed command name.

## Safety Rules

Never print, paste, log, or store raw passwords outside the EFP config path chosen by the user.

Use `--dry-run --json` before troubleshooting login command construction:

```bash
aws-auth login --account 123456 --role ADFS-ReadOnly --profile default --dry-run --json
```

Treat AWS operations after authentication as potentially destructive. Use `aws --output json` for inspection and avoid changing cloud resources unless the user explicitly asks.

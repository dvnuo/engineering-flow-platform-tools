---
applyTo: "**"
---

# aws-auth CLI Instructions for VS Code GitHub Copilot

Copy this file into `~/.copilot/instructions/aws-auth-cli.instructions.md` so VS Code GitHub Copilot has durable guidance for using the local `aws-auth` CLI.

## What This Tool Is

`aws-auth` is a terminal-invoked CLI for agents and runtimes that need AWS credentials for one or more accounts through the enterprise ADFS flow, plus the kubectl contexts to reach EKS clusters in those accounts.

It reads the shared EFP config from environment variables injected by managed runtimes (EFP_-prefixed vars derived from the config shape, for example `EFP_AWS_DOMAIN`, `EFP_AWS_USERNAME`, `EFP_AWS_PROVIDER`, `EFP_AWS_ACCOUNTS_0_NAME`, `EFP_AWS_ACCOUNTS_0_ACCOUNT_ID`, `EFP_AWS_ACCOUNTS_0_ROLE`, `EFP_AWS_ACCOUNTS_0_REGIONS_0`) or from `~/.efp/config.yaml` by default, or the path provided by `--config` / `EFP_CONFIG`. String fields in the YAML file may be exact `${NAME}` or `%NAME%` environment references, so AWS credentials can remain outside the file. When environment variables manage the whole config, `auth login` requires an explicit `--config` path to write. It ignores `ATLASSIAN_CONFIG`; that legacy override is for Jira and Confluence only. It drives a provider binary (`adfs-assume` by default, or `saml2aws`), or writes role-chaining profiles (`assume-role`); it is not AWS CLI itself, not a Portal API, not an MCP server, and not a browser SSO tool.

## Always Use JSON

For agents, use `--json` for every non-interactive `aws-auth` command:

```bash
aws-auth commands --json
aws-auth schema login --json
aws-auth account list --json
aws-auth login --account cps-dev --json
aws-auth status --json
aws-auth eks kubeconfig --account cps-dev --cluster cps-dev-eks --json
```

Only omit `--json` for human-facing interactive `aws-auth login`, where the command may prompt for an account and role that are not configured.

Read these fields first:

- `ok`
- `data`
- `error.code`
- `error.message`
- `error.hint`

If `ok=false`, inspect `error.code`, `error.message`, and `error.hint` before retrying.

## Account Matrix

The `aws` node lists the accounts an agent may use. Each account owns an AWS CLI profile named after it, so several accounts can be queried side by side:

```yaml
aws:
  enabled: true
  provider: adfs-assume          # adfs-assume | saml2aws | assume-role
  domain: HBEU
  username: "${AWS_AUTH_USERNAME}"
  password: "%AWS_AUTH_PASSWORD%"
  default_account: cps-dev
  default_region: ap-east-1
  accounts:
    - name: cps-dev
      account_id: "818354133892"
      role: ADFS-ReadOnly
      regions: [ap-east-1, eu-west-1]
    - name: dcc-dev
      account_id: "334430002784"
      role: ADFS-ReadOnly
      regions: [ap-east-1]
```

Start with:

```bash
aws-auth account list --json
```

`data.accounts[]` carries `name`, `account_id`, `role`, `role_arn`, `regions`, `profile`, `enabled`, and `default`.

## Login

Authorize one configured account (its role, region, and profile come from the matrix):

```bash
aws-auth login --account cps-dev --json
```

Authorize every enabled configured account in one call; the response is `partial=true` when some accounts failed and `data.results[]` says which:

```bash
aws-auth login --all --json
```

Authorize an account that is not in the matrix (the pre-matrix form); it writes the `saml` profile unless `--profile` is given:

```bash
aws-auth login --account 123456 --role ADFS-ReadOnly --profile saml --json
```

After a successful login `data.profile` names the AWS CLI profile. Use it explicitly on every later call:

```bash
aws --profile cps-dev sts get-caller-identity --output json
aws --profile cps-dev --region ap-east-1 ecr describe-repositories --output json
```

`login` verifies the new credentials with `aws sts get-caller-identity` and reports `data.verified` plus `data.identity`. Pass `--verify=false` to skip that. `data.expires_at` is present when the provider records the session expiry (saml2aws does).

Providers:

- `adfs-assume` (default): runs `adfs-assume --domain HBEU --username GB-SVC-XXX-XXX --role ADFS-ReadOnly --account 123456 --profile cps-dev --no-warning --display-token --jenkins`; the password travels in `AD_PASS` for that process only.
- `saml2aws`: runs `saml2aws login --idp-provider ADFS --url <aws.idp_url> --username HBEU\GB-SVC-XXX-XXX --role arn:aws:iam::<account_id>:role/<role> --profile cps-dev --skip-prompt --session-duration 3600 --disable-keychain`; the password travels in `SAML2AWS_PASSWORD`. `aws.idp_url` is required.
- `assume-role`: writes `[profile cps-dev]` with `role_arn` and `source_profile` (default `default`, or `aws.source_profile`) into the AWS config file; no password is needed. The source profile must already resolve credentials.

## Status

```bash
aws-auth status --json
aws-auth status --verify --json
```

`data.profiles[]` lists each configured account's profile with `present`, `expires_at`, `expired`, and `seconds_remaining` when the provider recorded an expiry. `--verify` calls STS for every present profile. When `aws` reports `ExpiredToken`, run `aws-auth login --account <name> --json` again.

## EKS

List clusters visible to an account, then write a kubectl context named `<account>/<cluster>`:

```bash
aws-auth eks list --account cps-dev --region ap-east-1 --json
aws-auth eks kubeconfig --account cps-dev --cluster cps-dev-eks --json
kubectl --context cps-dev/cps-dev-eks get pods -n payments
```

`eks kubeconfig` runs `aws eks update-kubeconfig` for the account's profile, writes to `--kubeconfig`, else `KUBECONFIG`, else `aws.kubeconfig_path` (default `~/.efp/kube/config`), and then checks `kubectl auth can-i list pods` (`data.can_list_pods`; `data.kubectl_missing=true` when kubectl is not installed). The context authenticates through `aws eks get-token`, so an expired SAML session surfaces as a kubectl authentication error: log in again.

Keep kubectl read-only: `get`, `describe`, `logs --tail`, `events`, `top`, `explain`, `auth can-i`. Never `apply`, `delete`, `edit`, `patch`, `scale`, `rollout`, `exec`, `port-forward`, or read secrets.

## Configure Auth

Store the ADFS domain, username, and password in the shared EFP config (the account matrix and other `aws` settings are preserved):

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

## Error Recovery

- `config_missing`: run `aws-auth auth login --password-stdin --json`, or pass `--config`; for `saml2aws` also set `aws.idp_url`.
- `account_required`: several accounts are configured; pass `--account <name>` from `data.candidates` or set `aws.default_account`.
- `unknown_account`: the name is not in the matrix; pick one from `data.candidates` or pass a numeric account id.
- `invalid_args`: inspect `aws-auth schema <command> --json`; provide required flags or stdin.
- `provider_missing`: the provider binary (`adfs-assume` or `saml2aws`) is not installed or not on `PATH`.
- `execution_failed`: the provider could not be run; read `error.message`.
- `auth_failed`: verify the domain, username, password, account, and role.
- `session_expired` / `credentials_missing`: run `aws-auth login --account <name> --json` again.
- `aws_cli_missing`: the AWS CLI is not installed in this runtime.
- `not_found`: run `aws-auth commands --json` and use a listed command name.

## Safety Rules

Never print, paste, log, or store raw passwords, access keys, or session tokens; `aws-auth` output redacts them and so must you.

Use `--dry-run --json` before troubleshooting command construction:

```bash
aws-auth login --account cps-dev --dry-run --json
aws-auth eks kubeconfig --account cps-dev --cluster cps-dev-eks --dry-run --json
```

Treat AWS operations after authentication as potentially destructive. Use `aws --output json` for inspection and avoid changing cloud resources unless the user explicitly asks.

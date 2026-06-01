# Configuration

Environment variable: `ATLASSIAN_CONFIG`

Default config path:

- Linux/macOS: `~/.config/atlassian/config.json`
- Windows: `%APPDATA%\\atlassian\\config.json`

## JSON structure

```json
{
  "version": 1,
  "jira": {
    "default_instance": "jira-main",
    "instances": []
  },
  "confluence": {
    "default_instance": "confluence-main",
    "instances": []
  }
}
```

### Jira instance fields

- `name`
- `base_url`
- `api_version`: `"2"`
- `rest_path`: `"/rest/api/2"`
- `auth.type`: `basic_password | basic_api_key | bearer_token`
- `auth.username`
- `auth.password`
- `auth.api_key`
- `auth.token`
- `default_project`
- `verify_ssl`
- `ca_cert`

### Confluence instance fields

- `name`
- `base_url`
- `rest_path`: `"/rest/api"`
- `auth` (same auth structure as Jira)
- `default_space`
- `verify_ssl`
- `ca_cert`


## TLS and CA behavior

- `verify_ssl=false` disables certificate verification and is intended only for internal testing.
- `ca_cert` can embed PEM text for private CA trust.

## Inspect Image Config

`inspect-image` does not use the Atlassian config file or schema above. It stores one combined config/auth file:

- Environment override: `INSPECT_IMAGE_CONFIG`
- Copilot home override: `COPILOT_HOME`
- Linux/macOS/Windows default: `~/.copilot/inspect-image.json`

The file contains provider/API defaults, image limits, auth tokens, and privacy settings. It is written with `0600` permissions where supported, and token values must be redacted from all command output.

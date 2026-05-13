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

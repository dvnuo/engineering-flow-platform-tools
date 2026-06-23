# Install

Download the release archive for your operating system and CPU architecture, extract it, and place `jira`, `confluence`, `jenkins`, `aws-auth`, `browser`, `mobile`, `inspect-image`, and `visual` on your `PATH`.

Verify the install:

```bash
jira version --json
confluence version --json
jenkins version --json
aws-auth version --json
browser version --json
mobile version --json
inspect-image version --json
visual version --json
```

`browser probe` uses Chrome by default and requires Chrome, Edge, or Chromium to be installed on the machine where it runs. Inside OpenCode runtime containers, a separate runtime image change is required to install a browser executable and place the `browser` binary on PATH.

`mobile` requires BrowserStack credentials for live calls. Set `BROWSERSTACK_USERNAME` and `BROWSERSTACK_ACCESS_KEY`. Private managed runs also require the BrowserStack Local binary on PATH or configured through `BROWSERSTACK_LOCAL_BINARY`; the CLI does not download it automatically.

The tools support linux, darwin, and windows on amd64 and arm64.

# Install

Download the release archive for your operating system and CPU architecture, extract it, and place `jira`, `confluence`, `jenkins`, `browser`, and `inspect-image` on your `PATH`.

Verify the install:

```bash
jira version --json
confluence version --json
jenkins version --json
browser version --json
inspect-image version --json
```

`browser probe` requires Edge, Chrome, or Chromium to be installed on the machine where it runs. Inside OpenCode runtime containers, a separate runtime image change is required to install a browser executable and place the `browser` binary on PATH.

The tools support linux, darwin, and windows on amd64 and arm64.

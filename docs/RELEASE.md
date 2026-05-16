# Release

Release builds are created from tags matching `v*`.

The release workflow builds linux, darwin, and windows binaries for amd64 and arm64. Archives are named with the repository, version, OS, and architecture, for example:

- `engineering-flow-platform-tools_0.1.0_linux_amd64.tar.gz`
- `engineering-flow-platform-tools_0.1.0_darwin_arm64.tar.gz`
- `engineering-flow-platform-tools_0.1.0_windows_amd64.zip`

Each archive includes `jira`, `confluence`, `README.md`, `docs/INSTALL.md`, `docs/CONFIG.md`, and `docs/LLM_USAGE.md`.

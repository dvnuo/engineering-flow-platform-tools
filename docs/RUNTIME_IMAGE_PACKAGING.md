# Runtime Image Packaging

This repository builds Go CLI tools. Runtime images should normally consume prebuilt binaries instead of compiling this repository inside the runtime Dockerfile.

Example Jenkins-style flow:

```bash
# In engineering-flow-platform-tools
bash scripts/build.sh --snapshot

# For an amd64 Linux runtime image
mkdir -p /path/to/engineering-flow-platform-opencode-runtime/runtime-tools
cp dist/linux-amd64/jira /path/to/engineering-flow-platform-opencode-runtime/runtime-tools/jira
cp dist/linux-amd64/confluence /path/to/engineering-flow-platform-opencode-runtime/runtime-tools/confluence
cp dist/linux-amd64/jenkins /path/to/engineering-flow-platform-opencode-runtime/runtime-tools/jenkins
cp dist/linux-amd64/browser /path/to/engineering-flow-platform-opencode-runtime/runtime-tools/browser
cp dist/linux-amd64/mobile-auto /path/to/engineering-flow-platform-opencode-runtime/runtime-tools/mobile-auto
cp dist/linux-amd64/inspect-image /path/to/engineering-flow-platform-opencode-runtime/runtime-tools/inspect-image

# BrowserStackLocal is a BrowserStack-provided binary, not built by this repo.
cp /secure/pipeline/browserstack/linux-amd64/BrowserStackLocal /path/to/engineering-flow-platform-opencode-runtime/runtime-tools/BrowserStackLocal

# Then build opencode runtime image
cd /path/to/engineering-flow-platform-opencode-runtime
docker build -t engineering-flow-platform-opencode-runtime:local .
```

For arm64 Linux runtime images, copy from:

```bash
dist/linux-arm64/jira
dist/linux-arm64/confluence
dist/linux-arm64/jenkins
dist/linux-arm64/browser
dist/linux-arm64/mobile-auto
dist/linux-arm64/inspect-image
```

For mobile automation inside native or OpenCode agents, the runtime should expose:

- `mobile-auto` on `PATH`
- `BrowserStackLocal` at `/usr/local/bin/BrowserStackLocal` when `private-managed` runs are allowed
- `EFP_CONFIG=/workspace/.efp/config.yaml`
- `MOBILE_AUTO_STATE_DIR=/workspace/.efp/mobile-auto/runs`
- `MOBILE_AUTO_ARTIFACTS_DIR=/workspace/.efp/mobile-auto/artifacts`
- `BROWSERSTACK_LOCAL_BINARY=/usr/local/bin/BrowserStackLocal`

Portal should project BrowserStack credentials, API/Appium proxy settings, Local mode, and Local proxy settings into the `mobile-auto` YAML node under `EFP_CONFIG`. Agents should start with `mobile-auto doctor --json` and `mobile-auto auth test --json`, then use `private-external` with an existing Local identifier or `private-managed` only when the runtime image includes BrowserStackLocal.

The runtime image should not need the Go toolchain if binaries are prepared externally.

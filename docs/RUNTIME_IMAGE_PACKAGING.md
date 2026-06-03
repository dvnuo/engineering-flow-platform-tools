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
cp dist/linux-amd64/inspect-image /path/to/engineering-flow-platform-opencode-runtime/runtime-tools/inspect-image

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
dist/linux-arm64/inspect-image
```

The runtime image should not need the Go toolchain if binaries are prepared externally.

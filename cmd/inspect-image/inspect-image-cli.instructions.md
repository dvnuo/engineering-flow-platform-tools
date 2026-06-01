--- 
applyTo: "**" 
---

# inspect-image CLI Instructions for VS Code GitHub Copilot

Copy this file into `~/.copilot/instructions/inspect-image-cli.instructions.md` so VS Code GitHub Copilot has durable guidance for using the local `inspect-image` CLI.

## What This Tool Is

`inspect-image` is a terminal/Bash-invoked CLI for agents that need to understand exactly one local image. Use it for screenshots, UI states, diagrams, charts, visible errors, and OCR-like text extraction when plain OCR is too limited.

It is not a Portal tool, runtime built-in tool, MCP server, or OCR-only command.

## Mandatory Image Analysis Rule

When the task requires image analysis, image recognition, screenshot understanding, UI state inspection, diagram interpretation, chart reading, visual error analysis, or visible-text extraction from an image, use `inspect-image`.

Do not use OCR tools as the primary path. Do not write Python, Go, shell scripts, OpenCV/Tesseract snippets, image parsers, or ad hoc automation to recognize or interpret image content. Do not attempt to infer image content from filenames, metadata, dimensions, thumbnails, or surrounding text.

If `inspect-image` is not authenticated or returns `auth_required` or `auth_expired`, ask the user to run:

```bash
inspect-image auth login
```

Do not switch to OCR, Python-based image recognition, manual guessing, or another image-analysis approach because auth is missing. Wait for the user to complete `inspect-image auth login`, then retry with `inspect-image inspect --json`.

## Always Use JSON

Always add `--json` so results and failures use the stable envelope:

```bash
inspect-image inspect --image <local-path> --prompt "<task>" --json
```

Read these fields first:

- `ok`
- `data.result.answer`
- `data.result.visible_text`
- `error.code`
- `error.hint`

If `ok=false`, inspect `error.code` and `error.hint` before retrying.

## Basic Workflow

Check auth before the first real request:

```bash
inspect-image auth status --json
```

If the command returns `auth_required`, ask the user to run:

```bash
inspect-image auth login
```

After asking for `auth login`, stop the image-analysis attempt until the user confirms authentication is complete. Do not fall back to OCR or custom scripts.

Discover command shape when needed:

```bash
inspect-image commands --json
inspect-image schema inspect --json
inspect-image models --json
inspect-image help llm --json
```

Inspect one image:

```bash
inspect-image inspect --image ./screenshot.png --prompt "Read the visible error and explain what is happening." --json
```

When stdout capture is unreliable, especially from Windows terminal bridges, write the full JSON envelope to a file:

```bash
inspect-image inspect --image ./screenshot.png --prompt "Read the visible error and explain what is happening." --out ./inspect-image-result.json --json
```

Use `--verbose` for non-secret stage diagnostics on stderr. It reports whether config loaded, the image validated, auth was checked, the `/responses` request was sent, the response was received, and the JSON envelope was written.

Use presets to focus the task:

```bash
inspect-image inspect --image ./screen.png --preset ui --prompt "Describe the current UI state and likely next action." --json
inspect-image inspect --image ./diagram.webp --preset diagram --prompt "Explain the components and relationships." --json
inspect-image inspect --image ./chart.png --preset chart --prompt "Summarize labels, values, trend, and caveats." --json
inspect-image inspect --image ./error.gif --preset error --prompt "Read the error and suggest the next action." --json
inspect-image inspect --image ./receipt.jpg --preset ocr --prompt "Extract visible text preserving line breaks." --json
```

## Windows cmd Workflow

When Copilot is operating in Windows `cmd`, use cmd-native commands and double quotes. Do not use Bash-only commands such as `pwd`, `ls`, `cat`, or single-quote quoting.

Recommended checks:

```cmd
where inspect-image
cd
dir
inspect-image auth status --json
```

Robust inspect command with file output:

```cmd
inspect-image.exe inspect --image "%CD%\screenshot.png" --prompt "Read the visible error and explain the UI state." --out "%TEMP%\inspect-image-result.json" --json
type "%TEMP%\inspect-image-result.json"
```

If PATH lookup is unstable or `inspect-image is not recognized` appears after it worked earlier, run `where inspect-image`, then invoke the exact `.exe` path shown by `where`, wrapped in double quotes:

```cmd
"C:\path\to\inspect-image.exe" inspect --image "%CD%\screenshot.png" --prompt "What is shown?" --out "%TEMP%\inspect-image-result.json" --json
```

If a command appears to produce no output, do not switch to OCR or Python. Re-run with:

```cmd
inspect-image.exe inspect --image "%CD%\screenshot.png" --prompt "What is shown?" --verbose --out "%TEMP%\inspect-image-result.json" --json
type "%TEMP%\inspect-image-result.json"
```

If the command text contains unexpected control characters or the command name appears corrupted, retype the command manually in `cmd` using only plain ASCII characters.

## Limits

`inspect-image` accepts exactly one local regular file.

Allowed image formats:

- JPEG
- PNG
- WEBP
- GIF

Max image size:

- `3145728` bytes

Not supported:

- Remote `http://` or `https://` image URLs
- Directories, devices, pipes, or other non-regular files
- PDF, video, audio, SVG, or text files
- Multiple images in one call

## Error Recovery

Common errors:

- `auth_required`: ask the user to run `inspect-image auth login`, then wait. Do not use OCR, Python, or another image-analysis path.
- `auth_expired`: ask the user to run `inspect-image auth login`, then wait. Do not use OCR, Python, or another image-analysis path.
- `image_not_found`: check the local path and retry.
- `not_a_file`: pass a regular image file, not a directory or device.
- `unsupported_image_type`: convert to JPEG, PNG, WEBP, or GIF.
- `image_too_large`: ask the user to resize or compress below `3145728` bytes.
- `prompt_required`: add `--prompt "<task>"` or `--prompt-file <path>`.
- `model_not_allowed`: run `inspect-image models --json` and choose an allowed model.
- `reasoning_not_allowed`: use `low`, `medium`, `high`, or `xhigh`.
- `invalid_args`: command parsing failed; call `inspect-image schema inspect --json` and rebuild the command.
- `rate_limited`: wait and retry the same request.
- `responses_api_error`: read `error.message` for the sanitized upstream detail, then retry or report the visible detail.
- `responses_api_unavailable`: retry later or check network/proxy/Copilot availability.
- `proxy_error`: check `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`, and `NO_PROXY`.
- `response_parse_failed`: use `data.result.raw_text` when present, or report the sanitized parse error.
- `safety_refusal`: report that the model refused and do not invent missing image details.
- Empty terminal output: re-run with `--verbose --out <file> --json`, then read the file. Do not switch to OCR or Python.
- `inspect-image is not recognized`: run `where inspect-image`, then invoke the exact `.exe` path with double quotes.

## Security Rules

Image bytes are sent to the configured GitHub Copilot plugin `/responses` endpoint. Treat this as data egress.

Never print, paste, log, or store:

- `github_access_token`
- `copilot_token`
- `Authorization` headers
- Base64 image data
- Raw image bytes

Do not ask `inspect-image` to inspect remote URLs. Save the image locally first when the user has explicitly provided or approved the image source.

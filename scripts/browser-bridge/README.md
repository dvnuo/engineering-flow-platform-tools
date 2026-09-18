# Browser bridge (EFP Portal local browser connector)

This directory holds the non-binary files of the bridge package that the
Portal offers for download, plus the scripts used to verify a workstation
before rolling the connector out to it. The bridge itself is the
`browser serve` subcommand of the `browser` CLI in this repository; nothing
here replaces it.

## Download package layout

`package.sh` builds one zip per platform, `efp-browser-bridge-<os>-<arch>.zip`
(`windows-amd64`, `windows-arm64`, `darwin-arm64`, `darwin-amd64`, `linux-amd64`,
`linux-arm64`), holding only what a member needs to install the bridge:

| Platform | Binary | Installer | Docs |
|---|---|---|---|
| Windows | `browser.exe` | `install-bridge.cmd` | `README.md` (`PACKAGE_README.md` here) |
| macOS | `browser` | `install-bridge.sh` | `README.md` |
| Linux | `browser` | `install-bridge.sh` | `README.md` |

```bash
scripts/browser-bridge/package.sh                       # all six, into dist/bridge/
scripts/browser-bridge/package.sh --os darwin --arch arm64 --version 0.2.0
```

The release workflow runs it for every target, uploads the zips with the
build artifacts, and attaches them to the GitHub release of the tag, so a
Portal can point `LOCAL_BROWSER_CLI_DOWNLOAD_URL` at
`https://github.com/<org>/engineering-flow-platform-tools/releases/download/<tag>/efp-browser-bridge-{platform}.zip`
(`{platform}` is filled in per member) or copy the zips into its
`app/static/downloads/`, the fallback location. The Portal panel offers the
member's own system first and lists the others. The verify scripts below are
not part of the packages; `zip(1)` is needed to keep the execute bits of the
macOS and Linux packages (the script falls back to Python's `zipfile` where
`zip` is missing, which is enough for the Windows package).

The installer runs `browser serve --register-protocol --origin <portal-origin>`
relative to its own directory, which registers the `efp-bridge://` link for
the current user only and stores the origin as `browser.serve.allowed_origin`:

- Windows: `HKCU\Software\Classes\efp-bridge` pointing at `browser.exe bridge-launch "%1"`.
- macOS: a small `~/Applications/EFP Bridge.app` (built with `osacompile`) that owns
  the `efp-bridge` URL scheme and runs `browser bridge-launch "<url>"`.
- Linux: `~/.local/share/applications/efp-bridge.desktop` registered through
  `xdg-mime default efp-bridge.desktop x-scheme-handler/efp-bridge`.

No administrator rights and no autostart entry are needed on any platform.

Install steps for a user:

1. Unpack the package into the `bin` folder of the home directory (`%USERPROFILE%\bin`
   on Windows, `~/bin` elsewhere), creating it when missing and overwriting files
   already there.
2. Run `install-bridge.cmd https://portal.example.com` (Windows) or
   `./install-bridge.sh https://portal.example.com` (macOS, Linux) from that folder.
3. On the Portal Connectors page click **Start bridge**. The page opens
   `efp-bridge://start?origin=<portal origin>&port=8765`; the operating system runs
   `browser bridge-launch "<url>"`, which starts `browser serve --origin <origin> --port 8765`
   in the background (or does nothing when a bridge already answers `GET /ping`).
4. Click **Test connection**. A Chrome window with a dedicated profile opens; sign in to
   the Portal inside that window once.

To remove the handler run `browser serve --unregister-protocol`. To start the bridge
without the protocol link run `browser serve --origin https://portal.example.com`.
The bridge listens on `127.0.0.1:8765` (falling back to `8766`-`8770`), logs one line
per request to stderr, and leaves the Chrome session running when it stops.

## Verify scripts

`efp-bridge-verify.ps1` (Windows; `efp-bridge-verify.bat` wraps it with the execution
policy bypassed) and `efp-bridge-verify.sh` (macOS, Linux) check on the current machine
that the whole chain works. They need `browser` on `PATH` and, for the shell version,
`curl`:

```powershell
.\efp-bridge-verify.ps1 -PortalUrl https://portal.example.com
.\efp-bridge-verify.bat https://portal.example.com -SkipLogin -StopSession
```

```bash
./efp-bridge-verify.sh https://portal.example.com --skip-login --stop-session
```

Four checks: the CLI runs, `browser serve` starts and Chrome opens with a DevTools port
and a dedicated profile (the `RemoteDebuggingAllowed` policy test), a page served from
the Portal origin can call `127.0.0.1` (CORS plus the private-network preflight), and the
bridge executes `tab.list` end to end while answering the preflight and refusing foreign
origins. After the second check the script pauses so you can sign in inside the EFP
window and note how SSO behaves there (`-SkipLogin` / `--skip-login` skips it).

How to read a failure: check 1 means user-directory executables are blocked
(AppLocker/WDAC on Windows, Gatekeeper quarantine on macOS); check 2 means Chrome refused
a DevTools port (`RemoteDebuggingAllowed` policy); check 3 means the page cannot reach
loopback and the bridge would need an outbound connection instead; check 4 is a bridge
bug or a port conflict.

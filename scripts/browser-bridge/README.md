# Browser bridge (EFP Portal local browser connector)

This directory holds the pieces that ship in the Portal download package and the
scripts used to verify the local bridge on a Windows workstation.

## Download package layout

The Portal serves `efp-browser-bridge.zip` (see `LOCAL_BROWSER_CLI_DOWNLOAD_URL`
in the Portal). It contains exactly three files:

| File | Purpose |
|---|---|
| `browser.exe` | The `browser` CLI from this repository (`scripts/build.sh --os windows --arch amd64` produces `dist/windows-amd64/browser.exe`). |
| `install-bridge.cmd` | One-click registration: runs `browser.exe serve --register-protocol --origin <portal-origin>` relative to its own directory, stores the origin as `browser.serve.allowed_origin`, and prints the next steps. |
| `README.md` | This file. |

Install steps for a user:

1. Unzip the package into a folder that stays put (for example `%LOCALAPPDATA%\efp\browser-bridge`).
2. Run `install-bridge.cmd https://portal.example.com` from that folder. No administrator rights are needed; the handler is written under `HKCU\Software\Classes\efp-bridge`.
3. On the Portal Connectors page click **Start bridge**. The page opens `efp-bridge://start?origin=<portal origin>&port=8765`; Windows runs `browser.exe bridge-launch "<url>"`, which starts `browser serve --origin <origin> --port 8765` in the background (or does nothing when a bridge already answers `GET /ping`).
4. Click **Test connection**. A Chrome window with a dedicated profile opens; log in to the Portal inside that window once.

To remove the handler run `browser.exe serve --unregister-protocol`. To start the
bridge without the protocol link run `browser.exe serve --origin https://portal.example.com`.
The bridge listens on `127.0.0.1:8765` (falling back to `8766`-`8770`), logs one line
per request to stderr, and leaves the Chrome session running when it stops.

## Verify script

`efp-bridge-verify.ps1` checks, on the current machine, that the whole chain works:

```powershell
.\efp-bridge-verify.ps1 -PortalUrl https://portal.example.com
.\efp-bridge-verify.ps1 -PortalUrl https://portal.example.com -SkipLogin -StopSession
```

It requires `browser` on `PATH` and Python 3, and runs five checks: the CLI starts,
Chrome starts with a debug port and a dedicated profile (the `RemoteDebuggingAllowed`
policy test), CDP can list tabs and read a page, the `https` Portal page can call
`127.0.0.1` (CORS plus the private-network preflight), and an end-to-end
page -> local service -> CLI -> Chrome round trip. Step 4 and 5 use
`efp_bridge_probe.py`, a stand-alone Python prototype of the bridge that shells out to
the CLI; it exists only for verification and is not the product bridge
(`browser serve` is). `start-bridge.cmd` is the prototype protocol-handler launcher
used before `bridge-launch` existed; edit the three `set` lines before using it.

Interpretation printed by the verify script: step 1 failing means user-directory
executables are blocked (AppLocker/WDAC); step 2 failing means remote debugging is
disabled by policy; step 4 failing means the page cannot reach loopback and the
bridge must use an outbound connection instead.

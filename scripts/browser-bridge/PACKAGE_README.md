# EFP browser bridge

This package lets the assistants in the EFP Portal read and operate pages in a
Chrome window on this computer, with your own logins. It contains only what the
install needs:

- `browser` (`browser.exe` on Windows): the EFP browser program; the Portal uses
  its `serve` bridge, which listens on `127.0.0.1` only.
- `install-bridge.cmd` (Windows) or `install-bridge.sh` (macOS, Linux): registers
  the `efp-bridge://` link for your user account. No administrator rights, no
  autostart entry.
- this file.

## Install

1. Unzip into a folder that stays put, for example `%LOCALAPPDATA%\efp\browser-bridge`
   on Windows or `~/efp/browser-bridge` on macOS and Linux.
2. Run the installer from that folder with your Portal address (the Portal's
   Connectors page shows the exact command):
   - Windows: double-click `install-bridge.cmd` and enter the address when asked,
     or run `install-bridge.cmd https://portal.example.com`.
   - macOS, Linux: `./install-bridge.sh https://portal.example.com`
     (run `chmod +x install-bridge.sh browser` first if the files are not executable).
3. Back on the Portal Connectors page click **Start bridge** and allow the
   `efp-bridge` link when the browser asks. A Chrome window titled EFP opens;
   sign in to your work sites there once.
4. Click **Test connection**, then switch the connector on.

## If something blocks it

- Windows SmartScreen may warn the first time `browser.exe` runs: choose
  "More info", then "Run anyway".
- macOS Gatekeeper may refuse the downloaded `browser`: the installer clears the
  quarantine flag; if it still refuses, run `xattr -d com.apple.quarantine browser`.
- Nothing happens after Start bridge: every attempt is written to
  `.efp/browser/logs/bridge-serve.log` in your home folder.

## Remove or start by hand

- `browser serve --unregister-protocol` removes the `efp-bridge://` handler.
- `browser serve --origin https://portal.example.com` starts the bridge without
  the link (keep the window open; Ctrl+C stops it).

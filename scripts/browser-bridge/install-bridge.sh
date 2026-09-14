#!/usr/bin/env bash
# install-bridge.sh - register the efp-bridge:// protocol handler for the EFP
# Portal local browser connector on macOS or Linux, using the `browser` binary
# that sits next to this script (the Portal download package for these
# platforms contains browser, install-bridge.sh, and README.md).
#
# Usage:  ./install-bridge.sh https://portal.example.com
#         (without an argument it asks for the address)
#
# No administrator rights are needed. macOS gets a small "EFP Bridge.app" in
# ~/Applications that owns the URL scheme; Linux gets
# ~/.local/share/applications/efp-bridge.desktop registered with xdg-mime.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ORIGIN="${1:-}"
BROWSER_BIN="$HERE/browser"

if [ -z "$ORIGIN" ] && [ -t 0 ]; then
  echo "This installer needs the address of your EFP Portal, for example https://portal.example.com"
  echo "(the Portal's Connectors page shows the exact address)."
  printf 'Portal address: '
  read -r ORIGIN
fi
if [ -z "$ORIGIN" ]; then
  echo "Usage: install-bridge.sh <portal-origin>" >&2
  echo "Example: install-bridge.sh https://portal.example.com" >&2
  exit 2
fi
if [ ! -f "$BROWSER_BIN" ]; then
  echo "browser was not found next to this script: $BROWSER_BIN" >&2
  echo "Unpack the whole package and run install-bridge.sh from that folder." >&2
  exit 1
fi
chmod +x "$BROWSER_BIN" 2>/dev/null || true
if [ "$(uname -s)" = "Darwin" ]; then
  # A downloaded binary carries the quarantine flag; Gatekeeper would refuse it.
  xattr -d com.apple.quarantine "$BROWSER_BIN" 2>/dev/null || true
fi

echo "Registering the efp-bridge:// protocol handler for $ORIGIN ..."
"$BROWSER_BIN" serve --register-protocol --origin "$ORIGIN" --json

cat <<EOF

Next steps:
  1. Go back to the Portal Connectors page and click "Start bridge"
     (it opens efp-bridge://start?origin=...&port=8765).
  2. Allow the "EFP Bridge" prompt once if your browser shows one. The bridge
     then starts browser serve in the background.
  3. Click "Test connection" on the Portal page. A Chrome window with a
     dedicated profile opens; sign in to the Portal inside that window once.
  4. Turn on the Browser toggle in the chat composer when you want the
     assistant to use your local browser for that chat.

Manual start (if the protocol link is blocked):
  "$BROWSER_BIN" serve --origin "$ORIGIN"
Remove the handler later with:
  "$BROWSER_BIN" serve --unregister-protocol
EOF

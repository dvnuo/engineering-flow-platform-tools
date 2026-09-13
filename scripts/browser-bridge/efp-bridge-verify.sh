#!/usr/bin/env bash
# efp-bridge-verify.sh - verify the EFP browser bridge (browser serve) on a
# macOS or Linux workstation. Mirrors efp-bridge-verify.ps1.
#
# Requires the `browser` CLI from engineering-flow-platform-tools on PATH and
# curl. Checks:
#   1. the browser CLI runs
#   2. `browser serve` starts and Chrome opens with a DevTools port and a
#      dedicated profile (the RemoteDebuggingAllowed policy test)
#   3. a page served from the Portal origin can call 127.0.0.1
#      (CORS plus the private-network preflight Chrome requires)
#   4. the bridge executes commands end to end (POST /run tab.list), answers
#      the preflight, and refuses requests from other origins
#
# Usage: efp-bridge-verify.sh <portal-url> [--port 8765] [--skip-login] [--stop-session]
set -u

PORTAL_URL=""
PORT=8765
SKIP_LOGIN=0
STOP_SESSION=0
while [ $# -gt 0 ]; do
  case "$1" in
    --port) PORT="$2"; shift 2 ;;
    --skip-login) SKIP_LOGIN=1; shift ;;
    --stop-session) STOP_SESSION=1; shift ;;
    -h|--help) sed -n '2,15p' "$0"; exit 0 ;;
    *) PORTAL_URL="$1"; shift ;;
  esac
done
if [ -z "$PORTAL_URL" ]; then
  echo "Usage: efp-bridge-verify.sh <portal-url> [--port 8765] [--skip-login] [--stop-session]" >&2
  exit 2
fi
PORTAL_ORIGIN="$(printf '%s' "$PORTAL_URL" | sed -E 's#^((https?)://[^/]+).*#\1#')"

LOG_DIR="${TMPDIR:-/tmp}/efp-bridge-verify"
mkdir -p "$LOG_DIR"
SERVE_LOG="$LOG_DIR/serve.log"
SERVE_PID=""
BRIDGE_PORT=""
RESULTS=()

pass() { RESULTS+=("PASS | $1 | $2"); printf '\033[32m[PASS]\033[0m %s - %s\n' "$1" "$2"; }
fail() { RESULTS+=("FAIL | $1 | $2"); printf '\033[31m[FAIL]\033[0m %s - %s\n' "$1" "$2"; }
json_has() { printf '%s' "$1" | tr -d ' \n' | grep -q "$2"; }
json_field() { printf '%s' "$1" | tr -d '\n' | sed -E "s/.*\"$2\":\"?([^\",}]*)\"?.*/\1/"; }

cleanup() {
  if [ -n "$SERVE_PID" ] && kill -0 "$SERVE_PID" 2>/dev/null; then kill "$SERVE_PID" 2>/dev/null; fi
  if [ "$STOP_SESSION" = "1" ]; then browser session stop default --json >/dev/null 2>&1 || true; fi
  echo
  printf '%s\n' "${RESULTS[@]}"
  echo
  echo "How to read this: step 1 failing means the binary cannot run (on macOS remove the quarantine flag: xattr -d com.apple.quarantine browser); step 2 failing means Chrome refused a DevTools port (RemoteDebuggingAllowed policy); step 3 failing means the page cannot reach loopback; step 4 failing is a bridge bug or a port conflict. Logs: $LOG_DIR"
}
trap cleanup EXIT

ping_bridge() { curl -s --max-time 3 -H "Origin: $PORTAL_ORIGIN" "http://127.0.0.1:$1/ping" 2>/dev/null; }

# ---- 1. CLI runs ---------------------------------------------------------------
VERSION_OUT="$(browser version --json 2>/dev/null)"
if json_has "$VERSION_OUT" '"ok":true'; then
  pass "1 browser CLI runs" "version $(json_field "$VERSION_OUT" version)"
else
  fail "1 browser CLI runs" "browser version --json did not return ok:true"
  exit 1
fi

# ---- 2. browser serve starts, Chrome opens with a DevTools port -----------------
browser serve --origin "$PORTAL_ORIGIN" --port "$PORT" --url "$PORTAL_URL" --session default --json >"$SERVE_LOG" 2>&1 &
SERVE_PID=$!
PING=""
for _ in $(seq 1 60); do
  sleep 0.5
  for candidate in $(seq "$PORT" $((PORT + 5))); do
    out="$(ping_bridge "$candidate")"
    if json_has "$out" '"ok":true'; then PING="$out"; BRIDGE_PORT="$candidate"; break; fi
  done
  [ -n "$PING" ] && break
  kill -0 "$SERVE_PID" 2>/dev/null || break
done
if [ -z "$PING" ]; then
  fail "2 browser serve starts with Chrome" "no /ping answer within 30 s; log: $(tr -s ' \n' ' ' <"$SERVE_LOG" | cut -c1-300)"
  exit 1
fi
for _ in $(seq 1 20); do
  json_has "$PING" '"alive":true' && break
  sleep 0.5
  PING="$(ping_bridge "$BRIDGE_PORT")"
done
if json_has "$PING" '"alive":true'; then
  pass "2 browser serve starts with Chrome" "bridge v$(json_field "$PING" version) on port $BRIDGE_PORT, DevTools port $(json_field "$PING" debug_port)"
else
  fail "2 browser serve starts with Chrome" "bridge is up on port $BRIDGE_PORT but the Chrome session is not alive: $(browser session status default --json 2>/dev/null | tr -d '\n' | cut -c1-300)"
  exit 1
fi

# ---- login pause ------------------------------------------------------------------
if [ "$SKIP_LOGIN" != "1" ]; then
  echo
  echo "Sign in to the Portal inside the EFP browser window that just opened, then open one work site in a new tab."
  read -r -p "Press Enter here when done " _
fi

# ---- 3. Portal page -> 127.0.0.1 ----------------------------------------------------
TABS="$(browser tab list --json 2>/dev/null)"
TARGET_ID="$(printf '%s' "$TABS" | tr -d '\n' | grep -o "{[^{}]*\"url\":\"$PORTAL_ORIGIN[^{}]*}" | head -1 | sed -E 's/.*"id":"([^"]+)".*/\1/')"
if [ -n "$TARGET_ID" ]; then browser tab activate --target-id "$TARGET_ID" --json >/dev/null 2>&1; fi
FETCH="$(browser page fetch --url "http://127.0.0.1:$BRIDGE_PORT/ping" --json 2>/dev/null)"
if json_has "$FETCH" '"ok":true' && json_has "$FETCH" '"status":200'; then
  pass "3 page can call the bridge" "HTTP 200 from the Portal tab to 127.0.0.1:$BRIDGE_PORT (CORS and private-network preflight accepted)"
else
  fail "3 page can call the bridge" "$(printf '%s' "$FETCH" | tr -d '\n' | cut -c1-300)"
fi

# ---- 4. bridge executes commands, preflight, foreign origin ------------------------
PRE_HEADERS="$(curl -s -i -X OPTIONS --max-time 5 -H "Origin: $PORTAL_ORIGIN" -H "Access-Control-Request-Method: POST" -H "Access-Control-Request-Private-Network: true" "http://127.0.0.1:$BRIDGE_PORT/run")"
PRE_OK=0
printf '%s' "$PRE_HEADERS" | grep -qi "^Access-Control-Allow-Origin: $PORTAL_ORIGIN" && printf '%s' "$PRE_HEADERS" | grep -qi "^Access-Control-Allow-Private-Network: true" && PRE_OK=1
FOREIGN_CODE="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 -H "Origin: https://evil.example.test" "http://127.0.0.1:$BRIDGE_PORT/ping")"
RUN="$(curl -s --max-time 40 -X POST -H "Origin: $PORTAL_ORIGIN" -H "Content-Type: application/json" -d '{"command":"tab.list","params":{},"session":"default","timeout_seconds":20}' "http://127.0.0.1:$BRIDGE_PORT/run")"
RUN_OK=0
json_has "$RUN" '"ok":true' && json_has "$RUN" '"tabs":\[' && RUN_OK=1
if [ "$RUN_OK" = "1" ] && [ "$PRE_OK" = "1" ] && [ "$FOREIGN_CODE" = "403" ]; then
  pass "4 bridge executes commands" "POST /run tab.list ok; preflight headers present; foreign origin rejected with 403"
else
  fail "4 bridge executes commands" "run ok=$RUN_OK preflight ok=$PRE_OK foreign origin status=$FOREIGN_CODE; run body: $(printf '%s' "$RUN" | tr -d '\n' | cut -c1-200)"
fi

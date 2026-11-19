#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
#
# UI-3: the API-only binary serves no UI (/ is 404, /healthz is 200); the
# bundled binary serves index.html at / and falls back to it for client
# routes such as /setups, while unknown API paths stay JSON 404.
set -euo pipefail

api_bin="$1"
ui_bin="$2"
tmp="${TEST_TMPDIR:-$(mktemp -d)}"
pid=""
cleanup() { [ -n "$pid" ] && kill "$pid" 2>/dev/null && wait "$pid" 2>/dev/null || true; }
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; [ -f "$tmp/log" ] && tail -n 20 "$tmp/log" >&2; exit 1; }

# start <binary>: run it on a free-ish local port; sets $base once /healthz answers.
start() {
  local bin="$1" try port
  for try in 1 2 3 4 5; do
    port=$((20000 + RANDOM % 20000))
    rm -f "$tmp/h.db" "$tmp/h.db-"*
    HOURGLASS_LISTEN="127.0.0.1:$port" HOURGLASS_DATABASE="$tmp/h.db" \
      HOURGLASS_SESSION_KEY="0123456789abcdef0123456789abcdef" \
      "$bin" >"$tmp/log" 2>&1 &
    pid=$!
    for _ in $(seq 1 100); do
      kill -0 "$pid" 2>/dev/null || break
      if [ "$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$port/healthz" || true)" = 200 ]; then
        base="http://127.0.0.1:$port"
        return 0
      fi
      sleep 0.1
    done
    cleanup; pid=""
  done
  fail "$bin did not start"
}

code() { curl -s -o /dev/null -w '%{http_code}' "$base$1"; }

# API-only binary.
start "$api_bin"
[ "$(code /healthz)" = 200 ] || fail "api-only: /healthz"
[ "$(code /)" = 404 ] || fail "api-only: / should be 404, got $(code /)"
[ "$(code /setups)" = 404 ] || fail "api-only: /setups should be 404"
grep -q "built without it" "$tmp/log" || fail "api-only: log does not say the UI is absent"
cleanup; pid=""

# Bundled binary.
start "$ui_bin"
[ "$(code /healthz)" = 200 ] || fail "bundled: /healthz"
body="$(curl -s "$base/")"
case "$body" in *"<html"*) ;; *) fail "bundled: / is not HTML: $body" ;; esac
[ "$(curl -s -D - -o /dev/null "$base/" | tr -d '\r' | grep -ci '^content-type: text/html')" = 1 ] || fail "bundled: / content type"
[ "$(curl -s "$base/setups")" = "$body" ] || fail "bundled: /setups did not fall back to index.html"
[ "$(code /api/v1/nope)" = 404 ] || fail "bundled: unknown API path should be 404"
case "$(curl -s "$base/api/v1/nope")" in *"<html"*) fail "bundled: unknown API path returned HTML" ;; esac
[ "$(code /auth/nope)" = 404 ] || fail "bundled: unknown auth path should be 404"
asset="$(printf '%s' "$body" | grep -o '/assets/[^"]*\.js' | head -n 1)"
[ -n "$asset" ] || fail "bundled: index.html names no script under /assets/"
[ "$(code "$asset")" = 200 ] || fail "bundled: $asset not served"
grep -q "serving the bundled UI" "$tmp/log" || fail "bundled: log does not say the UI is served"
cleanup; pid=""

# ui.serve=false on the bundled binary: split mode, nothing at /.
HOURGLASS_UI_SERVE=false start "$ui_bin"
[ "$(code /)" = 404 ] || fail "ui.serve=false: / should be 404"
[ "$(code /healthz)" = 200 ] || fail "ui.serve=false: /healthz"
grep -q "ui.serve is false" "$tmp/log" || fail "ui.serve=false: log does not say so"
echo PASS

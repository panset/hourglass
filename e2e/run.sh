#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Headless-browser acceptance test against the local stacks. Needs Docker, Bazel (to load images), node and npm.
# Not part of `bazel test //...`.
#
#   e2e/run.sh <bundled|split|both>
#
# Loads the images, starts the stack, runs e2e/browser (Playwright, Chromium), collects the Playwright report,
# screenshots, traces and container logs into e2e/out/<mode>/ and always tears the stack down (volume included).
# Exit status is the test's. Each mode takes roughly 15 to 25 minutes (collector polls are a minute apart).
set -uo pipefail

mode="${1:-}"
case "$mode" in bundled | split | both) ;; *) echo "usage: $0 <bundled|split|both>" >&2; exit 2 ;; esac

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
loaded=0

run_mode() {
  local m="$1" compose="deploy/compose.$1.yaml" out="$root/e2e/out/$1" rc=0
  rm -rf "$out"; mkdir -p "$out"
  echo "== [$m] start $(date +%H:%M:%S)"
  local t0=$SECONDS
  docker compose -f "$compose" down -v --remove-orphans >/dev/null 2>&1 # fresh volume, no leftovers from a failed run
  if [ "$loaded" = 0 ]; then
    deploy/load-images.sh >"$out/load-images.log" 2>&1 || { echo "image load failed, see $out/load-images.log" >&2; return 1; }
    loaded=1
  fi
  if ! docker compose -f "$compose" up -d --wait >"$out/up.log" 2>&1; then
    echo "stack did not become ready, see $out/up.log" >&2
    rc=1
  else
    (cd e2e/browser && E2E_MODE="$m" E2E_OUT="$out" npx playwright test) 2>&1 | tee "$out/playwright.log"
    rc=${PIPESTATUS[0]}
  fi
  docker compose -f "$compose" logs --no-color >"$out/containers.log" 2>&1
  docker compose -f "$compose" down -v --remove-orphans >"$out/down.log" 2>&1
  echo "== [$m] exit $rc after $((SECONDS - t0)) s (artifacts in e2e/out/$m)"
  echo "$((SECONDS - t0))" >"$out/seconds.txt"
  return "$rc"
}

# Playwright's Chromium: installed once and cached (npx playwright install chromium); skipped when it already launches.
chromium_ok() { (cd e2e/browser && node -e "require('@playwright/test').chromium.launch().then(b => b.close()).catch(() => process.exit(1))") >/dev/null 2>&1; }
if ! chromium_ok; then
  (cd e2e/browser && { [ -d node_modules ] || npm ci; } && npx playwright install chromium) \
    || { echo "could not install Playwright or Chromium (see e2e/browser/README note in tasks/35-e2e/RESULT.md)" >&2; exit 1; }
  chromium_ok || { echo "Chromium installed but does not launch" >&2; exit 1; }
fi

status=0
case "$mode" in
  both) run_mode bundled || status=1; run_mode split || status=1 ;;
  *) run_mode "$mode" || status=1 ;;
esac
exit "$status"

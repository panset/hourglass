#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Headless-browser acceptance test against the local stacks. Needs Docker, Bazel (to load images), node and npm.
# Not part of `bazel test //...`.
#
#   e2e/run.sh <bundled|split|both|integration>
#
# Loads the images, starts the stack (deploy/compose.<mode>.yaml with the test overrides e2e/compose.<mode>.yaml:
# the *-e2e images and the clock control port), runs e2e/browser (Playwright, Chromium), collects the Playwright
# report, screenshots, traces and container logs into e2e/out/<mode>/ and always tears the stack down (volume
# included). Exit status is the test's. Each mode takes roughly 15 to 25 minutes (collector polls are a minute apart).
# `integration` runs the bundled stack with the `reported` extensions (e2e/compose.integration.yaml) and the scripted
# reporter scenario (e2e/browser/tests/integration.spec.ts) instead of the acceptance path; `both` does not include it.
# What it needs and how it behaves: e2e/README.md. Installs the npm packages when e2e/browser/node_modules is
# missing and downloads Chromium only when neither it nor system Chrome already launches.
set -uo pipefail

mode="${1:-}"
case "$mode" in bundled | split | both | integration) ;; *) echo "usage: $0 <bundled|split|both|integration>" >&2; exit 2 ;; esac

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
loaded=0

run_mode() {
  local m="$1" out="$root/e2e/out/$1" rc=0
  local compose=(docker compose -f "deploy/compose.$1.yaml" -f "e2e/compose.$1.yaml") spec=acceptance
  if [ "$1" = integration ]; then
    compose=(docker compose -f deploy/compose.bundled.yaml -f e2e/compose.bundled.yaml -f e2e/compose.integration.yaml)
    spec=integration
  fi
  rm -rf "$out"; mkdir -p "$out"
  echo "== [$m] start $(date +%H:%M:%S)"
  local t0=$SECONDS
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 # fresh volume, no leftovers from a failed run
  if [ "$loaded" = 0 ]; then
    deploy/load-images.sh >"$out/load-images.log" 2>&1 || { echo "image load failed, see $out/load-images.log" >&2; return 1; }
    loaded=1
  fi
  if ! "${compose[@]}" up -d --wait >"$out/up.log" 2>&1; then
    echo "stack did not become ready, see $out/up.log" >&2
    rc=1
  else
    (cd e2e/browser && E2E_MODE="$m" E2E_OUT="$out" npx playwright test "$spec") 2>&1 | tee "$out/playwright.log"
    rc=${PIPESTATUS[0]}
  fi
  "${compose[@]}" logs --no-color >"$out/containers.log" 2>&1
  "${compose[@]}" down -v --remove-orphans >"$out/down.log" 2>&1
  echo "== [$m] exit $rc after $((SECONDS - t0)) s (artifacts in e2e/out/$m)"
  echo "$((SECONDS - t0))" >"$out/seconds.txt"
  return "$rc"
}

# Browser: install the npm packages when missing; use Playwright's Chromium if it launches, else system Chrome
# (the "chrome" channel, exported as E2E_BROWSER_CHANNEL); download Chromium only when neither launches.
launches() { # launches [channel]
  (cd e2e/browser && node -e "require('@playwright/test').chromium.launch(process.argv[1] ? {channel: process.argv[1]} : {}).then(b => b.close()).catch(() => process.exit(1))" "${1:-}") >/dev/null 2>&1
}
if [ ! -d e2e/browser/node_modules ]; then
  echo "== npm ci in e2e/browser (node_modules is missing)"
  (cd e2e/browser && npm ci) || { echo "npm ci failed in e2e/browser; see e2e/README.md" >&2; exit 1; }
fi
if ! launches; then
  if launches chrome; then
    echo "== using system Chrome (Playwright's Chromium is not installed)"
    export E2E_BROWSER_CHANNEL=chrome
  else
    echo "== downloading Chromium (npx playwright install chromium)"
    (cd e2e/browser && npx playwright install chromium) \
      || { echo "could not download Chromium; the download can hang while extracting, see 'Known problem' in e2e/README.md" >&2; exit 1; }
    launches || { echo "Chromium is installed but does not launch; see e2e/README.md" >&2; exit 1; }
  fi
fi

status=0
case "$mode" in
  both) run_mode bundled || status=1; run_mode split || status=1 ;;
  *) run_mode "$mode" || status=1 ;;
esac
exit "$status"

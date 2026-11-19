#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Fails listing tracked/untracked-not-ignored source files lacking an SPDX MIT line in the first 5 lines.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
bad=0
while IFS= read -r f; do
  [ -f "$f" ] || continue
  case "$f" in docs/*|.orchestrator/*|*.lock|*/package-lock.json|package-lock.json|go.sum|*/node_modules/*|*/dist/*|ui/pnpm-lock.yaml) continue ;; esac
  if ! head -n 5 "$f" | grep -q 'SPDX-License-Identifier: MIT'; then
    echo "missing SPDX header: $f"; bad=1
  fi
done < <(git ls-files --cached --others --exclude-standard -- \
  '*.go' '*.sh' '*.bzl' 'BUILD.bazel' '*/BUILD.bazel' 'MODULE.bazel' \
  '*.ts' '*.tsx' '*.js' '*.mjs' '*.css' '*.html' '*.sql' '*.yaml' '*.yml' | sort -u)
exit $bad

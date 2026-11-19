#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Dependency license check (SPEC "License"). Usage: tools/check_licenses.sh [--write]
#   (no flag)  classify every Go module in use, verify that each npm lockfile still matches the
#              hash recorded in tools/npm_licenses.json, evaluate the recorded npm licenses against
#              the policy, and fail if docs/third-party-licenses.md is out of date. Needs only
#              bash and go (Go modules must be in the module cache: go mod download); no
#              node_modules, pnpm or node.
#   --write    re-read the npm packages (needs ui/node_modules from "pnpm install --frozen-lockfile",
#              and e2e/browser/node_modules from "npm ci" only if its lockfile lacks a license),
#              then rewrite tools/npm_licenses.json and docs/third-party-licenses.md.
# The checker is tools/licensecheck.go (build tag "ignore", so Bazel and "go build ./..." skip it).
set -euo pipefail
cd "$(dirname "$0")/.."
command -v go >/dev/null || { echo "check_licenses: go is required" >&2; exit 2; }
exec go run tools/licensecheck.go "$@"

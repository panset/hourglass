#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
set -euo pipefail
cd "$(dirname "$0")"
bash tools/check_spdx.sh
bash tools/check_core_generic.sh
bash tools/check_licenses.sh
go build ./...
# UI-3: the no-tag path must build without Node, Bazel or built UI files.
CGO_ENABLED=0 go build -o /dev/null ./cmd/hourglassd
go vet ./...
bazel run //:gazelle -- update -mode=diff -build_file_name=BUILD.bazel
bazel build //...
bazel test //...

#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Loads the images the local stacks need into the local Docker daemon, for the
# host architecture, as <name>:dev. Needs Bazel and a running Docker daemon.
#
#   deploy/load-images.sh                 every image (stacks: bundled, split, end-to-end)
#   deploy/load-images.sh hourglass-ui    only the named images
#
# Images: hourglass hourglass-api hourglass-ui (product) and
# hourglass-e2e hourglass-api-e2e hourglass-vcsim (test stacks only).
set -euo pipefail
cd "$(dirname "$0")/.."

all=(hourglass hourglass-api hourglass-ui hourglass-e2e hourglass-api-e2e hourglass-vcsim)
if [ "$#" -gt 0 ]; then names=("$@"); else names=("${all[@]}"); fi

for n in "${names[@]}"; do
  ok=0
  for a in "${all[@]}"; do [ "$a" = "$n" ] && ok=1; done
  if [ "$ok" = 0 ]; then
    echo "load-images: unknown image '$n' (known: ${all[*]})" >&2
    exit 2
  fi
done

for n in "${names[@]}"; do
  echo "== loading $n:dev"
  bazel run "//deploy/images:load_$n"
done
docker image ls --format '{{.Repository}}:{{.Tag}}  {{.Size}}' | grep -E '^hourglass(-[a-z0-9-]+)?:dev ' || true

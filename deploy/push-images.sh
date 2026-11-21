#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
#
# Pushes the product images (hourglass, hourglass-api, hourglass-ui) to a
# registry, each as one multi-platform image (linux/amd64 and linux/arm64).
# The test images are never pushed.
#
#   docker login ghcr.io                    # once; a token with write:packages
#   deploy/push-images.sh v1.0.0            # ghcr.io/panset/NAME:v1.0.0
#   deploy/push-images.sh v1.0.0 latest     # several tags
#   HOURGLASS_REGISTRY=registry.example.com/team deploy/push-images.sh v1.0.0
#
# Needs Bazel and registry credentials in the Docker config; it does not need a
# running Docker daemon.
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ $# -eq 0 ]]; then
  echo "usage: deploy/push-images.sh TAG [TAG...]" >&2
  exit 2
fi
args=()
for tag in "$@"; do
  if [[ ! $tag =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]]; then
    echo "push-images: '$tag' is not a valid image tag" >&2
    exit 2
  fi
  args+=(--tag "$tag")
done

for name in hourglass hourglass-api hourglass-ui; do
  repo=()
  if [[ -n ${HOURGLASS_REGISTRY:-} ]]; then
    repo=(--repository "${HOURGLASS_REGISTRY%/}/$name")
  fi
  echo "==> pushing $name ($*)"
  bazel run "//deploy/images:push_$name" -- "${args[@]}" ${repo[@]+"${repo[@]}"}
done

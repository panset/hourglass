#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Smoke check of the product images (not part of `bazel test`; run by hand and
# by the end-to-end task). For each of hourglass:dev, hourglass-api:dev and
# hourglass-ui:dev it runs the container with a temporary volume and a
# throwaway admin password, waits for /healthz 200, checks what / serves,
# confirms the process is not root, and removes everything it created.
#
#   deploy/smoke-images.sh            load the images (deploy/load-images.sh), then check
#   deploy/smoke-images.sh --no-load  check the images already in Docker
set -euo pipefail
cd "$(dirname "$0")/.."

if [ "${1:-}" != "--no-load" ]; then
  deploy/load-images.sh hourglass hourglass-api hourglass-ui
fi

run_id="hgsmoke-$$"
pw="$(openssl rand -hex 16)"
cleanup() {
  docker rm -f "$run_id-bundled" "$run_id-api" "$run_id-ui" >/dev/null 2>&1 || true
  docker volume rm -f "$run_id-v1" "$run_id-v2" >/dev/null 2>&1 || true
  docker network rm "$run_id-net" >/dev/null 2>&1 || true
}
trap cleanup EXIT
fail() { echo "SMOKE FAIL: $*" >&2; docker logs --tail 20 "$run_id-bundled" 2>&1 | sed 's/^/  bundled| /' >&2 || true; docker logs --tail 20 "$run_id-api" 2>&1 | sed 's/^/  api| /' >&2 || true; docker logs --tail 20 "$run_id-ui" 2>&1 | sed 's/^/  ui| /' >&2 || true; exit 1; }

# host_port NAME -> the loopback port published for the container's 8080.
host_port() { docker port "$1" 8080/tcp | head -n1 | sed 's/.*://'; }

# wait_for URL: waits up to 60 seconds for HTTP 200.
wait_for() {
  local url="$1" i
  for i in $(seq 1 60); do
    [ "$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)" = 200 ] && return 0
    sleep 1
  done
  return 1
}

# code URL -> HTTP status.
code() { curl -s -o /dev/null -w '%{http_code}' "$1"; }

# nonroot NAME: the configured user is 65532 and the running process is not uid 0.
nonroot() {
  local c="$1" user uid
  user="$(docker inspect -f '{{.Config.User}}' "$c")"
  [ "$user" = "65532:65532" ] || fail "$c: image user is '$user', want 65532:65532"
  uid="$(docker top "$c" -eo uid,pid | awk 'NR==2 {print $1}')"
  case "$uid" in
    ''|0|root) fail "$c: the main process runs as '$uid'" ;;
  esac
  echo "   $c runs as uid $uid"
}

echo "== hourglass:dev (bundled)"
docker volume create "$run_id-v1" >/dev/null
docker run -d --name "$run_id-bundled" -v "$run_id-v1:/data" -p 127.0.0.1::8080 \
  -e HOURGLASS_ADMIN_PASSWORD="$pw" hourglass:dev >/dev/null
p="$(host_port "$run_id-bundled")"
wait_for "http://127.0.0.1:$p/healthz" || fail "bundled: /healthz never returned 200"
curl -s "http://127.0.0.1:$p/healthz" | grep -q '"database":"ok"' || fail "bundled: database not ok (is /data writable by 65532?)"
curl -s -H 'Accept: text/html' "http://127.0.0.1:$p/" | grep -qi '<html' || fail "bundled: / is not HTML"
[ "$(code "http://127.0.0.1:$p/setups")" = 200 ] || fail "bundled: SPA fallback /setups is not 200"
nonroot "$run_id-bundled"
docker rm -f "$run_id-bundled" >/dev/null

echo "== hourglass-api:dev (API only) and hourglass-ui:dev (split)"
docker network create "$run_id-net" >/dev/null
docker volume create "$run_id-v2" >/dev/null
docker run -d --name "$run_id-api" --network "$run_id-net" --network-alias api -v "$run_id-v2:/data" -p 127.0.0.1::8080 \
  -e HOURGLASS_ADMIN_PASSWORD="$pw" hourglass-api:dev >/dev/null
pa="$(host_port "$run_id-api")"
wait_for "http://127.0.0.1:$pa/healthz" || fail "api: /healthz never returned 200"
[ "$(code "http://127.0.0.1:$pa/")" = 404 ] || fail "api: / is not 404"
nonroot "$run_id-api"

docker run -d --name "$run_id-ui" --network "$run_id-net" -p 127.0.0.1::8080 \
  -e HOURGLASS_UI_API=http://api:8080 hourglass-ui:dev >/dev/null
pu="$(host_port "$run_id-ui")"
wait_for "http://127.0.0.1:$pu/ui-healthz" || fail "ui: /ui-healthz never returned 200"
wait_for "http://127.0.0.1:$pu/healthz" || fail "ui: /healthz through the proxy never returned 200"
curl -s -H 'Accept: text/html' "http://127.0.0.1:$pu/" | grep -qi '<html' || fail "ui: / is not HTML"
curl -s "http://127.0.0.1:$pu/api/v1/auth/methods" | grep -q '"local"' || fail "ui: /api/ is not proxied to the API"
[ "$(code "http://127.0.0.1:$pu/setups")" = 200 ] || fail "ui: SPA fallback /setups is not 200"
nonroot "$run_id-ui"

echo "SMOKE OK: hourglass, hourglass-api, hourglass-ui"

#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# The integration scenario's stack (reported provider and pressure source, webhook to host port 9100), brought up
# by hand and left running, for trying the curl examples of docs/integrating.md. Test builds only; fake credentials.
#
#   e2e/integration-stack.sh up      load the images if missing, start the stack, set up the admin and a service
#                                    account "docs-example" with the four scopes, print its token
#   e2e/integration-stack.sh token   issue another token for "docs-example"
#   e2e/integration-stack.sh down    stop the stack and delete its volume
#
# Admin: user "admin", password "dev-only-e2e-admin-password" (the initial password is changed on first use).
# App: http://localhost:8080. Clock control: http://localhost:8091.
set -euo pipefail
cd "$(dirname "$0")/.."
compose=(docker compose -f deploy/compose.bundled.yaml -f e2e/compose.bundled.yaml -f e2e/compose.integration.yaml)
base=http://localhost:8080
initial=dev-only-not-a-secret
admin_pw=dev-only-e2e-admin-password
jar="$(mktemp)"; trap 'rm -f "$jar"' EXIT

login() {
  curl -fsS -c "$jar" -b "$jar" -o /dev/null -H 'content-type: application/json' -d "{\"username\":\"admin\",\"password\":\"$1\"}" "$base/auth/local"
}
csrf() { curl -fsS -b "$jar" "$base/api/v1/session" | sed -n 's/.*"csrf_token":"\([^"]*\)".*/\1/p'; }
post() { curl -fsS -b "$jar" -H "X-CSRF-Token: $(csrf)" -H 'content-type: application/json' -d "$2" "$base$1"; }

signin() {
  if login "$admin_pw" 2>/dev/null; then return; fi
  login "$initial"
  post /auth/local/password "{\"current_password\":\"$initial\",\"new_password\":\"$admin_pw\"}" >/dev/null
  login "$admin_pw"
}

account_id() {
  curl -fsS -b "$jar" "$base/api/v1/admin/service-accounts" | python3 -c '
import json, sys
for a in json.load(sys.stdin)["service_accounts"]:
    if a["name"] == "docs-example":
        print(a["id"]); break'
}

issue() {
  signin
  id="$(account_id)"
  if [ -z "$id" ]; then
    id="$(post /api/v1/admin/service-accounts '{"name":"docs-example","description":"integration stack","scopes":["state:read","pressure:write","deployments:write","holds:release"]}' | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
  fi
  post "/api/v1/admin/service-accounts/$id/tokens" '{}' | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])'
}

case "${1:-}" in
  up)
    for i in hourglass-e2e hourglass-vcsim; do docker image inspect "$i:dev" >/dev/null 2>&1 || { deploy/load-images.sh; break; }; done
    "${compose[@]}" up -d --wait
    echo "token: $(issue)"
    ;;
  token) echo "token: $(issue)" ;;
  down) "${compose[@]}" down -v --remove-orphans ;;
  *) echo "usage: $0 <up|token|down>" >&2; exit 2 ;;
esac

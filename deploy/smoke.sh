#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Smoke test for the local stacks: deploy/smoke.sh <bundled|split>
#
# Loads the images, brings the stack up, checks it with curl, and always
# tears it down (docker compose down -v). Exits non-zero on any failure.
# Needs Bazel (to load images), Docker with compose v2, curl and python3.
# Set HOURGLASS_SMOKE_NO_LOAD=1 to skip loading the images.
set -uo pipefail
cd "$(dirname "$0")/.."

mode="${1:-}"
case "$mode" in
  bundled) app=hourglass ;;
  split) app=hourglass-api ;;
  *) echo "usage: deploy/smoke.sh <bundled|split>" >&2; exit 2 ;;
esac
compose=(docker compose -f "deploy/compose.$mode.yaml")
base=http://localhost:8080
issuer=http://keycloak.localhost:8081/realms/hourglass
admin_password=dev-only-not-a-secret
new_password=dev-only-changed-password
tmp="$(mktemp -d)"
jar="$tmp/cookies"
fail=0

teardown() {
  echo "== teardown"
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1
  rm -rf "$tmp"
}
trap teardown EXIT

check() { # check <description> <command...>
  local d="$1"; shift
  if "$@" >"$tmp/out" 2>&1; then echo "ok   $d"; else echo "FAIL $d"; sed 's/^/     /' "$tmp/out" | tail -5; fail=1; fi
}
status_is() { [ "$(curl -s -o /dev/null -w '%{http_code}' "${@:2}")" = "$1" ]; }
json() { python3 -I -c "import sys,json; d=json.load(sys.stdin); $1"; }

if [ "${HOURGLASS_SMOKE_NO_LOAD:-}" != 1 ]; then
  echo "== loading images"
  deploy/load-images.sh >"$tmp/load.log" 2>&1 || { tail -20 "$tmp/load.log"; echo "FAIL load images"; exit 1; }
fi

echo "== up ($mode)"
"${compose[@]}" down -v --remove-orphans >/dev/null 2>&1
if ! "${compose[@]}" up -d --wait --wait-timeout 300 >"$tmp/up.log" 2>&1; then
  tail -20 "$tmp/up.log"; "${compose[@]}" ps -a; echo "FAIL compose up"; exit 1
fi

echo "== checks"
check "/healthz is 200" status_is 200 "$base/healthz"
check "/ returns the UI's HTML" bash -c "curl -fsS $base/ | grep -qi '<!doctype html'"
check "/api/v1/auth/methods reports OIDC available" bash -c \
  "curl -fsS $base/api/v1/auth/methods | python3 -I -c 'import sys,json; d=json.load(sys.stdin); sys.exit(0 if d[\"oidc\"] is True and d[\"local\"] is True else 1)'"
check "/auth/oidc/start redirects to the issuer" bash -c \
  "loc=\$(curl -s -o /dev/null -w '%{redirect_url}' $base/auth/oidc/start); case \"\$loc\" in $issuer/*) ;; *) echo \"Location: \$loc\"; exit 1;; esac"
check "Keycloak discovery reachable from the host at the issuer URL" bash -c \
  "curl -fsS $issuer/.well-known/openid-configuration | python3 -I -c 'import sys,json; sys.exit(0 if json.load(sys.stdin)[\"issuer\"] == \"$issuer\" else 1)'"
# The product images have no shell or curl, so "from inside" means: the same
# network, the same alias (curl treats *.localhost as loopback itself, so the
# probe resolves the alias explicitly), and Hourglass's own discovery
# (/auth/oidc/start above only redirects once Hourglass fetched it itself).
inside() { "${compose[@]}" exec -T deps-ready sh -c \
  "ip=\$(getent hosts keycloak.localhost | cut -d' ' -f1); curl -fsS --resolve keycloak.localhost:8081:\$ip $issuer/.well-known/openid-configuration >/dev/null"; }
check "Keycloak discovery reachable inside the compose network under the issuer's host name" inside
check "simulator control endpoint answers" bash -c "curl -fsS http://localhost:8990/healthz | grep -q ok"
if [ "$mode" = split ]; then
  check "hourglass-ui /ui-healthz answers" bash -c "curl -fsS $base/ui-healthz | grep -q ok"
fi

echo "== local admin"
signin() { curl -s -o "$tmp/session.json" -w '%{http_code}' -c "$jar" -b "$jar" -H 'Content-Type: application/json' \
  -d "{\"username\":\"admin\",\"password\":\"$1\"}" "$base/auth/local"; }
code="$(signin "$admin_password")"
check "POST /auth/local with the compose password gives 200 and a session cookie" \
  bash -c "[ '$code' = 200 ] && grep -q hourglass_session '$jar'"
csrf="$(json 'print(d["csrf_token"])' <"$tmp/session.json" 2>/dev/null)"
must="$(json 'print(d["must_change_password"])' <"$tmp/session.json" 2>/dev/null)"
check "first sign-in requires a password change" test "$must" = True
check "password change succeeds" bash -c "curl -fsS -c '$jar' -b '$jar' -H 'Content-Type: application/json' -H 'X-CSRF-Token: $csrf' \
  -d '{\"current_password\":\"$admin_password\",\"new_password\":\"$new_password\"}' $base/auth/local/password -o '$tmp/session.json'"
csrf="$(json 'print(d["csrf_token"])' <"$tmp/session.json" 2>/dev/null)"

echo "== first poll (up to 5 minutes)"
deadline=$((SECONDS + 300))
ok=0
while [ "$SECONDS" -lt "$deadline" ]; do
  if curl -fsS -b "$jar" "$base/api/v1/admin/collector" >"$tmp/collector.json" 2>/dev/null &&
     json 'e=d["extensions"]; r=d["recent_runs"]; sys.exit(0 if e["provider"]=="vsphere" and e["pressure_source"]=="vsphere" and d["last_successful_poll_at"] and r and r[0]["ok"] else 1)' <"$tmp/collector.json"; then
    ok=1; break
  fi
  sleep 3
done
if [ "$ok" = 1 ]; then echo "ok   collector: provider vsphere, pressure source vsphere, last poll OK"
else echo "FAIL collector did not report a successful vsphere poll"; tail -c 600 "$tmp/collector.json" 2>/dev/null; fail=1; fi

check "the six seeded setups exist (lab-01 .. lab-06)" bash -c \
  "curl -fsS -b '$jar' $base/api/v1/setups | python3 -I -c 'import sys,json; d=json.load(sys.stdin); n=sorted(s[\"name\"] for s in d[\"setups\"]); sys.exit(0 if n==[\"lab-0%d\" % i for i in range(1,7)] else (print(n) or 1))'"

if [ "$fail" = 0 ]; then echo "SMOKE OK ($mode)"; else echo "SMOKE FAILED ($mode)"; "${compose[@]}" logs --tail 30 "$app" 2>&1 | cut -c1-300; fi
exit "$fail"

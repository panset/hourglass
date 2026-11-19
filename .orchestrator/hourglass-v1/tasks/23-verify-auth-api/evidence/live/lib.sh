# SPDX-License-Identifier: MIT
# helpers for the live attacks (verification task 23)
B=http://127.0.0.1:18923
IDP=http://127.0.0.1:18925
# oidc_signin sub email name -> prints "cookie csrf location"
oidc_signin() {
  curl -s "$IDP/control?sub=$1&email=$2&name=$3&mode=${4:-ok}" >/dev/null
  local h; h=$(curl -s -D - -o /dev/null $B/auth/oidc/start)
  local loc fc; loc=$(echo "$h" | grep -i '^location:' | tr -d '\r' | cut -d' ' -f2)
  fc=$(echo "$h" | grep -i 'set-cookie: hourglass_oidc=' | sed 's/.*hourglass_oidc=\([^;]*\).*/\1/')
  local cb; cb=$(curl -s -D - -o /dev/null "$loc" | grep -i '^location:' | tr -d '\r' | cut -d' ' -f2)
  local h2; h2=$(curl -s -D - -o /dev/null -H "Cookie: hourglass_oidc=$fc" "$cb")
  local sc; sc=$(echo "$h2" | grep -i 'set-cookie: hourglass_session=' | sed 's/.*hourglass_session=\([^;]*\).*/\1/')
  local l2; l2=$(echo "$h2" | grep -i '^location:' | tr -d '\r' | cut -d' ' -f2)
  local tok=""; [ -n "$sc" ] && tok=$(curl -s -H "Cookie: hourglass_session=$sc" $B/api/v1/session | python3 -c 'import sys,json;print(json.load(sys.stdin)["csrf_token"])')
  echo "$sc $tok $l2"
}
get() { curl -s -H "Cookie: hourglass_session=$1" "$B$2"; }
send() { curl -s -X "$2" -H "Cookie: hourglass_session=$1" -H "X-CSRF-Token: $T2" -H 'Content-Type: application/json' -d "$4" "$B$3"; }

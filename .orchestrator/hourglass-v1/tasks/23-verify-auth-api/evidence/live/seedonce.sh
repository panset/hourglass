#!/bin/bash
# SPDX-License-Identifier: MIT
# SET-7 seed-once and UI-3 fallback checks on a throwaway server (task 23, check 8)
cd "$(dirname "$0")"; rm -f s7.db*; P=127.0.0.1:18930; B=http://$P
start(){ env HOURGLASS_LISTEN=$P HOURGLASS_DATABASE=$PWD/s7.db HOURGLASS_ADMIN_PASSWORD=seed-once-password-1 HOURGLASS_SESSION_KEY=seed-once-session-key-0123456789abcdef "$@" > s7.log 2>&1 & echo $! > s7.pid; for i in $(seq 1 50); do curl -s -o /dev/null $B/healthz && break; sleep 0.2; done; }
stop(){ kill $(cat s7.pid); while kill -0 $(cat s7.pid) 2>/dev/null; do sleep 0.2; done; }
login(){ local r; r=$(curl -s -D s7h.txt -H 'Content-Type: application/json' -d "{\"username\":\"admin\",\"password\":\"$1\"}" $B/auth/local); C=$(grep -i 'set-cookie: hourglass_session=' s7h.txt | sed 's/.*hourglass_session=\([^;]*\).*/\1/'); T=$(echo "$r" | python3 -c 'import sys,json;print(json.load(sys.stdin)["csrf_token"])'); }
HOURGLASS_SEED=x start ../bin/hourglassd-ui -config /dev/null 2>/dev/null; stop
printf 'listen: "%s"\nseed: %s/seed.yaml\n' $P $PWD > s7.yaml
rm -f s7.db*; start ../bin/hourglassd-ui -config s7.yaml
login seed-once-password-1
curl -s -H "Cookie: hourglass_session=$C" -H "X-CSRF-Token: $T" -H 'Content-Type: application/json' -d '{"current_password":"seed-once-password-1","new_password":"seed-once-password-2"}' -D s7h.txt $B/auth/local/password > s7r.json; login seed-once-password-2
echo "first start setups: $(curl -s -H "Cookie: hourglass_session=$C" $B/api/v1/setups | python3 -c 'import sys,json;print([s["name"] for s in json.load(sys.stdin)["setups"]])')"
echo "rules: $(curl -s -H "Cookie: hourglass_session=$C" $B/api/v1/admin/rules | python3 -c 'import sys,json;print([r["selector"] for r in json.load(sys.stdin)["rules"]])')"
curl -s -X DELETE -H "Cookie: hourglass_session=$C" -H "X-CSRF-Token: $T" $B/api/v1/admin/setups/1 -o /dev/null -w "delete: %{http_code}\n"
echo "== UI-3 fallback (ui-tagged binary, ui.serve default true)"
for p in / /setups /admin/users /assets/app.js /assets/missing.js /api/nope /api /auth/nope /healthz /healthzx; do printf "%-20s %s\n" $p "$(curl -s -o /dev/null -w '%{http_code} %{content_type}' $B$p) csp=$(curl -s -D - -o /dev/null $B$p | grep -ci content-security-policy)"; done
stop; start ../bin/hourglassd-ui -config s7.yaml; login seed-once-password-2
echo "after restart setups: $(curl -s -H "Cookie: hourglass_session=$C" $B/api/v1/setups | python3 -c 'import sys,json;print([s["name"] for s in json.load(sys.stdin)["setups"]])')"
grep -c "seed file imported" s7.log
stop; HOURGLASS_UI_SERVE=false start ../bin/hourglassd-ui -config s7.yaml
echo "ui.serve=false: / -> $(curl -s -o /dev/null -w '%{http_code}' $B/)  /setups -> $(curl -s -o /dev/null -w '%{http_code}' $B/setups)"
grep -i "ui" s7.log | head -2 | cut -c1-200
stop

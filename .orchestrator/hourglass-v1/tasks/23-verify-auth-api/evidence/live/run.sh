#!/bin/bash
# SPDX-License-Identifier: MIT
# starts fakeidp and hourglassd with sentinel secrets
D=$(cd "$(dirname "$0")" && pwd)
pkill -f "bin/fakeidp 127"; pkill -f "bin/hourglassd -config"; while pgrep -f "bin/hourglassd -config" >/dev/null; do sleep 0.3; done; sleep 0.3
nohup "$D/../bin/fakeidp" 127.0.0.1:18925 > "$D/idp.log" 2>&1 &
HOURGLASS_SESSION_KEY=SENTINEL-SESSKEY-0123456789abcdef0123456789 \
HOURGLASS_CORS_ALLOWED_ORIGINS="http://dev.example:5173, null" \
HOURGLASS_ADMIN_PASSWORD=SENTINEL-ADMINPW-7f3a \
HOURGLASS_OIDC_ISSUER=http://127.0.0.1:18925 \
HOURGLASS_OIDC_CLIENT_ID=hourglass \
HOURGLASS_OIDC_CLIENT_SECRET=SENTINEL-OIDCSECRET-91c2 \
HOURGLASS_OIDC_REDIRECT_URL=http://127.0.0.1:18923/auth/callback \
nohup "$D/../bin/hourglassd" -config "$D/config.yaml" >> "$D/server.log" 2>&1 &
for i in $(seq 1 50); do curl -s -o /dev/null http://127.0.0.1:18923/healthz && break; sleep 0.2; done

#!/bin/bash
# SPDX-License-Identifier: MIT
# runs hourglassd with bad configs carrying sentinel secrets (task 23, check 6)
cd "$(dirname "$0")"; H=../bin/hourglassd; TO=../bin/to; : > configerr.log
run(){ echo "### $1" >> configerr.log; shift; env "$@" >> configerr.log 2>&1; echo "exit $?" >> configerr.log; }
run "short session key" HOURGLASS_SESSION_KEY=SENTINEL-SHORTKEY HOURGLASS_DATABASE=$PWD/tmp1.db HOURGLASS_LISTEN=127.0.0.1:18930 $TO 5 $H
run "short admin password" HOURGLASS_ADMIN_PASSWORD=SENTINEL-pw HOURGLASS_DATABASE=$PWD/tmp4.db HOURGLASS_LISTEN=127.0.0.1:18930 $TO 5 $H
run "unknown provider" $TO 5 $H -config cfg_unknown.yaml
run "bad hook config" $TO 5 $H -config cfg_badhook.yaml
run "yaml syntax error" $TO 5 $H -config cfg_yamlerr.yaml
run "oidc issuer unreachable + secret" HOURGLASS_OIDC_ISSUER=http://127.0.0.1:1 HOURGLASS_OIDC_CLIENT_ID=c HOURGLASS_OIDC_REDIRECT_URL=http://x/auth/callback HOURGLASS_OIDC_CLIENT_SECRET=SENTINEL-OIDCSECRET-91c2 HOURGLASS_DATABASE=$PWD/tmp5.db HOURGLASS_LISTEN=127.0.0.1:18930 $TO 4 $H
run "check-provider" HOURGLASS_SESSION_KEY=SENTINEL-SESSKEY-0123456789abcdef0123456789 HOURGLASS_OIDC_CLIENT_SECRET=SENTINEL-OIDCSECRET-91c2 HOURGLASS_ADMIN_PASSWORD=SENTINEL-ADMINPW-7f3a $TO 20 $H check-provider -config config.yaml
run "check-provider unknown" $TO 20 $H check-provider -config cfg_unknown.yaml

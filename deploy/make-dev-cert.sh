#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
#
# Writes a self-signed certificate and key for the local HTTPS stack
# (compose.tls.yaml) to deploy/certs/, which git ignores. Development only.
#
#   deploy/make-dev-cert.sh                  # localhost, hourglass, 127.0.0.1
#   deploy/make-dev-cert.sh lab.example.com  # extra host names
#
# An existing pair is kept; delete deploy/certs/ to make a new one.
set -euo pipefail
cd "$(dirname "$0")"

if [[ -s certs/tls.crt && -s certs/tls.key ]]; then
  echo "make-dev-cert: deploy/certs/tls.crt exists, keeping it"
  exit 0
fi
san="DNS:localhost,DNS:hourglass,IP:127.0.0.1"
for host in "$@"; do
  san+=",DNS:$host"
done
mkdir -p certs
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout certs/tls.key -out certs/tls.crt \
  -subj "/CN=localhost" -addext "subjectAltName=$san" 2>/dev/null
# The container runs as uid 65532 and must read the key through a bind mount.
# World-readable is acceptable only because this key protects nothing real.
chmod 644 certs/tls.crt certs/tls.key
echo "make-dev-cert: wrote deploy/certs/tls.crt and deploy/certs/tls.key ($san)"

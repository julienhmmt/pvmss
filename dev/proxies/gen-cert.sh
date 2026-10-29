#!/bin/sh
# Self-signed certificate shared by the dev reverse proxies (localhost only).
# Idempotent: keeps an existing certificate so the browser exception sticks.
set -eu
dir=/certs
[ -f "$dir/tls.crt" ] && exit 0
apk add --no-cache openssl >/dev/null
openssl req -x509 -newkey rsa:2048 -nodes -days 825 -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" \
  -keyout "$dir/tls.key" -out "$dir/tls.crt" 2>/dev/null
cat "$dir/tls.crt" "$dir/tls.key" > "$dir/haproxy.pem"
chmod 644 "$dir"/*

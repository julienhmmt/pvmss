#!/bin/sh
# Dev PKI for the SIEM receiver (localhost only), written to the bind-mounted
# dev/rsyslog/tls/ (gitignored):
#   ca.crt                 CA that pvmss-dev trusts via PVMSS_SYSLOG_TLS_CA_FILE
#   server.crt/key         rsyslog listener cert (syslog, localhost)
# Idempotent: keeps existing files.
set -eu
dir=/certs
[ -f "$dir/server.crt" ] && exit 0
apk add --no-cache openssl >/dev/null
cd "$dir"

openssl req -x509 -newkey rsa:2048 -nodes -days 825 -subj "/CN=PVMSS dev SIEM CA" \
  -addext "basicConstraints=critical,CA:true" -addext "keyUsage=critical,keyCertSign,cRLSign" \
  -keyout ca.key -out ca.crt 2>/dev/null

openssl req -newkey rsa:2048 -nodes -subj "/CN=syslog" -keyout server.key -out server.csr 2>/dev/null
printf 'extendedKeyUsage=serverAuth\nsubjectAltName=DNS:syslog,DNS:localhost,IP:127.0.0.1\n' > server.ext
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 825 \
  -extfile server.ext -out server.crt 2>/dev/null
rm -f server.csr server.ext ca.srl ca.key
chmod 644 ./*

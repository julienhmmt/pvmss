#!/bin/sh
# Dev PKI for the SIEM receiver (localhost only), written to the bind-mounted
# dev/victorialogs/tls/ (gitignored):
#   ca.crt                 CA that pvmss-dev trusts via PVMSS_SYSLOG_TLS_CA_FILE
#   server.crt/key         VictoriaLogs syslog listener cert (victorialogs, localhost)
# Idempotent: keeps existing files, unless they predate the victorialogs SAN.
set -eu
dir=/certs
apk add --no-cache openssl >/dev/null
if [ -f "$dir/server.crt" ] && openssl x509 -in "$dir/server.crt" -noout -ext subjectAltName | grep -q 'DNS:victorialogs'; then
  exit 0
fi
cd "$dir"

openssl req -x509 -newkey rsa:2048 -nodes -days 825 -subj "/CN=PVMSS dev SIEM CA" \
  -addext "basicConstraints=critical,CA:true" -addext "keyUsage=critical,keyCertSign,cRLSign" \
  -keyout ca.key -out ca.crt 2>/dev/null

openssl req -newkey rsa:2048 -nodes -subj "/CN=victorialogs" -keyout server.key -out server.csr 2>/dev/null
printf 'extendedKeyUsage=serverAuth\nsubjectAltName=DNS:victorialogs,DNS:localhost,IP:127.0.0.1\n' > server.ext
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 825 \
  -extfile server.ext -out server.crt 2>/dev/null
rm -f server.csr server.ext ca.srl ca.key
chmod 644 ./*

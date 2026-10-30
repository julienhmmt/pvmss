#!/bin/sh
# Dev PKI for the reverse-proxy stack (localhost only), written to the
# bind-mounted dev/proxies/certs/ (gitignored):
#   ca.crt                         CA trusted by every hop
#   tls.crt/key, haproxy.pem       proxies' server cert (localhost)
#   pvmss.crt/key                  pvmss-dev server cert (pvmss-dev, pvmss.internal)
#   proxy-client.crt/key/.pem      proxies -> pvmss-dev client cert
#   client.crt/key, client.p12     browser / Playwright client cert (p12 password: pvmss)
# Idempotent: keeps existing files so imported certificates keep working.
set -eu
dir=/certs
[ -f "$dir/client.p12" ] && exit 0
apk add --no-cache openssl >/dev/null
cd "$dir"

openssl req -x509 -newkey rsa:2048 -nodes -days 825 -subj "/CN=PVMSS dev CA" \
  -addext "basicConstraints=critical,CA:true" -addext "keyUsage=critical,keyCertSign,cRLSign" \
  -keyout ca.key -out ca.crt 2>/dev/null

# issue NAME CN EKU SAN
issue() {
  openssl req -newkey rsa:2048 -nodes -subj "/CN=$2" -keyout "$1.key" -out "$1.csr" 2>/dev/null
  printf 'extendedKeyUsage=%s\n%s\n' "$3" "${4:+subjectAltName=$4}" > "$1.ext"
  openssl x509 -req -in "$1.csr" -CA ca.crt -CAkey ca.key -CAcreateserial -days 825 \
    -extfile "$1.ext" -out "$1.crt" 2>/dev/null
  rm -f "$1.csr" "$1.ext"
}

issue tls localhost serverAuth "DNS:localhost,DNS:pvmss.localhost,IP:127.0.0.1"
issue pvmss pvmss-dev serverAuth "DNS:pvmss-dev,DNS:pvmss.internal,DNS:localhost"
issue proxy-client pvmss-proxy clientAuth ""
issue client alice clientAuth ""

cat tls.crt tls.key > haproxy.pem
cat proxy-client.crt proxy-client.key > proxy-client.pem
# -legacy: macOS Keychain cannot import OpenSSL 3's default p12 encryption.
openssl pkcs12 -export -legacy -passout pass:pvmss -name "PVMSS dev client" \
  -inkey client.key -in client.crt -certfile ca.crt -out client.p12
rm -f ca.srl
chmod 644 ./*

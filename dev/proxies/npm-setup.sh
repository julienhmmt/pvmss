#!/bin/sh
# Creates the Nginx Proxy Manager proxy host pvmss.localhost -> https pvmss-dev:50000
# (WebSockets on, mTLS on both hops) through its API. Idempotent: an existing
# host is updated. Uses NPM's first-boot admin
# unless NPM_EMAIL / NPM_PASSWORD point at the account you created.
set -eu
api=${NPM_API:-http://localhost:50081/api}
email=${NPM_EMAIL:-admin@example.com}
password=${NPM_PASSWORD:-changeme}
certs=$(dirname "$0")/certs

i=0
until curl -fs "$api/" >/dev/null; do
  i=$((i + 1)); [ "$i" -ge 60 ] && { echo "npm-setup: API not reachable at $api" >&2; exit 1; }
  sleep 1
done

# NPM >= 2.16 seeds no admin: create it on first boot (setup:false).
if curl -fs "$api/" | grep -q '"setup":false'; then
  curl -fs -X POST "$api/users" -H 'Content-Type: application/json' -d "{\"name\":\"Admin\",\"nickname\":\"admin\",\"email\":\"$email\",\"roles\":[\"admin\"],\"is_disabled\":false,\"auth\":{\"type\":\"password\",\"secret\":\"$password\"}}" >/dev/null
fi

token=$(curl -fs -X POST "$api/tokens" -H 'Content-Type: application/json' \
  -d "{\"identity\":\"$email\",\"secret\":\"$password\"}" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
[ -n "$token" ] || { echo "npm-setup: login failed for $email (set NPM_EMAIL/NPM_PASSWORD)" >&2; exit 1; }

auth="Authorization: Bearer $token"

# Custom certificate: the dev PKI's localhost server cert.
cert_id=$(curl -fs "$api/nginx/certificates" -H "$auth" | sed 's/},{"id"/}\n{"id"/g' | grep '"nice_name":"pvmss-dev"' | grep -o '^[[{]*"id":[0-9]*' | head -n1 | grep -o '[0-9]*$' || true)
if [ -z "$cert_id" ]; then
  cert_id=$(curl -fs -X POST "$api/nginx/certificates" -H "$auth" -H 'Content-Type: application/json' \
    -d '{"provider":"other","nice_name":"pvmss-dev"}' | grep -o '"id":[0-9]*' | head -n1 | cut -d: -f2)
fi
[ -n "$cert_id" ] || { echo "npm-setup: cannot create the pvmss-dev certificate" >&2; exit 1; }
# Always (re)upload: also repairs a certificate record left without files.
curl -fs -X POST "$api/nginx/certificates/$cert_id/upload" -H "$auth" \
  -F certificate=@"$certs/tls.crt" -F certificate_key=@"$certs/tls.key" >/dev/null

# Inside the NPM container the PKI is mounted at /certs.
advanced='ssl_client_certificate /certs/ca.crt;\nssl_verify_client on;\nproxy_ssl_certificate /certs/proxy-client.crt;\nproxy_ssl_certificate_key /certs/proxy-client.key;\nproxy_ssl_trusted_certificate /certs/ca.crt;\nproxy_ssl_verify on;\nproxy_ssl_server_name on;\nproxy_ssl_name pvmss-dev;'

body=$(cat <<JSON
{
  "domain_names": ["pvmss.localhost"],
  "forward_scheme": "https",
  "forward_host": "pvmss-dev",
  "forward_port": 50000,
  "allow_websocket_upgrade": true,
  "block_exploits": false,
  "caching_enabled": false,
  "access_list_id": 0,
  "certificate_id": $cert_id,
  "ssl_forced": false,
  "http2_support": false,
  "hsts_enabled": false,
  "hsts_subdomains": false,
  "advanced_config": "$advanced",
  "locations": [],
  "meta": {}
}
JSON
)

# One object per line (its nested objects split off), keep the host's own id.
host_id=$(curl -fs "$api/nginx/proxy-hosts" -H "$auth" | sed 's/},{"id"/}\n{"id"/g' | grep '"pvmss.localhost"' | grep -o '^[[{]*"id":[0-9]*' | head -n1 | grep -o '[0-9]*$' || true)
if [ -n "$host_id" ]; then
  curl -fs -X PUT "$api/nginx/proxy-hosts/$host_id" -H "$auth" -H 'Content-Type: application/json' -d "$body" >/dev/null
  echo "npm-setup: proxy host pvmss.localhost updated"
else
  curl -fs -X POST "$api/nginx/proxy-hosts" -H "$auth" -H 'Content-Type: application/json' -d "$body" >/dev/null
  echo "npm-setup: proxy host pvmss.localhost created"
fi

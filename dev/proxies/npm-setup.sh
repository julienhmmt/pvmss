#!/bin/sh
# Creates the Nginx Proxy Manager proxy host localhost -> pvmss-dev:50000
# (WebSockets on) through its API. Idempotent. Uses NPM's first-boot admin
# unless NPM_EMAIL / NPM_PASSWORD point at the account you created.
set -eu
api=${NPM_API:-http://localhost:50081/api}
email=${NPM_EMAIL:-admin@example.com}
password=${NPM_PASSWORD:-changeme}

i=0
until curl -fs "$api/" >/dev/null; do
  i=$((i + 1)); [ "$i" -ge 60 ] && { echo "npm-setup: API not reachable at $api" >&2; exit 1; }
  sleep 1
done

token=$(curl -fs -X POST "$api/tokens" -H 'Content-Type: application/json' \
  -d "{\"identity\":\"$email\",\"secret\":\"$password\"}" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
[ -n "$token" ] || { echo "npm-setup: login failed for $email (set NPM_EMAIL/NPM_PASSWORD)" >&2; exit 1; }

if curl -fs "$api/nginx/proxy-hosts" -H "Authorization: Bearer $token" | grep -q '"localhost"'; then
  echo "npm-setup: proxy host localhost already present"
  exit 0
fi

curl -fs -X POST "$api/nginx/proxy-hosts" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{
  "domain_names": ["localhost"],
  "forward_scheme": "http",
  "forward_host": "pvmss-dev",
  "forward_port": 50000,
  "allow_websocket_upgrade": true,
  "block_exploits": false,
  "caching_enabled": false,
  "access_list_id": 0,
  "certificate_id": 0,
  "ssl_forced": false,
  "http2_support": false,
  "hsts_enabled": false,
  "hsts_subdomains": false,
  "advanced_config": "",
  "locations": [],
  "meta": {}
}' >/dev/null
echo "npm-setup: proxy host localhost created"

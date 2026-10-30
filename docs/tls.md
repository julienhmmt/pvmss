# TLS and mutual TLS

PVMSS can serve plain HTTP, HTTPS, or HTTPS with mutual TLS (mTLS). Nothing is
forced, so it fits behind any reverse proxy, ingress or Gateway. **Mutual TLS
between your proxy and PVMSS is strongly recommended.**

This guide is for the people who run PVMSS. It covers:

1. [Which setup to pick](#which-setup-to-pick)
2. [How it works](#how-it-works)
3. [Reference: variables and behaviour](#reference)
4. [Quick start with Docker Compose](#quick-start-docker-compose)
5. [Building the certificates](#building-the-certificates)
6. [Kubernetes with the Helm chart](#kubernetes-with-the-helm-chart)
7. [Proxy recipes, proxy to PVMSS](#proxy-recipes-proxy-to-pvmss)
8. [Client certificates for users (optional)](#client-certificates-for-users-optional)
9. [Renewal, rotation and monitoring](#renewal-rotation-and-monitoring)
10. [Checking a deployment](#checking-a-deployment)
11. [Troubleshooting](#troubleshooting)
12. [FAQ](#faq)

## Which setup to pick

```text
browser --(1)--> reverse proxy --(2)--> PVMSS --> Proxmox API
```

- **Hop 2, proxy to PVMSS.** This is what `PVMSS_TLS_*` protects. The "client"
  here is your reverse proxy (or Gateway), so you need **one client
  certificate per proxy**, not one per user. This is the setting that matters:
  without it, anything that can reach the PVMSS port (another pod, a
  compromised host on the VLAN) talks to the API directly and skips your
  proxy, its access rules and its logs.
- **Hop 1, browser to proxy.** Users already sign in to PVMSS with their
  Proxmox or local account. Requiring a client certificate in the browser as
  well limits access to managed devices. It is handled by the proxy, not by
  PVMSS, and it needs a PKI that can issue and revoke one certificate per user
  or device. See [Client certificates for users](#client-certificates-for-users-optional).

| Your situation                                                         | Recommended                                                              |
| ---------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| Production, PVMSS and proxy on different hosts or pods                 | **`mtls`** on hop 2                                                      |
| Kubernetes behind a Gateway / ingress controller                       | **`mtls`** on hop 2 (Helm `tls.*` + `tls.clientCA.*`)                    |
| Proxy runs as a sidecar in the same pod, PVMSS bound to `127.0.0.1`    | `http` is acceptable: the hop never leaves the pod                       |
| Migrating an existing deployment                                       | `tls` first, then `mtls` with `PVMSS_TLS_CLIENT_AUTH=optional`, then `require` |
| Access must be limited to managed laptops                              | `mtls` on hop 2 **and** client certificates on hop 1                     |
| Demo or local test                                                     | `http`                                                                   |

## How it works

- PVMSS checks the client certificate against a **CA bundle**, not against a
  list of certificates. The number of clients has no effect on PVMSS: 1, 342 or
  10 000 certificates signed by the same CA cost the same.
- A client certificate must carry `extendedKeyUsage=clientAuth`. A certificate
  with only `serverAuth` is refused.
- PVMSS does **not** check revocation (no CRL, no OCSP) and does **not** map a
  certificate to a PVMSS user. A certificate decides who may open a
  connection; the PVMSS sign-in still decides who the user is.
- Every client that the client CA signed is let in. **Use a CA dedicated to
  these client certificates**, never a broad internal CA or a cluster-wide
  cert-manager `ClusterIssuer` that also signs unrelated services.

## Reference

| Mode   | Variables                                                                  | Banner `tls` field |
| ------ | -------------------------------------------------------------------------- | ------------------ |
| `http` | none (default)                                                             | `http`             |
| `tls`  | `PVMSS_TLS_CERT_FILE`, `PVMSS_TLS_KEY_FILE`                                | `tls`              |
| `mtls` | the two above + `PVMSS_TLS_CLIENT_CA_FILE` (+ `PVMSS_TLS_CLIENT_AUTH`)     | `mtls`             |

| Variable                   | Meaning                                                                                                     |
| -------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `PVMSS_TLS_CERT_FILE`      | PEM server certificate, full chain (leaf first). Set together with the key.                                  |
| `PVMSS_TLS_KEY_FILE`       | PEM private key of that certificate (RSA or ECDSA, unencrypted).                                            |
| `PVMSS_TLS_CLIENT_CA_FILE` | PEM bundle of the CAs allowed to sign client certificates. Turns on mTLS. Several CAs may be concatenated (useful while rotating the CA). |
| `PVMSS_TLS_CLIENT_AUTH`    | `require` (default): no valid client certificate, no connection. `optional`: a certificate is verified when sent, clients without one still get in. Needs `PVMSS_TLS_CLIENT_CA_FILE`. `optional` is reported as `tls`. |

Behaviour:

- TLS 1.2 is the minimum; TLS 1.3 is used when the client offers it.
- The files are loaded at startup. A missing, unreadable or invalid file stops
  PVMSS with an explicit error, for example
  `PVMSS_TLS_CLIENT_CA_FILE holds no PEM certificate`. Invalid combinations
  are refused too (a key without a certificate, a client CA without a server
  certificate).
- The files are **read once**. Restart PVMSS after renewing a certificate (see
  [Renewal](#renewal-rotation-and-monitoring)).
- The `pvmss starting` log line carries `tls=http|tls|mtls`. In `http` and
  `tls` modes PVMSS also logs one `Warn` line with `event=tls_advice`, so a
  log alert can list the instances that run without mTLS.
- A refused handshake is logged at `Warn` with `component=http`, for example
  `http: TLS handshake error from 10.0.3.7:51234: tls: client didn't provide a certificate`.
- The listener speaks HTTP/1.1 and HTTP/2. Keep the proxy-to-PVMSS hop on
  HTTP/1.1 (the recipes below do): the console WebSocket upgrades over it.
- `PVMSS_METRICS_PORT` stays plain HTTP. Keep it on a private network and
  restrict it with a NetworkPolicy or firewall rule.

## Quick start (Docker Compose)

Five minutes from nothing to an `mtls` PVMSS behind nginx, on one host.

**1. Create the certificates** (see [Building the certificates](#building-the-certificates) for the details):

```bash
mkdir -p tls && cd tls
# CA that signs the PVMSS server certificate
openssl req -x509 -newkey rsa:3072 -nodes -days 3650 -subj "/CN=PVMSS server CA" \
  -addext "basicConstraints=critical,CA:true" -addext "keyUsage=critical,keyCertSign,cRLSign" \
  -keyout server-ca.key -out server-ca.crt
# CA that signs the proxy's client certificate (dedicated, see "How it works")
openssl req -x509 -newkey rsa:3072 -nodes -days 3650 -subj "/CN=PVMSS client CA" \
  -addext "basicConstraints=critical,CA:true" -addext "keyUsage=critical,keyCertSign,cRLSign" \
  -keyout client-ca.key -out client-ca.crt
# PVMSS server certificate: the SAN is the name nginx dials
openssl req -newkey rsa:2048 -nodes -subj "/CN=pvmss" -keyout pvmss.key -out pvmss.csr
printf 'extendedKeyUsage=serverAuth\nsubjectAltName=DNS:pvmss\n' > server.ext
openssl x509 -req -in pvmss.csr -CA server-ca.crt -CAkey server-ca.key -CAcreateserial \
  -days 397 -extfile server.ext -out pvmss.crt
# nginx client certificate
openssl req -newkey rsa:2048 -nodes -subj "/CN=nginx-edge-01" -keyout proxy.key -out proxy.csr
printf 'extendedKeyUsage=clientAuth\n' > client.ext
openssl x509 -req -in proxy.csr -CA client-ca.crt -CAkey client-ca.key -CAcreateserial \
  -days 397 -extfile client.ext -out proxy.crt
rm -f ./*.csr ./*.ext ./*.srl
# PVMSS runs as uid 65532 in the image
chmod 644 pvmss.crt client-ca.crt server-ca.crt proxy.crt
chmod 640 pvmss.key proxy.key && sudo chgrp 65532 pvmss.key
cd ..
```

Keep `server-ca.key` and `client-ca.key` off the PVMSS and proxy hosts once
the certificates are issued.

**2. `docker-compose.yml`**

```yaml
services:
  pvmss:
    image: jhmmt/pvmss:0.4
    env_file: .env
    environment:
      PVMSS_TLS_CERT_FILE: /tls/pvmss.crt
      PVMSS_TLS_KEY_FILE: /tls/pvmss.key
      PVMSS_TLS_CLIENT_CA_FILE: /tls/client-ca.crt
      # One proxy hop in front of PVMSS (client IP and rate limits).
      PVMSS_TRUSTED_PROXY_HOPS: "1"
    volumes:
      - pvmss_data:/data
      - ./tls/pvmss.crt:/tls/pvmss.crt:ro
      - ./tls/pvmss.key:/tls/pvmss.key:ro
      - ./tls/client-ca.crt:/tls/client-ca.crt:ro
    # Not published: only nginx reaches it.
    expose: ["50000"]

  nginx:
    image: nginx:stable-alpine
    ports: ["443:443"]
    volumes:
      - ./nginx.conf:/etc/nginx/conf.d/default.conf:ro
      - ./tls/proxy.crt:/tls/proxy.crt:ro
      - ./tls/proxy.key:/tls/proxy.key:ro
      - ./tls/server-ca.crt:/tls/server-ca.crt:ro
      - ./site:/site:ro   # your public certificate for pvmss.example.com
    depends_on: [pvmss]

volumes:
  pvmss_data: {}
```

**3. `nginx.conf`**: the full [nginx recipe](#nginx) below.

**4. Check it**: `docker compose up -d`, then follow
[Checking a deployment](#checking-a-deployment). The `pvmss` logs must show
`tls=mtls` and no `tls_advice` warning.

## Building the certificates

### Rules for every certificate

| Certificate             | `extendedKeyUsage` | SAN                                                    | Signed by        | Suggested lifetime   |
| ----------------------- | ------------------ | ------------------------------------------------------ | ---------------- | -------------------- |
| PVMSS server            | `serverAuth`       | every name a proxy dials (`pvmss`, `pvmss.ns.svc`, ...) | server CA        | 90 days (automated) to 397 days |
| Proxy client (hop 2)    | `clientAuth`       | not needed; use a CN naming the proxy (`nginx-edge-01`) | **client CA**    | 90 days (automated) to 397 days |
| User client (hop 1)     | `clientAuth`       | not needed; CN or email naming the user                | user CA          | 1 day to 1 year, see [revocation](#revocation) |

- One client certificate **per proxy instance** (or per proxy fleet) lets you
  remove a single proxy by rotating the CA bundle without touching the others,
  and tells you in the PVMSS handshake logs which proxy was refused.
- The server CA and the client CA may be the same CA in a small setup, but a
  separate client CA is what keeps the trust narrow: anything the client CA
  signs gets in.
- Prefer 2048/3072-bit RSA or P-256 ECDSA keys.

### OpenSSL

The [quick start](#quick-start-docker-compose) script is the complete
OpenSSL path. To inspect what you issued:

```bash
openssl x509 -in pvmss.crt -noout -subject -issuer -enddate -ext subjectAltName,extendedKeyUsage
openssl verify -CAfile server-ca.crt pvmss.crt
openssl verify -CAfile client-ca.crt proxy.crt
```

### cert-manager

Two issuers: one for the server certificate (any issuer you already use) and
a **dedicated** CA issuer for client certificates.

```yaml
# Self-signed root used only to bootstrap the client CA.
apiVersion: cert-manager.io/v1
kind: Issuer
metadata: { name: pvmss-selfsigned, namespace: pvmss }
spec: { selfSigned: {} }
---
# The dedicated client CA (its Secret holds ca.crt, tls.crt, tls.key).
apiVersion: cert-manager.io/v1
kind: Certificate
metadata: { name: pvmss-client-ca, namespace: pvmss }
spec:
  isCA: true
  commonName: PVMSS client CA
  secretName: pvmss-client-ca
  duration: 87600h # 10 years
  privateKey: { algorithm: ECDSA, size: 256 }
  issuerRef: { name: pvmss-selfsigned, kind: Issuer }
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata: { name: pvmss-client-ca, namespace: pvmss }
spec:
  ca: { secretName: pvmss-client-ca }
---
# Client certificate for the Gateway / ingress controller. Create it in the
# namespace where the proxy reads its certificates.
apiVersion: cert-manager.io/v1
kind: Certificate
metadata: { name: pvmss-gateway-client, namespace: pvmss }
spec:
  commonName: gateway-pvmss
  secretName: pvmss-gateway-client
  duration: 2160h    # 90 days
  renewBefore: 360h  # 15 days
  usages: [client auth, digital signature, key encipherment]
  issuerRef: { name: pvmss-client-ca, kind: Issuer }
```

Point `tls.clientCA.secretName` at `pvmss-client-ca` (key `ca.crt`). Never at
the server certificate's Secret: its `ca.crt` is the server issuer.

### step-ca, Vault and others

Any PKI works as long as the certificates follow [the rules](#rules-for-every-certificate). Two common ones:

```bash
# step-ca: a 24 h proxy client certificate, renewed by a timer or `step ca renew --daemon`
step ca certificate nginx-edge-01 proxy.crt proxy.key --not-after 24h
step ca renew --daemon proxy.crt proxy.key --exec "nginx -s reload"

# Vault PKI: a role restricted to client auth
vault write pki_pvmss_client/roles/proxy allowed_domains=proxy.pvmss allow_subdomains=true \
  server_flag=false client_flag=true max_ttl=720h
vault write pki_pvmss_client/issue/proxy common_name=nginx-edge-01.proxy.pvmss ttl=720h
```

## Kubernetes with the Helm chart

The chart wires everything from `tls.*`:

| Value                               | Effect                                                                                     |
| ----------------------------------- | ------------------------------------------------------------------------------------------ |
| `tls.enabled`                       | Mounts the server certificate Secret at `/tls/server` and sets `PVMSS_TLS_CERT_FILE` / `_KEY_FILE`. Probes use HTTPS. |
| `tls.secretName`                    | An existing `kubernetes.io/tls` Secret (`tls.crt`, `tls.key`).                              |
| `tls.certManager.enabled`           | The chart creates a `Certificate` instead (`issuerRef`, `dnsNames`, `duration`, `renewBefore`). The default SANs are the Service DNS names. |
| `tls.clientCA.enabled`              | Turns on mTLS: mounts the client CA at `/tls/client-ca`, sets `PVMSS_TLS_CLIENT_CA_FILE` and `PVMSS_TLS_CLIENT_AUTH`. |
| `tls.clientCA.secretName`           | **Required** with `clientCA.enabled`: the Secret holding the dedicated client CA.          |
| `tls.clientCA.key`                  | Key of the PEM bundle in that Secret (default `ca.crt`).                                   |
| `tls.clientCA.clientAuth`           | `require` (default) or `optional`. With `require`, probes switch to `tcpSocket` because the kubelet has no client certificate. |
| `httpRoute.backendTLSPolicy.*`      | Makes the Gateway dial the pod over HTTPS and verify the PVMSS certificate (`hostname`, `caCertificateRefs` or `wellKnownCACertificates`). |

A complete `values.yaml` excerpt with cert-manager:

```yaml
tls:
  enabled: true
  certManager:
    enabled: true
    issuerRef: { name: internal-ca, kind: ClusterIssuer }
    # Default: the Service DNS names, which is what the Gateway dials.
    dnsNames: []
  clientCA:
    enabled: true
    secretName: pvmss-client-ca   # from the cert-manager example above
    key: ca.crt
    clientAuth: require

httpRoute:
  enabled: true
  backendTLSPolicy:
    enabled: true
    # A SAN of the PVMSS certificate.
    hostname: pvmss.pvmss.svc.cluster.local
    caCertificateRefs:
      - group: ""
        kind: ConfigMap
        name: internal-ca-bundle   # PEM of the issuer behind internal-ca

# Restart the pod when cert-manager renews the Secrets (stakater/Reloader).
extraAnnotations:
  reloader.stakater.com/auto: "true"
```

What the chart does not do for you:

- **The Gateway's own client certificate.** `BackendTLSPolicy` only covers
  server verification. How the Gateway presents a client certificate to the
  backend is controller-specific: for example a client certificate configured
  on the Gateway or in a backend policy of your controller (Envoy Gateway
  `BackendTLSPolicy` + `EnvoyProxy` client certificate, Istio
  `DestinationRule` with `tls.mode: MUTUAL`, Traefik `ServersTransport`).
  Use the `pvmss-gateway-client` Secret from the cert-manager example.
- **CA bundle for `caCertificateRefs`.** Gateway API expects a ConfigMap. With
  cert-manager, [trust-manager](https://cert-manager.io/docs/trust/trust-manager/)
  can publish the issuer's CA into one.
- **Probes in strict mTLS** are `tcpSocket`: they prove the port is open, not
  that `/health` answers. If you need a real health check, monitor `/health`
  through the proxy (see [Checking a deployment](#checking-a-deployment)).

## Proxy recipes, proxy to PVMSS

Each recipe is a complete working block, verified by `make proxy-e2e` in this
repository (`dev/proxies/`). Replace:

- `pvmss:50000` by the PVMSS address,
- `pvmss` in the server-name settings by a SAN of the PVMSS certificate,
- the `/tls/...` paths by yours: `proxy.crt`/`proxy.key` (client certificate),
  `server-ca.crt` (CA of the PVMSS server certificate).

Every recipe keeps the hop on HTTP/1.1 and raises the idle timeout, which the
console WebSocket needs.

### HAProxy

```text
frontend https
  bind :443 ssl crt /etc/haproxy/site.pem alpn h2,http/1.1
  default_backend pvmss

backend pvmss
  timeout tunnel 1h
  http-request set-header X-Forwarded-Proto https
  # proxy.pem = client certificate followed by its key
  server pvmss pvmss:50000 ssl verify required ca-file /tls/server-ca.crt crt /tls/proxy.pem sni str(pvmss) verifyhost pvmss alpn http/1.1
```

```bash
cat proxy.crt proxy.key > proxy.pem
```

### nginx

```nginx
map $http_upgrade $connection_upgrade {
  default upgrade;
  ''      close;
}

server {
  listen 443 ssl;
  http2 on;
  server_name pvmss.example.com;
  ssl_certificate     /site/fullchain.pem;
  ssl_certificate_key /site/privkey.pem;

  location / {
    proxy_pass https://pvmss:50000;
    proxy_http_version 1.1;
    proxy_set_header Host $http_host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
    proxy_buffering off;

    # mTLS to PVMSS
    proxy_ssl_certificate         /tls/proxy.crt;
    proxy_ssl_certificate_key     /tls/proxy.key;
    proxy_ssl_trusted_certificate /tls/server-ca.crt;
    proxy_ssl_verify on;
    proxy_ssl_verify_depth 2;
    proxy_ssl_server_name on;
    proxy_ssl_name pvmss;
    # Reuse TLS sessions to PVMSS instead of a full handshake per connection.
    proxy_ssl_session_reuse on;
  }
}
```

### Caddy

```caddy
pvmss.example.com {
  reverse_proxy https://pvmss:50000 {
    transport http {
      tls_client_auth /tls/proxy.crt /tls/proxy.key
      tls_trust_pool file /tls/server-ca.crt
      tls_server_name pvmss
      versions 1.1
    }
  }
}
```

### Traefik (file provider)

```yaml
http:
  routers:
    pvmss:
      rule: Host(`pvmss.example.com`)
      entryPoints: [websecure]
      service: pvmss
      tls: {}
  services:
    pvmss:
      loadBalancer:
        serversTransport: pvmss-mtls
        servers: [{ url: https://pvmss:50000 }]
  serversTransports:
    pvmss-mtls:
      serverName: pvmss
      rootCAs: [/tls/server-ca.crt]
      certificates: [{ certFile: /tls/proxy.crt, keyFile: /tls/proxy.key }]
      disableHTTP2: true
```

With the Kubernetes CRD provider, the same settings go in a `ServersTransport`
resource referenced by the `IngressRoute` service.

### Nginx Proxy Manager

1. Mount the certificates into the NPM container (`/tls`, read-only).
2. Proxy host: scheme `https`, host `pvmss`, port `50000`, *Websockets Support* on.
3. *Advanced* tab:

```nginx
proxy_ssl_certificate         /tls/proxy.crt;
proxy_ssl_certificate_key     /tls/proxy.key;
proxy_ssl_trusted_certificate /tls/server-ca.crt;
proxy_ssl_verify on;
proxy_ssl_server_name on;
proxy_ssl_name pvmss;
```

`dev/proxies/npm-setup.sh` does the same through the NPM API.

## Client certificates for users (optional)

### Should you?

Do it when access must be limited to **managed devices** and you already have
a way to issue, install and revoke one certificate per user or device. It adds
a device check before the PVMSS sign-in; it does not replace it.

### Sizing

| Users / devices | What changes                                                                                        |
| --------------- | --------------------------------------------------------------------------------------------------- |
| any number      | Nothing on PVMSS or on the proxy: validation is against the CA, the cost of a handshake is constant. |
| about 10        | Manual issuance with OpenSSL and a shared `.p12` procedure is workable.                             |
| 50 and more     | Manual issuance is not: use auto-enrollment or an MDM (below). Track who holds which certificate.    |

For example, 342 users means 342 certificates (or one per device), issued and
renewed automatically, and **one** CA file on the proxy.

### Distributing certificates

| Tool                                        | How                                                                                  |
| ------------------------------------------- | ------------------------------------------------------------------------------------ |
| Active Directory Certificate Services       | A *User* or *Workstation* template with *Client Authentication*, auto-enrolled by GPO. Windows and Edge/Chrome pick it from the user store. |
| Microsoft Intune                            | SCEP or PKCS certificate profile to Windows, macOS, iOS, Android.                    |
| Jamf Pro / other MDM                        | Certificate payload (SCEP or PKCS#12) in a configuration profile.                    |
| step-ca                                     | ACME, OIDC or SCEP provisioners; short-lived certificates renewed by `step` on the device. |
| Manual (few users)                          | `openssl pkcs12 -export -legacy` then import (below).                                |

Manual issuance, per user:

```bash
openssl req -newkey rsa:2048 -nodes -subj "/CN=alice/emailAddress=alice@example.com" \
  -keyout alice.key -out alice.csr
printf 'extendedKeyUsage=clientAuth\n' > client.ext
openssl x509 -req -in alice.csr -CA users-ca.crt -CAkey users-ca.key -CAcreateserial \
  -days 365 -extfile client.ext -out alice.crt
# -legacy: macOS Keychain cannot import OpenSSL 3's default PKCS#12 encryption.
openssl pkcs12 -export -legacy -name "PVMSS alice" -inkey alice.key -in alice.crt \
  -certfile users-ca.crt -out alice.p12
```

Send the `.p12` and its password through two different channels. The user
imports it in Keychain Access (macOS), `certmgr.msc` / double-click (Windows),
or *Settings > Privacy and security > Manage certificates* (Chrome/Edge on
Linux), *Settings > Certificates > Your Certificates* (Firefox).

### Proxy settings

| Proxy   | Require a browser client certificate                                                |
| ------- | ----------------------------------------------------------------------------------- |
| HAProxy | `bind :443 ssl crt site.pem ca-file /tls/users-ca.crt verify required`               |
| nginx   | `ssl_client_certificate /tls/users-ca.crt; ssl_verify_client on;`                   |
| Caddy   | `tls { client_auth { mode require_and_verify; trust_pool file /tls/users-ca.crt } }` |
| Traefik | `tls.options.default.clientAuth: { caFiles: [/tls/users-ca.crt], clientAuthType: RequireAndVerifyClientCert }` |
| NPM     | the nginx lines in *Advanced*, with a custom SSL certificate on the host            |

Use a **users CA**, separate from the client CA of hop 2: a user certificate
must not open a direct connection to PVMSS.

To log who connected, pass the certificate subject to your access log (nginx
`$ssl_client_s_dn`, HAProxy `%[ssl_c_s_dn]`). Do not forward it to PVMSS as a
header: PVMSS does not use it.

### Revocation

PVMSS does not check revocation; for hop 1 the proxy does. Pick one or both:

1. **Short-lived certificates** (hours to days, renewed automatically by
   step-ca, Intune or AD auto-enrollment). A lost laptop stops working when its
   certificate expires. This is the most reliable option.
2. **A CRL on the proxy**, refreshed by a job and followed by a reload:

| Proxy   | Setting                                                         |
| ------- | --------------------------------------------------------------- |
| nginx   | `ssl_crl /tls/users-ca.crl;`                                    |
| HAProxy | `bind ... ca-file /tls/users-ca.crt crl-file /tls/users-ca.crl verify required` |
| Traefik | not supported natively: use short-lived certificates            |
| Caddy   | not supported natively: use short-lived certificates            |

```bash
# Revoke alice with an OpenSSL CA (openssl ca needs an index; most PKIs do this for you)
openssl ca -config ca.cnf -revoke alice.crt
openssl ca -config ca.cnf -gencrl -out users-ca.crl
nginx -s reload
```

A CRL has a `nextUpdate` date: once past it, nginx refuses **every** client.
Regenerate it on a schedule well before that date, not only when you revoke.

In every case, disabling the user in Proxmox or in PVMSS still blocks the
sign-in, independently of the certificate.

## Renewal, rotation and monitoring

**PVMSS reads its certificates once.** After a renewal:

| Platform       | How to restart                                                                    |
| -------------- | --------------------------------------------------------------------------------- |
| Kubernetes     | stakater/Reloader (`extraAnnotations: { reloader.stakater.com/auto: "true" }`), or `kubectl -n pvmss rollout restart deploy/pvmss` |
| Docker Compose | `docker compose restart pvmss` from the renewal job                              |
| systemd        | `systemctl restart pvmss` in the renewal hook (`certbot --deploy-hook`, `step ca renew --exec`) |

Rotating the **client CA** without downtime:

1. Concatenate the old and new CA into `PVMSS_TLS_CLIENT_CA_FILE`, restart PVMSS.
2. Issue the proxies' new client certificates from the new CA, reload the proxies.
3. Remove the old CA from the bundle, restart PVMSS.

Rotating the **server certificate** needs nothing on the proxies as long as it
is signed by the CA they trust and keeps the same SAN.

**Monitoring expiry:**

```bash
# Days left on the certificate PVMSS actually serves (run from a host holding a client cert)
openssl s_client -connect pvmss:50000 -servername pvmss \
  -cert proxy.crt -key proxy.key -CAfile server-ca.crt </dev/null 2>/dev/null \
  | openssl x509 -noout -enddate
# Fails (exit 1) when a file expires within 15 days: use it in a cron or CI check
openssl x509 -in pvmss.crt -noout -checkend $((15*24*3600))
openssl x509 -in proxy.crt -noout -checkend $((15*24*3600))
```

- With cert-manager, alert on `certmanager_certificate_expiration_timestamp_seconds`
  and `certmanager_certificate_ready_status`.
- With Blackbox Exporter, `probe_ssl_earliest_cert_expiry` on a `tcp` module
  configured with the proxy's client certificate.
- Alert on the PVMSS log `event=tls_advice` to catch instances started
  without mTLS, and on a rising rate of `TLS handshake error` lines.

## Checking a deployment

Run these from a host that can reach PVMSS directly (not through the proxy).

```bash
# 1. Without a client certificate: must FAIL in mtls mode
curl -sS --cacert server-ca.crt https://pvmss:50000/health
#    expected: curl: (56) ... alert certificate required (or "handshake failure")

# 2. With the proxy's client certificate: must answer 200
curl -sS --cacert server-ca.crt --cert proxy.crt --key proxy.key \
  -o /dev/null -w '%{http_code}\n' https://pvmss:50000/health

# 3. Show the handshake details (protocol, server chain, requested client CAs)
openssl s_client -connect pvmss:50000 -servername pvmss -CAfile server-ca.crt \
  -cert proxy.crt -key proxy.key </dev/null

# 4. Through the proxy, as a user would
curl -sS -o /dev/null -w '%{http_code}\n' https://pvmss.example.com/health

# 5. Startup mode
docker compose logs pvmss | grep 'pvmss starting'        # tls=mtls
kubectl -n pvmss logs deploy/pvmss | grep -E 'pvmss starting|tls_advice'
```

Then open a VM console through the proxy: it exercises the WebSocket over the
mTLS hop, which plain HTTP checks do not.

## Troubleshooting

| Symptom                                                                                       | Cause and fix                                                                                                    |
| --------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| PVMSS does not start: `PVMSS_TLS_CERT_FILE and PVMSS_TLS_KEY_FILE must be set together`        | Only one of the two is set.                                                                                      |
| PVMSS does not start: `PVMSS_TLS_CERT_FILE/PVMSS_TLS_KEY_FILE: open ...: permission denied`    | The key is not readable by uid 65532 (image user). `chgrp 65532` + `chmod 640`, or `fsGroup: 65532` in Kubernetes. |
| PVMSS does not start: `... private key does not match public key`                              | Certificate and key are from different requests.                                                                 |
| PVMSS does not start: `PVMSS_TLS_CLIENT_CA_FILE holds no PEM certificate`                      | The file is DER, empty or a key. Convert with `openssl x509 -inform der -in ca.der -out ca.crt`.                 |
| Proxy returns `502`; PVMSS logs `tls: client didn't provide a certificate`                     | The proxy has no client certificate configured for the backend (hop 2).                                          |
| PVMSS logs `tls: failed to verify certificate: x509: certificate signed by unknown authority`  | The proxy's certificate is not signed by a CA in `PVMSS_TLS_CLIENT_CA_FILE`.                                     |
| PVMSS logs `... x509: certificate specifies an incompatible key usage`                         | The client certificate lacks `extendedKeyUsage=clientAuth`.                                                      |
| PVMSS logs `... certificate has expired or is not yet valid`                                   | Renew the proxy certificate; check the clocks (NTP) on both hosts.                                               |
| Proxy logs `upstream SSL certificate does not match "pvmss"` (nginx) or a hostname error       | `proxy_ssl_name` / `verifyhost` / `serverName` is not a SAN of the PVMSS certificate.                            |
| Proxy logs `upstream SSL certificate verify error` / `unknown authority`                       | The proxy's trusted CA (`proxy_ssl_trusted_certificate`, `ca-file`, `rootCAs`) is not the server CA.             |
| Proxy returns `400` or `502` and PVMSS logs `client sent an HTTP request to an HTTPS server`   | The proxy still targets `http://`. Use `https://`.                                                               |
| Browser: `400 No required SSL certificate was sent` (nginx) or `ERR_BAD_SSL_CLIENT_AUTH_CERT`  | Hop 1 mTLS is on and the browser has no valid user certificate (not imported, expired, revoked, CRL out of date). |
| Pods never become ready in Kubernetes                                                          | An `httpGet` probe under strict mTLS. The chart uses `tcpSocket` for `clientAuth: require`; check custom probes. |
| Everything works except the VM console (black screen, WebSocket `1006`)                        | The hop to PVMSS uses HTTP/2 or a short idle timeout. Force HTTP/1.1 and a 1 h tunnel/read timeout (recipes above). |

## FAQ

**Do I need one certificate per user for mTLS?** No. PVMSS's mTLS is between
the proxy and PVMSS: one client certificate per proxy. Per-user certificates
are only for the optional hop 1, see [above](#client-certificates-for-users-optional).

**Can PVMSS sign users in with their certificate?** No. The certificate gates
the connection; the PVMSS sign-in (Proxmox or local account) identifies the
user.

**Does mTLS slow PVMSS down?** No measurably. A full handshake happens when the
proxy opens a connection; keep-alive and session reuse
(`proxy_ssl_session_reuse on`) avoid repeating it per request.

**Can I terminate TLS on PVMSS without a proxy?** Yes, `tls` or `mtls` mode
exposes PVMSS directly. You then lose what the proxy provides (public
certificate automation, access logs, rate limiting in front, WAF). A proxy in
front is the recommended layout.

**Is the metrics port protected?** No, `PVMSS_METRICS_PORT` is plain HTTP.
Restrict it at the network level.

**Does PVMSS-to-Proxmox use this?** No. That connection is configured per
cluster in *Admin > Clusters* (CA, fingerprint).

**How do I test all of this locally?** `make proxy-e2e` starts five proxies in
front of PVMSS with mTLS on both hops and runs the console test through each
one. The dev PKI is in `dev/proxies/certs/` (see the README, "Behind a reverse
proxy").

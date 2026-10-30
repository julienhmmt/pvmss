# TLS and mutual TLS

PVMSS can serve plain HTTP, HTTPS, or HTTPS with mutual TLS (mTLS). Nothing is
forced, so it fits behind any reverse proxy or ingress. **Mutual TLS is strongly
recommended**: it means only the components you trust can reach PVMSS, even
from inside the network. Without it, any process that can open a TCP connection
to the pod or the host talks to the API directly and bypasses your proxy.

At startup PVMSS logs its mode in the `pvmss starting` banner (`tls`: `http`,
`tls` or `mtls`). For `http` and `tls` it also logs a `Warn` line with
`event=tls_advice`.

## Modes

| Mode   | Variables                                                                             | When to use                                                        |
| ------ | ------------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| `http` | none (default)                                                                        | TLS ends at the proxy, and the proxy-to-PVMSS hop is trusted (same pod, loopback) |
| `tls`  | `PVMSS_TLS_CERT_FILE`, `PVMSS_TLS_KEY_FILE`                                           | Encrypt the proxy-to-PVMSS hop, or expose PVMSS directly          |
| `mtls` | the two above + `PVMSS_TLS_CLIENT_CA_FILE` (+ `PVMSS_TLS_CLIENT_AUTH`)                | **Recommended.** Only clients with a certificate from your CA get in |

| Variable                   | Meaning                                                                                                    |
| -------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `PVMSS_TLS_CERT_FILE`      | PEM server certificate (full chain). Set together with the key.                                            |
| `PVMSS_TLS_KEY_FILE`       | PEM private key of that certificate.                                                                       |
| `PVMSS_TLS_CLIENT_CA_FILE` | PEM bundle of the CAs allowed to sign client certificates. Turns on mTLS.                                  |
| `PVMSS_TLS_CLIENT_AUTH`    | `require` (default): no valid client certificate, no connection. `optional`: a certificate is verified when sent, but clients without one still get in (migration step, reported as `tls`). |

TLS 1.2 is the minimum. Files are loaded at startup: a missing or invalid file
stops PVMSS with an explicit error. Certificates are read once, so restart PVMSS
after you renew them. A rejected handshake is logged at `Warn` with
`component=http`.

`PVMSS_METRICS_PORT` stays plain HTTP: keep it on a private network.

## Choosing where mTLS applies

```text
browser --(1)--> reverse proxy --(2)--> PVMSS
```

- **Hop 2 (proxy to PVMSS)** is what `PVMSS_TLS_*` protects. Give the proxy a
  client certificate signed by `PVMSS_TLS_CLIENT_CA_FILE` and have it verify
  the PVMSS server certificate.
- **Hop 1 (browser to proxy)** is the proxy's job. Requiring client
  certificates there restricts PVMSS to managed devices. It is optional and
  sits on top of PVMSS's own sign-in.

## Certificates

Use your PKI (an internal CA, cert-manager, Vault, step-ca). The server
certificate needs `extendedKeyUsage=serverAuth` and a SAN matching the name the
proxy dials. Client certificates need `extendedKeyUsage=clientAuth`; PVMSS
rejects a client certificate that only carries `serverAuth`.

A minimal OpenSSL example:

```bash
openssl req -x509 -newkey rsa:2048 -nodes -days 825 -subj "/CN=PVMSS CA" \
  -addext "basicConstraints=critical,CA:true" -addext "keyUsage=critical,keyCertSign,cRLSign" \
  -keyout ca.key -out ca.crt
openssl req -newkey rsa:2048 -nodes -subj "/CN=pvmss" -keyout pvmss.key -out pvmss.csr
printf 'extendedKeyUsage=serverAuth\nsubjectAltName=DNS:pvmss,DNS:pvmss.example.internal\n' > srv.ext
openssl x509 -req -in pvmss.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 825 -extfile srv.ext -out pvmss.crt
openssl req -newkey rsa:2048 -nodes -subj "/CN=proxy" -keyout proxy.key -out proxy.csr
printf 'extendedKeyUsage=clientAuth\n' > cli.ext
openssl x509 -req -in proxy.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 825 -extfile cli.ext -out proxy.crt
```

`dev/proxies/gen-cert.sh` builds the same PKI for the development stack.

## Running PVMSS with mTLS

```yaml
services:
  pvmss:
    environment:
      PVMSS_TLS_CERT_FILE: /tls/pvmss.crt
      PVMSS_TLS_KEY_FILE: /tls/pvmss.key
      PVMSS_TLS_CLIENT_CA_FILE: /tls/ca.crt
    volumes:
      - ./tls:/tls:ro
```

The image runs as uid 65532: the key file must be readable by it.

**Health checks.** In `mtls` mode, a checker that cannot present a client
certificate (a Kubernetes `httpGet` probe, for example) is refused. Use a
`tcpSocket` probe, or `PVMSS_TLS_CLIENT_AUTH=optional`, or keep `http` mode
with the proxy as a sidecar in the same pod. The Helm chart handles this: with
`tls.enabled` it mounts the certificate Secret (or issues it with
`tls.certManager`), sets `PVMSS_TLS_*`, probes `/health` over HTTPS, and
switches to a `tcpSocket` probe when `tls.clientCA.enabled` with
`clientAuth: require`. `tls.clientCA.secretName` is required and must hold a
CA dedicated to client certificates: reusing the server issuer's CA (a
cluster-wide ClusterIssuer, say) would let in every certificate it signs.
`httpRoute.backendTLSPolicy` makes the Gateway dial the
pod over HTTPS; the Gateway's client certificate is set on the controller side.

## Proxy recipes (hop 2)

These are the settings verified by `make proxy-e2e`. Replace the paths and the
`pvmss` name with yours. The `server name` must match a SAN in the PVMSS
certificate.

**HAProxy**

```text
server pvmss pvmss:50000 ssl verify required ca-file /tls/ca.crt crt /tls/proxy.pem sni str(pvmss) verifyhost pvmss alpn http/1.1
```

`proxy.pem` is the client certificate followed by its key.

**nginx**

```nginx
proxy_pass https://pvmss:50000;
proxy_ssl_certificate         /tls/proxy.crt;
proxy_ssl_certificate_key     /tls/proxy.key;
proxy_ssl_trusted_certificate /tls/ca.crt;
proxy_ssl_verify on;
proxy_ssl_server_name on;
proxy_ssl_name pvmss;
```

**Caddy**

```caddy
reverse_proxy https://pvmss:50000 {
  transport http {
    tls_client_auth /tls/proxy.crt /tls/proxy.key
    tls_trust_pool file /tls/ca.crt
    tls_server_name pvmss
    versions 1.1
  }
}
```

**Traefik** (file provider)

```yaml
http:
  services:
    pvmss:
      loadBalancer:
        serversTransport: pvmss-mtls
        servers: [{ url: https://pvmss:50000 }]
  serversTransports:
    pvmss-mtls:
      serverName: pvmss
      rootCAs: [/tls/ca.crt]
      certificates: [{ certFile: /tls/proxy.crt, keyFile: /tls/proxy.key }]
      disableHTTP2: true
```

**Nginx Proxy Manager**: scheme `https`, and in the proxy host's *Advanced*
tab the `proxy_ssl_*` lines of the nginx recipe (mount the certificates into
the NPM container).

Keep the proxy-to-PVMSS hop on HTTP/1.1 (`alpn http/1.1`, `versions 1.1`,
`disableHTTP2`): the console WebSocket upgrades over HTTP/1.1.

## Proxy recipes (hop 1, optional)

| Proxy   | Require a browser client certificate                                                |
| ------- | ----------------------------------------------------------------------------------- |
| HAProxy | `bind :443 ssl crt site.pem ca-file /tls/ca.crt verify required`                    |
| nginx   | `ssl_client_certificate /tls/ca.crt; ssl_verify_client on;`                         |
| Caddy   | `tls { client_auth { mode require_and_verify; trust_pool file /tls/ca.crt } }`      |
| Traefik | `tls.options.default.clientAuth: { caFiles: [/tls/ca.crt], clientAuthType: RequireAndVerifyClientCert }` |
| NPM     | the nginx lines in *Advanced*, with a custom SSL certificate on the host            |

Users then import a PKCS#12 bundle (`openssl pkcs12 -export -legacy ...`; the
`-legacy` flag is what macOS Keychain can import) into their browser or
keychain.

## Troubleshooting

| Symptom                                             | Cause                                                                     |
| --------------------------------------------------- | ------------------------------------------------------------------------- |
| Proxy gets `502` / handshake failure, PVMSS logs `tls: client didn't provide a certificate` | The proxy sends no client certificate (hop 2 not configured)              |
| PVMSS logs `certificate signed by unknown authority` | The client certificate is not from `PVMSS_TLS_CLIENT_CA_FILE`             |
| Proxy logs a hostname mismatch                      | The proxy's server name is not in the PVMSS certificate SAN               |
| `400 No required SSL certificate was sent` (nginx)  | Hop 1 mTLS is on and the browser has no client certificate imported       |
| Console stays black, the rest works                 | The hop to PVMSS negotiated HTTP/2; force HTTP/1.1 on it                  |

# Observability

For operators. What PVMSS logs, how to read it, how to turn on traces and
metrics, and three alerts worth having. Conventions for people changing the
code are in the "Logging and observability" section of `AGENTS.md`.

## Configuration

| Variable                      | Effect                                                                                                                                                  |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `LOG_LEVEL`                   | Startup level: `debug`, `info`, `warn`, `error` (lowercase). Required.                                                                                  |
| `LOG_FORMAT`                  | `json` for production (Loki, ELK, `jq`), `console` for a terminal. Required.                                                                            |
| `LOG_OUTPUT`                  | `stdout`, `stderr` or a file path. Required.                                                                                                            |
| `PVMSS_METRICS_PORT`          | Empty (default): no metrics listener. A port (1-65535, not `PVMSS_PORT`): serve `GET /metrics` on `PVMSS_HOST:<port>`.                                  |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Turns on OTLP/HTTP export of traces and metrics (for example `http://otel-collector:4318`). `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` also works for traces. |
| `OTEL_SERVICE_NAME`           | Service name on spans and metrics. Default `pvmss`.                                                                                                     |
| `OTEL_SDK_DISABLED`           | `true` forces telemetry export off even when an endpoint is set.                                                                                        |
| `OTEL_EXPORTER_OTLP_HEADERS`  | Auth headers for the collector. The OpenTelemetry SDK reads it; PVMSS never logs it.                                                                    |

The first four rows are validated at startup by `server/internal/config/load.go`
(the server refuses to boot on a bad value). The `OTEL_*` variables are the
standard OpenTelemetry ones and are read by the SDK itself. With no OTLP
endpoint there is no exporter, no extra goroutine and no network traffic.

### Changing the level without a restart

An admin can switch the level in **Admin > Settings > Log level**, or with
`PUT /api/v1/admin/ops/log-level` and `{"level": "debug"}`. It takes effect at
once, is audited, and is not persisted: a restart returns to `LOG_LEVEL`. At
`debug` the source file and line are added to each record.

## What each level shows

| Level   | What you see                                                                                                                          |
| ------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| `error` | Something is broken: a 5xx (from the access log), a database failure, a cluster that just became unreachable.                         |
| `warn`  | Degraded or refused: failed logins, 403, CSRF rejects, 429, a cluster that is still down (reminder every 5 minutes), Proxmox retries. |
| `info`  | Startup banner, "server ready", shutdown, logins and logouts, every write request, every audited action, cluster recovery.            |
| `debug` | Every GET request, every Proxmox call (templated path, status, duration), every successful inventory refresh, and any request the client abandoned (`result=canceled`, whatever its status). |

Every request produces one access-log line (`message` = `http request`) with
`method`, `route` (the route pattern, never the URL or query string),
`status`, `bytes`, `durationMs`, `clientIp`, `requestId`, `user` when
authenticated, `cluster` and `vmid` when the route has them, and `error` when a
handler attached a cause. Every audit-log row is also logged (`message` =
`audit event`, `event` = `audit`) with `actor`, `action`, `cluster`, `vmid`,
`targetType`, `targetId`. Request bodies, headers and secrets are never logged.

VM migration (admin) adds two lines. `vm.migrate` is an audited action: one
`audit event` line at `info` (`actor`, `cluster`, `vmid`, `targetType` = `vm`,
`targetId` = `<cluster>:<vmid>`), written only after Proxmox accepts the
dispatch; the audit row's `detail` JSON carries the source and target node
(`{"summary":"migrate <source> -> <target>","changes":[{"field":"node",...}]}`).
It is counted in `pvmss_vm_actions_total` under `action="vm.migrate"`, with no
new label. A migration PVMSS refuses before dispatching (invalid target,
locked VM, live-migration blocker) logs one `vm migration refused` line at
`warn` with `component`, `cluster`, `vmid` and `reason` (`invalid_target`,
`vm_locked` or `migration_blocked`) and writes no audit row.

The response header `X-Request-Id` carries the request ID, so a user can quote
it. A valid inbound `X-Request-Id` (1-64 characters of `A-Za-z0-9._-`) is kept.

Browser-side failures are logged too: the SPA posts uncaught errors, unhandled
rejections and SvelteKit errors to `POST /api/v1/client-errors`, which logs
them at `warn` as `client error reported` with `event` = `client_error`, the
error `message`, the SPA `path`, and the `stack` when present. The endpoint is
unauthenticated (a pre-login error must still report) and per-IP rate limited
at 60/min, so a loop of errors cannot lock out logins or flood the log.

The startup banner (`pvmss starting`) states the version, commit, Go version,
cluster source and names, log level and format, listen address, whether OTel is
on (and the collector host only), and the metrics address.

## Reading the logs

JSON keys `timestamp`, `level` and `message` are fixed. Examples use `jq` on
`LOG_FORMAT=json` output.

One user's actions (audited actions, then every write they sent):

```bash
jq -c 'select(.event == "audit" and .actor == "alice@pve")' pvmss.log
jq -c 'select(.message == "http request" and .user == "alice@pve" and .method != "GET")' pvmss.log
```

5xx responses by route, most frequent first:

```bash
jq -r 'select(.message == "http request" and .status >= 500) | .route' pvmss.log | sort | uniq -c | sort -rn
```

A cluster going down and coming back:

```bash
jq -c 'select(.message | test("^cluster (unreachable|still unreachable|recovered)$")) | {timestamp, message, cluster, downForMs}' pvmss.log
```

Everything about one request:

```bash
jq -c 'select(.requestId == "3f9c0b1a2d4e4b7a9c1d2e3f4a5b6c7d")' pvmss.log
```

The same in Loki (LogQL), assuming the pod is selected by `{app="pvmss"}`:

```logql
{app="pvmss"} | json | event="audit" | actor="alice@pve"
sum by (route) (count_over_time({app="pvmss"} | json | message="http request" | status >= 500 [1h]))
{app="pvmss"} | json | message=~"cluster (unreachable|still unreachable|recovered)"
{app="pvmss"} | json | requestId="3f9c0b1a2d4e4b7a9c1d2e3f4a5b6c7d"
```

## Ship logs to a SIEM

PVMSS can send the same records it writes to stdout to a syslog receiver over
TLS. The sink is off until `PVMSS_SYSLOG_ADDR` is set. Stdout always stays on:
the sink adds a copy, it never replaces one.

### SIEM sink configuration

| Variable                     | Effect                                                                                             |
| ---------------------------- | -------------------------------------------------------------------------------------------------- |
| `PVMSS_SYSLOG_ADDR`          | `host:port` of the receiver (RFC 5425 port, usually 6514). Empty (default): sink off.              |
| `PVMSS_SYSLOG_TLS_CA_FILE`   | PEM CA that signed the receiver's certificate. Empty: the system roots. Verification is always on. |
| `PVMSS_SYSLOG_TLS_CERT_FILE` | Client certificate for mutual TLS. Set together with the key.                                      |
| `PVMSS_SYSLOG_TLS_KEY_FILE`  | Client key for mutual TLS. Set together with the cert.                                             |

The TLS variables require `PVMSS_SYSLOG_ADDR`, and the server refuses to boot
on a bad address, a missing file or a cert without its key. See `docs/tls.md`
for building the CA and the client pair.

### Wire format

Each record is one RFC 5424 message in an RFC 5425 octet-counting frame over
TLS: a decimal byte count, a space, then the message. There are no newline
separators inside the TCP stream, so a receiver must read by count, not by
line. The receiver in the example below writes each message on its own line.

One real line, captured from the dev receiver after a login:

```text
<134>1 2026-10-30T23:09:44.245828Z ac1c7c9d25fe pvmss 1 auth [pvmss@32473 level="INFO" requestId="5d2b14f95138d10d6b5616bf24339294" clientIp="192.168.107.4" component="httpapi" event="auth" result="success" user="admin@pve"] login succeeded
```

Reading it: `<134>` is facility local0 and severity 6 (info). The APP-NAME is
`pvmss`. The MSGID is the `event` attribute (`auth` here) or `-` when a record
has none. The structured data ID `pvmss@32473` carries the flattened
attributes, `level` first. The message text follows the brackets.

### Parsing it

- **rsyslog**: `imtcp` with `StreamDriver.Mode="1"` and `Name="gtls"` reads
  octet-counted frames by itself. Use `%rawmsg%` to store the line as received.
- **Splunk**: a TCP or TLS data input with `sourcetype=syslog`. Splunk reads
  the RFC 5424 header; the `[pvmss@32473 ...]` block stays as text, so extract
  `key="value"` pairs with a `rex` or a props `EXTRACT`.
- **Wazuh**: a `<remote>` block with `connection="secure"` and `protocol="tcp"`
  and a decoder that reads the SD block, or a custom decoder for `pvmss`.
- **Elastic**: the `syslog` input of Elastic Agent or Filebeat, or Logstash
  with the `syslog` codec. Filter on `program` = `pvmss` and parse the SD with
  `kv`.
- **Microsoft Sentinel**: a Syslog connector on the Log Analytics agent or the
  AMA, with the same TLS receiver. Facility `local0`, severities as above.

### What is sent

- Every record that passes `LOG_LEVEL`, so the same levels as stdout. There is
  no separate SIEM level. `info` is the production level; `debug` adds one line
  per GET and per Proxmox call and multiplies the volume.
- Attributes are flattened to `group.key` and redacted by the same rule as
  stdout. Secret-looking values never reach the SIEM.
- Not sent: request bodies, headers, raw URLs and query strings, session or
  CSRF tokens, passwords. The sink never adds fields that stdout lacks.
- Records longer than 8192 bytes are cut at the limit and marked
  `truncated="true"` in the SD block.

### TLS and mTLS

Create a CA and a receiver certificate on the receiver side, give PVMSS the
CA file, and point `PVMSS_SYSLOG_ADDR` at the receiver's name. The certificate
must carry that name as a SAN, since PVMSS checks the host it dialed. For
mutual TLS, issue a client certificate from the same CA and set
`PVMSS_SYSLOG_TLS_CERT_FILE` and `PVMSS_SYSLOG_TLS_KEY_FILE`.

### Drops and the `syslog_drop` warning

The sink uses a queue of 1024 records. The write path only enqueues and never
blocks, so a slow or down receiver cannot slow requests. When the queue is
full, or a write fails, the record is dropped from the SIEM copy (stdout keeps
it). A warning with `event="syslog_drop"` and a running `dropped` count is
written to stdout at most once a minute. Treat it as a receiver outage or
undersized network: check the receiver and the path, then the count stops
rising. On shutdown the sink drains for up to 2 seconds.

Connections are dialed lazily and redialed after a failure, with backoff from
1 second up to 30 seconds.

### Volume and level

Expect one line per write request and per audited action, plus one per login,
logout and cluster state change at `info`. Inventory refreshes and GETs appear
only at `debug`. Size the receiver for the `info` rate of your user count, and
keep `LOG_LEVEL=info` in production.

### Kubernetes

On Kubernetes, a collector that reads the pod's stdout is still the
recommended path: it keeps the pod free of network dependencies and the
Helm chart defaults to `LOG_FORMAT: json`. The stdout records have the JSON
keys `timestamp`, `level`, `message`, plus the attributes. Both snippets
below parse those keys and send them on.

Vector (`sources.pvmss` reads the container logs, `transforms.parse` decodes
the JSON):

```yaml
sources:
  pvmss:
    type: kubernetes_logs
    extra_label_selector: "app.kubernetes.io/name=pvmss"
transforms:
  parse:
    type: remap
    inputs: [pvmss]
    source: |
      . = merge!(., parse_json!(.message))
sinks:
  siem:
    type: syslog
    inputs: [parse]
    mode: tcp
    address: "siem.example.internal:6514"
    encoding:
      codec: json
    tls:
      enabled: true
      ca_file: /etc/vector/siem-ca.crt
```

Fluent Bit (a `parser` for the JSON body, then the `syslog` output):

```ini
[INPUT]
    Name              tail
    Tag               pvmss.*
    Path              /var/log/containers/*pvmss*.log
    multiline.parser  docker, cri

[FILTER]
    Name         parser
    Match        pvmss.*
    Key_Name     log
    Parser       json
    Reserve_Data On

[OUTPUT]
    Name           syslog
    Match          pvmss.*
    Host           siem.example.internal
    Port           6514
    Mode           tls
    tls            on
    tls.ca_file    /fluent-bit/etc/siem-ca.crt
    Syslog_Format  rfc5424
    Syslog_Message_Key  log
```

Adjust the `Match` and selector labels to your install. Both snippets are
untested against your receiver; check the first record with the receiver
before relying on them.

### Helm chart

The chart mounts volumes and sets env through `deployment.extraVolumes` and
`deployment.extraEnv`. Mount the receiver CA from a Secret and point the
variables at it:

```yaml
deployment:
  extraVolumes:
    - name: siem-ca
      secret:
        secretName: pvmss-siem-ca
  extraVolumeMounts:
    - name: siem-ca
      mountPath: /etc/pvmss/siem
      readOnly: true
  extraEnv:
    - name: PVMSS_SYSLOG_ADDR
      value: "siem.example.internal:6514"
    - name: PVMSS_SYSLOG_TLS_CA_FILE
      value: /etc/pvmss/siem/ca.crt
```

### Dev receiver

`docker-compose.dev.yml` has an optional rsyslog receiver under the `siem`
profile. It writes the lines to the `syslog_data` volume:

```sh
PVMSS_SYSLOG_ADDR=syslog:6514 PVMSS_SYSLOG_TLS_CA_FILE=/syslog-tls/ca.crt \
  docker compose -f docker-compose.dev.yml --profile siem up -d --build pvmss-dev syslog
docker exec pvmss-syslog cat /var/log/siem/pvmss.log
```

## Traces

Set `OTEL_EXPORTER_OTLP_ENDPOINT` and restart. PVMSS then sends:

- one server span per API request, named by route (`GET /api/v1/vms/{cluster}/{vmid}`);
  `/health` and static assets are not traced;
- a client span per Proxmox call, named by templated path (`GET /nodes/{node}/qemu/{vmid}/status/current`),
  as a child of the request span;
- one `inventory.refresh` span per refresh cycle.

An inbound W3C `traceparent` header is honoured. Log lines written inside a
request carry `traceId` and `spanId`, so a trace links to its logs and back.

A minimal OpenTelemetry Collector that receives it and forwards to a backend:

```yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318
exporters:
  otlp:
    endpoint: tempo:4317
    tls:
      insecure: true
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp]
```

Then run PVMSS with `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318`.
Logs stay on stdout; there is no OTLP log export.

## Metrics

Set `PVMSS_METRICS_PORT` (for example `9464`) to serve Prometheus metrics at
`http://<pod>:9464/metrics`. Setting an OTLP endpoint also pushes the same
metrics over OTLP. The metrics port has **no authentication** and is not routed
by the HTTPRoute: keep it cluster-internal and, on a shared cluster, restrict
it with a NetworkPolicy. The main port answers `/metrics` with 404.

| Metric (Prometheus name)                    | Type              | Labels                                                           |
| ------------------------------------------- | ----------------- | ---------------------------------------------------------------- |
| `http_server_request_duration_seconds`      | histogram         | `http_route`, `http_request_method`, `http_response_status_code` |
| `pvmss_inventory_refresh_duration_seconds`  | histogram         | `cluster`                                                        |
| `pvmss_inventory_refresh_failures_total`    | counter           | `cluster`                                                        |
| `pvmss_inventory_last_success_seconds`      | gauge (unix time) | `cluster`                                                        |
| `pvmss_proxmox_request_duration_seconds`    | histogram         | `cluster`, `method`, `status_class`                              |
| `pvmss_vms`                                 | gauge             | `cluster`, `status`                                              |
| `pvmss_vm_actions_total`                    | counter           | `action`, `result`                                               |
| `pvmss_auth_logins_total`                   | counter           | `result`                                                         |
| `go_*` (Go runtime: memory, GC, goroutines) | various           | none                                                             |

Labels are limited to a fixed allowlist; `vmid`, `user` and `node` are never
labels.

### Helm

```yaml
metrics:
  enabled: true
  port: 9464
  serviceMonitor:
    enabled: true # needs the Prometheus Operator CRDs
    interval: 30s
```

This sets `PVMSS_METRICS_PORT`, adds a `metrics` container and Service port,
and renders a ServiceMonitor.

### Example alerts

Inventory has not refreshed for 10 minutes (the refresh interval is 30 seconds
by default):

```yaml
- alert: PVMSSInventoryStale
  expr: time() - pvmss_inventory_last_success_seconds > 600
  for: 5m
  labels: { severity: warning }
  annotations:
    summary: "Inventory for cluster {{ $labels.cluster }} is stale"
```

More than 5% of API requests failing with 5xx:

```yaml
- alert: PVMSSHighErrorRate
  expr: |
    sum(rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[5m]))
      / sum(rate(http_server_request_duration_seconds_count[5m])) > 0.05
  for: 10m
  labels: { severity: critical }
  annotations:
    summary: "PVMSS is answering more than 5% of requests with 5xx"
```

More than 10% of Proxmox calls failing (5xx or no response) for a cluster:

```yaml
- alert: PVMSSProxmoxErrors
  expr: |
    sum by (cluster) (rate(pvmss_proxmox_request_duration_seconds_count{status_class=~"5xx|error"}[5m]))
      / sum by (cluster) (rate(pvmss_proxmox_request_duration_seconds_count[5m])) > 0.1
  for: 10m
  labels: { severity: warning }
  annotations:
    summary: "Proxmox calls to {{ $labels.cluster }} are failing"
```

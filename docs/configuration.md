# Configuration

Environment variables only, validated at startup by
`server/internal/config/load.go` (fails fast).

**Required:**

| Variable               | Notes                                                       |
| ---------------------- | ----------------------------------------------------------- |
| `PVMSS_PORT`           | Integer 1-65535                                             |
| `PVMSS_DB_PATH`        | SQLite file path (image default `/data/pvmss.db`)           |
| `SESSION_SECRET`       | 32+ bytes                                                   |
| `LOG_LEVEL`            | `debug` \| `info` \| `warn` \| `error` - **lowercase only** |
| `LOG_FORMAT`           | `json` \| `console`                                         |
| `LOG_OUTPUT`           | `stdout` \| `stderr` \| a file path                         |
| `PVMSS_CLUSTER_SOURCE` | `fake` \| `proxmox` - no default on purpose                 |

`PVMSS_CLUSTER_SOURCE` has no default because `fake` ships hardcoded demo
credentials (`admin@pve` / `pvmss-admin`).

**Optional when `PVMSS_CLUSTER_SOURCE=proxmox`:** `PROXMOX_URL`,
`PROXMOX_API_TOKEN_NAME`, `PROXMOX_API_TOKEN_VALUE` are all-or-nothing. All
three set: they seed the `default` cluster on first boot only; afterwards the
admin UI owns clusters (add, edit, remove, and rename the id - the rename
rewrites every cluster-keyed row, audit history included, and old
`?cluster=` links stop working).
All three empty: the server boots with zero clusters
and serves the first-run setup (`/setup`) until an administrator registers the
first cluster there. A partial set still fails startup.

**Optional:**

| Variable                                      | Default                                                         |
| --------------------------------------------- | --------------------------------------------------------------- |
| `PVMSS_HOST`                                  | `127.0.0.1` (image sets `0.0.0.0`)                              |
| `PVMSS_WEB_DIR`                               | relative to the executable                                      |
| `ADMIN_PASSWORD_HASH`                         | empty; if set, must be `$2...`                                  |
| `PVMSS_COOKIE_SECURE`                         | `true`                                                          |
| `PVMSS_INVENTORY_REFRESH_INTERVAL`            | `30s`                                                           |
| `PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL` | `5s`                                                            |
| `PVMSS_INVENTORY_REFRESH_TIMEOUT`             | `15s`                                                           |
| `PVMSS_MAX_LIST_PAGE_SIZE`                    | `100`                                                           |
| `PVMSS_MAX_LOCAL_ADMINS`                      | `5` (range 1-50; excludes the bootstrap admin)                  |
| `PVMSS_DB_RESET_ON_INCOMPATIBLE`              | `false` (dev compose: `true`; old DB is renamed, never deleted) |
| `PVMSS_TRUSTED_PROXY_HOPS`                    | `0`                                                             |
| `PVMSS_RATE_LIMIT_MAX`                        | `0` (built-in ceilings; positive raises them, used by e2e)      |
| `PVMSS_METRICS_PORT`                          | empty (off); a port != `PVMSS_PORT` enables `GET /metrics`      |
| `PVMSS_TLS_CERT_FILE` / `PVMSS_TLS_KEY_FILE`  | empty (plain HTTP); both set = HTTPS listener                   |
| `PVMSS_TLS_CLIENT_CA_FILE`                    | empty; set = mutual TLS (advised, `docs/tls.md`)                |
| `PVMSS_TLS_CLIENT_AUTH`                       | `require` (or `optional`); needs the client CA                  |
| `PVMSS_SYSLOG_ADDR`                           | `host:port` of the SIEM receiver (RFC 5425 TLS). Empty: off.    |
| `PVMSS_SYSLOG_TLS_CA_FILE`                    | PEM CA for the receiver certificate; empty uses system roots.   |
| `PVMSS_SYSLOG_TLS_CERT_FILE`                  | Client cert for mutual TLS; needs `PVMSS_SYSLOG_TLS_KEY_FILE`.  |
| `PVMSS_SYSLOG_TLS_KEY_FILE`                   | Client key for mutual TLS; needs `PVMSS_SYSLOG_TLS_CERT_FILE`.  |

Tracing/OTLP use the standard `OTEL_*` variables read by the SDK (see
`docs/observability.md`). Setting a trace endpoint also turns on browser
telemetry: `POST /api/v1/traces` proxies the SPA's page-view spans to the
collector and `GET /api/v1/public/version` advertises it to the client, so an
unset endpoint means the browser sends nothing. `LOG_LEVEL` is only the
startup level; admins can change it at runtime (Admin > Settings).

**Administrator sign-in.** There are two paths. A Proxmox user is an
administrator when their ACL grants `Permissions.Modify` on `/` (the
`PVEAdmin` or `Administrator` role); only the `pve` and `pam` realms are
supported, OIDC is not. PVMSS local administrators are independent of any
cluster. `ADMIN_PASSWORD_HASH` seeds the immutable bootstrap owner (`admin`,
provider `bootstrap`); DB-backed accounts (`local_accounts`, capped by
`PVMSS_MAX_LOCAL_ADMINS`) are created and managed by that owner under Admin >
Settings > Accounts. When no `ADMIN_PASSWORD_HASH` exists, the account created
through `/setup` (`created_by = 'setup'`) is the owner instead. Local
administrators get the full admin surface but every account route answers
`owner_only` to them. Local passwords are 8+ characters and at most 72
bytes (bcrypt); five consecutive failures lock an account for 15 minutes.
The bootstrap admin is never locked: five failures from one client IP refuse
that IP (429) for 15 minutes. Resetting, disabling or deleting an account revokes its sessions.

No longer read (startup warns for the SSH ones): `PVMSS_OFFLINE`, `PVMSS_ENV`,
`JWT_SECRET`, `PROXMOX_VERIFY_SSL`, `LOG_FILE_PATH`, `PVMSS_SSH_KEY_FILE`,
`PVMSS_SSH_USER`, `PVMSS_SSH_PORT`.

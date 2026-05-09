# Feature Plan: Shared Resty Client with Connection Pooling

## Goal

Optimize backend resource usage and improve API latency by preventing the continuous recreation of HTTP clients, taking advantage of TCP Keep-Alive and connection pooling.

## Current State

Depending on the current implementation, the backend may be creating new Resty clients (`resty.New()`) for different modules or, in worst cases, per request. This causes overhead due to repeated TCP handshakes and TLS negotiations with the Proxmox API.

## Backend Implementation

1. **Singleton Client Initialization**:
   - Create a centralized, singleton instance of `*resty.Client` at application startup (e.g., within `main.go` or initialized inside `state/manager.go`).
   - This client will be shared across all Proxmox API communication handlers.

2. **Transport & Connection Pooling Tuning**:
   - Configure the underlying `http.Transport` to reuse connections efficiently.
   - Example configuration:

     ```go
     transport := &http.Transport{
         MaxIdleConns:          100,
         MaxIdleConnsPerHost:   20,
         IdleConnTimeout:       90 * time.Second,
         TLSHandshakeTimeout:   10 * time.Second,
         ExpectContinueTimeout: 1 * time.Second,
         // Skip TLS verify if configured
         TLSClientConfig:       &tls.Config{InsecureSkipVerify: skipVerify},
     }
     
     sharedClient := resty.New()
     sharedClient.SetTransport(transport)
     sharedClient.SetTimeout(30 * time.Second)
     ```

3. **Global Configurations**:
   - Apply global settings to the shared Resty client, such as base URLs, default headers, and standard retry mechanisms.
   - Example:

     ```go
     sharedClient.SetRetryCount(3)
     sharedClient.SetRetryWaitTime(1 * time.Second)
     ```

4. **Dependency Injection**:
   - Refactor handlers and aggregator functions to accept the shared `*resty.Client` as a parameter (or access it via the state manager), rather than instantiating their own.
   - Example: `proxmox.GetVMsResty(ctx, sharedClient)`

## Frontend Implementation

- No frontend changes are required. The frontend will implicitly benefit from faster API response times.

## Challenges & Considerations

- **Concurrency Safety**: The `go-resty` client is thread-safe for concurrent request execution, but modifying global client settings (like base URL or timeouts) *after* initialization is not thread-safe. All client configuration must happen once at startup. Request-specific configurations should happen at the request level (`client.R().SetHeader(...)`).
- **Memory Leaks**: Ensure that response bodies are fully read and closed (Resty handles this automatically for the most part) so that connections are successfully returned to the idle pool.

## Tasks

### Phase A — Shared Transport (DONE — 2026-05-09)

- [x] Add `backend/proxmox/shared_transport.go` with `getSharedTransport(insecureSkipVerify bool) *http.Transport` (lazy, sync-guarded, keyed by skip-verify).
- [x] Tune transport: `MaxIdleConns=100`, `MaxIdleConnsPerHost=50`, `IdleConnTimeout=90s`, `TLSHandshakeTimeout=10s`, `ExpectContinueTimeout=1s`, `ResponseHeaderTimeout=15s`, HTTP/2, keepalive dialer, TLS ≥1.2.
- [x] Wire `constants.HTTPMaxIdleConns`, `HTTPMaxIdleConnsPerHost`, `HTTPIdleConnTimeout`, `HTTPTLSHandshakeTimeout`, `HTTPExpectContinueTimeout`, `HTTPResponseHeaderTimeout` (previously dead).
- [x] `MakeRestyClient`: `client.SetTransport(getSharedTransport(insecureSkipVerify))`; remove per-call `SetTLSClientConfig`.
- [x] `MakeRestyClientCookieAuth`: same.
- [x] Drop unused `crypto/tls` import in `resty_client.go`.
- [x] Verify `pvmss/proxmox` tests pass.

### Phase B — Singleton Token Client + DI Refactor

- [ ] Add `proxmox.SharedTokenClient(cfg *envpkg.EnvConfig) (*RestyClient, error)` — `sync.Once`-guarded singleton built from `EnvConfig`. Client-level `SetTimeout` set to a generous ceiling (e.g. 60s); per-request deadlines flow via `context.Context`.
- [ ] Add `proxmox.MustSharedTokenClient()` accessor (returns initialized singleton; panics if not initialized).
- [ ] Initialize singleton at startup in `main.go` after `EnvConfig` validated, before HTTP server starts. Skip when `PVMSS_OFFLINE=true`.
- [ ] Confirm `*resty.Client` request execution is concurrency-safe; document that no post-init mutation of client-level config is allowed (only `R().Set...` per request).
- [ ] Audit all `Get/Post/PostEmpty/Put/Delete` methods on `RestyClient` — ensure they propagate caller `ctx` for per-request timeout (already true). No client-level timeout mutation post-init.
- [ ] Replace `MakeRestyClientFromEnv(timeout)` callsites with shared singleton + `context.WithTimeout(ctx, timeout)`:
  - [ ] `state/manager_cache.go` (×2)
  - [ ] `state/manager_proxmox.go`
  - [ ] `api/v1/vms.go`
  - [ ] `api/v1/vm_actions.go`
  - [ ] `api/v1/vnc.go`
  - [ ] `api/v1/admin_vms.go` (×4)
  - [ ] `api/v1/admin_handlers.go` (×5)
  - [ ] `api/v1/admin_mutations.go` (×6)
  - [ ] `api/v1/setup.go` (×2)
  - [ ] `handlers/limits_helpers.go`
  - [ ] `handlers/resty_helper.go`
- [ ] Deprecate `MakeRestyClientFromEnv` / `MakeRestyClientFromEnvConfig` (keep as thin shims that return the singleton, log deprecation in dev).
- [ ] Decide cookie-auth path:
  - [ ] Keep `MakeRestyClientCookieAuth` per-request (cookies are per-user) but ensure it uses shared transport (already done in Phase A).
  - [ ] OR add cookie-jar-aware singleton + per-request `R().SetCookie(...)`. Document chosen approach.
- [ ] Update `api/v1/auth.go` (×2) cookie-auth callsites accordingly.
- [ ] Add unit test: shared transport reused across multiple `MakeRestyClient` calls (assert pointer equality of transport).
- [ ] Add unit test: `SharedTokenClient` returns same `*RestyClient` across goroutines (concurrent calls).
- [ ] Add benchmark `BenchmarkSharedClientReuse` comparing pre/post handshake cost (optional, integration-tagged).

### Phase C — Verification

- [ ] `make test-offline-race` passes.
- [ ] `make go-lint` clean (no new warnings).
- [ ] Manual smoke: live Proxmox, observe connection reuse via `netstat`/`ss` — connections to Proxmox host stay in `ESTABLISHED` across requests instead of cycling.
- [ ] Measure: median latency on `/api/v1/vms` before/after under sustained load (k6 or `hey`). Target: ≥30% reduction on warm pool.
- [ ] Update `CLAUDE.md` if singleton access pattern becomes the documented norm.

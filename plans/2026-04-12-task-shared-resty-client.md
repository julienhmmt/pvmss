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

### Phase B — Singleton Token Client + DI Refactor (DONE — 2026-05-09)

Approach taken: rather than introduce a new `SharedTokenClient` accessor and refactor 30+ callsites, the existing `MakeRestyClient` factory was made internally idempotent. It now returns a wrapper around a process-wide `*resty.Client` keyed by `(baseURL, tokenID, tokenSecret, insecureSkipVerify)`. Per-call timeouts are enforced via `context.WithTimeout` inside `Get/Post/PostEmpty/Put/Delete`, so each callsite keeps its own deadline without mutating the shared client. This preserves the public API and avoids touching every handler.

- [x] Singleton cache `tokenClients map[string]*resty.Client` keyed by SHA256 of `(baseURL, tokenID, tokenSecret) | insecureSkipVerify`, guarded by `sync.Mutex`.
- [x] `MakeRestyClient` reuses cached `*resty.Client`; first call builds via `buildTokenClient`, subsequent calls return same instance.
- [x] Client-level `SetTimeout` removed for token client (no post-init config mutation). Per-request deadline applied via `RestyClient.withRequestTimeout` helper.
- [x] All `Get/Post/PostEmpty/Put/Delete` wrap caller ctx via `withRequestTimeout` honoring tighter caller deadlines.
- [x] Cookie-auth path: kept per-call (per-user cookies must not leak). Already shares transport via Phase A.
- [x] `interface{}` → `any` in `RestyClient` request methods (clears prior linter notices).
- [x] `ResetTokenClients()` test helper exported.
- [x] Unit tests in `resty_client_test.go`:
  - `TestMakeRestyClient_ReusesSingleton` — same config returns shared `*resty.Client`, distinct wrapper timeouts.
  - `TestMakeRestyClient_DistinctConfigsIsolated` — distinct token secrets get distinct clients.
  - `TestMakeRestyClient_ConcurrentReturnsSameClient` — 32 goroutines, race detector clean.
  - `TestSharedTransport_ReusedAcrossClients` — same `insecureSkipVerify` ⇒ same `Transport`.
  - `TestSharedTransport_DistinctSkipVerify` — different `insecureSkipVerify` ⇒ distinct `Transport`.
  - `TestMakeRestyClientCookieAuth_PerCallClient` — cookie-auth clients are per-call, transport still shared.
- [ ] Optional follow-up: introduce `proxmox.SharedTokenClient(ctx)` accessor + drop `timeout` argument from `MakeRestyClientFromEnv` callsites once a deprecation window passes. Tracked separately.
- [ ] Optional benchmark `BenchmarkSharedClientReuse` (integration-tagged).

### Phase C — Verification (DONE — 2026-05-09)

- [x] `pvmss/proxmox` tests: 29/29 pass under `-race`.
- [x] `make test-offline`: all packages pass except pre-existing `pvmss/env` failures (verified unrelated via `git stash` baseline).
- [x] `make go-lint`: 0 issues.
- [ ] Manual smoke: live Proxmox — observe connection reuse via `ss -tan | grep <pve-ip>`; connections should remain `ESTABLISHED` across consecutive requests.
- [ ] Measure: median latency on `/api/v1/vms` before/after under sustained load (k6 or `hey`). Target ≥30% reduction on warm pool.
- [ ] Update `CLAUDE.md` if/when callsites migrate to a singleton accessor pattern.

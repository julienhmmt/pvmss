# Item 23 findings: consoles and streams outliving the session

Verdict: **confirmed**. The VNC console WebSocket, the serial WebSocket and
the metrics SSE stream check the session once, at open time, and never again.
Logout, password reset, account disable, pool delete and session expiry do
not close an open stream. The stream lives until the browser or Proxmox side
closes it, or the server restarts.

## What exists

- **Session check at upgrade only.** `acceptConsoleWebSocket` calls
  `env.auth.Principal(r)` (`server/internal/httpapi/vm_console_shared.go:157-161`).
  `Principal` -> `sessions.Resolve` loads the row and runs `usable()`, which
  checks `ExpiresAt` and `CreatedAt + MaxSessionAge`
  (`server/internal/auth/session.go:254-256`, `MaxSessionAge` = 24h at
  `session.go:28`). That is the only session check on the WebSocket path.
- **Route not behind auth middleware.** `GET .../console/websocket` is
  registered bare (`server/internal/httpapi/router.go:286`), like the other
  VM routes. The auth check lives inside the handler, not in middleware, so
  there is no per-request re-check either.
- **Relay runs synchronously on the hijacked request.**
  `handleWebSocket` calls `relay.RelayConsole(r.Context(), ...)` and blocks
  (`server/internal/httpapi/vm_console.go:145`). `RelaySerial` is the same
  (`vm_serial_console.go:148`). `r.Context()` cancels only on client
  disconnect or when `ServeHTTP` returns, not on session revocation.
- **Relay loop has no session or time exit.** `proxmoxRelayConsole` and
  `proxmoxRelaySerial` (`server/internal/cluster/websocket_real.go:180-236`,
  `:280-336`) run two `io.Copy` goroutines and return only when one side
  errors. Nothing references the session, a deadline or a max duration.
- **Deadlines cleared on purpose.** `acceptConsoleWebSocket` clears read and
  write deadlines (`vm_console_shared.go:197-198`). The server's
  `ReadTimeout`/`WriteTimeout` (`server/cmd/pvmss/main.go:208-214`) would
  otherwise bound the connection. There is no `MaxDuration` or equivalent.
- **Metrics SSE uses the identity captured once.** `handleStream` resolves
  `identity` and `index` from the request (`vm_metrics.go:105-141`), then
  builds `deps` once (`vm_metrics.go:160`). The loop (`vm_metrics.go:163-177`)
  calls `writeStreamTick` -> `vm.GetMetricsCurrent(ctx, deps)`
  (`vm_metrics.go:180-196`), which resolves against `deps.Actor` and the
  captured `deps.Index` (`server/internal/vm/metrics.go:55-75`). No session
  lookup happens per tick. The running-VM check is also one-shot
  (`vm_metrics.go:131-137`).
- **Ticket is single-use, not session-bound.** `ConsoleTicketStore.Consume`
  (`server/internal/vm/console.go:146-172`) deletes the ticket on use. Its TTL
  (`TicketTTL`) applies only before the WebSocket is opened. Once open, the
  ticket is irrelevant.

## What does not exist

- No periodic session re-check in the relay or the SSE loop.
- No hard cap on connection lifetime (no `MaxSessionAge`-based timer).
- No cancellation hooked to `DeleteAccountSessions`, `DeleteSession`,
  `DeleteOtherSessions`, `DeleteOtherSessionsByProvider` or logout.

## Revocation paths that leave streams open

| Trigger | Where session rows are deleted | Open stream after it |
| --- | --- | --- |
| Logout (own session) | `server/internal/auth/session.go:173-177` (`DeleteSession`), called from `server/internal/httpapi/auth.go:492` | stays open |
| Password change (other sessions) | `session.go:195` (`DeleteOtherSessions`) | stays open |
| Admin password reset of pool user | `server/internal/httpapi/admin_pools.go:375-377` | stays open |
| Pool delete removes its user | `server/internal/httpapi/admin_pools.go:474-477` (`revokePoolUserSessions`) | stays open |
| Local admin account disabled | `server/internal/httpapi/admin_accounts.go:263-265` (`revokeSessions`) | stays open |
| Session expiry or 24h absolute cap | no row deletion; `usable()` only rejects new requests | stays open |

Expiry matters most: `MaxSessionAge` is documented as an absolute lifetime
(`session.go:25-28`), but an open console outlives it indefinitely.

## Risk

The WebSocket is effectively a bearer capability for the whole VM console. A
user who logs out, or whose account is disabled or reset, keeps full console
I/O until they close the tab or the VM stops. The VM-level authorization
(`vm.Resolve`) is also not re-run. The realistic impact is moderate: it
needs a user who was already authenticated and holds a console open.

## Not verified at runtime

No live test was run. The conclusions come from code reading only, which is
enough for the structural claim. A runtime test is the first step of the fix
below.

## Recommended fix (new item, not implemented here)

Smallest version:

1. Add a watcher helper in `server/internal/httpapi/` (for example
   `session_watch.go`) with a signature along the lines of
   `watchSession(ctx, lookup, token []byte, every time.Duration, closeFn func())`.
   It ticks every 15-30s, looks up the session row (`FindSession` + the
   `usable()` logic, reused from `server/internal/auth/session.go:254`), and
   calls `closeFn` when the row is gone or not usable. It also arms a
   `time.NewTimer` for `CreatedAt + MaxSessionAge` as the hard cap.
2. `acceptConsoleWebSocket` (`vm_console_shared.go`) starts the watcher after
   a successful upgrade with `closeFn = conn.CloseNow`. Closing the WebSocket
   makes the `NetConn` reads and writes fail, so the `io.Copy` goroutines
   return and the relay exits. This covers both VNC and serial.
3. `handleStream` (`vm_metrics.go:104`) wraps `r.Context()` with
   `context.WithCancel`, starts the same watcher with `cancel` as `closeFn`,
   and exits its loop on the cancelled context.
4. Store-side check: reuse the row lookup that `auth.Resolve` already uses
   (`server/internal/store/sessions.go`).
5. Docs: `docs/FEATURES.md` and `WORKFLOWS.md` (console workflow) note that
   an open console closes within the watcher interval after logout or revoke.

Test to add first (red, then green):

- In `server/internal/httpapi/`, a table test: start a test server with the
  `fake` cluster source, obtain a console ticket for a session, open the
  WebSocket, call `store.DeleteAccountSessions` (or `Logout` with the session
  cookie), and assert the client read returns a close error within
  `2 * interval`. Make the watcher interval injectable so the test runs in
  under a second.
- Same shape for `/metrics/stream`: revoke the session, assert the SSE body
  reader returns EOF within the interval.
- Unit test for `watchSession`: session row removed, `closeFn` called once;
  timer cap fires with a fake clock; no close while the session is valid.

Run with `make server-test` (race enabled) and `make server-lint`.

## Other observations

- The same gap applies to any future long-lived route. Put a shared helper
  in place rather than per-handler code.
- `docs/`, `WORKFLOWS.md` and `TECH_DEBT.md` were not checked in this pass.

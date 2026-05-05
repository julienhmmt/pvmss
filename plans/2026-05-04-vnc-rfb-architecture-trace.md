# VNC Console Architecture: noVNC RFB ↔ State Manager

> **Origin:** Knowledge graph query on `/graphify .` flagged the connection
> between `RFB` (noVNC protocol layer) and `appState` (State Manager) as a
> graph "surprising connection". This document traces the real data flow.

## Why the graph flagged it

The Louvain community detection placed `RFB` (noVNC, community: frontend UI)
and `appState` / `StateManager` (backend Go, community: auth+config) in distant
clusters. A 3-hop path existed in the AST graph — but it was a **false positive**
caused by a name collision: `.background()` in `rfb.js` (canvas fill color) was
inferred to call `.startNodeCacheWorker()` in `manager_cache.go` because both
are methods with no structural import link.

The real bridge is the **VNC ticket flow** documented below.

---

## Full Data Flow

```
VmConsole.svelte                          Frontend (Svelte 5)
  │
  ├─ onMount() → connect()
  │    ├─ getVNCTicket(vmid)              POST /api/v1/vms/:id/vnc-ticket
  │    │       ↓
  │    │  VNCHandler.GetVNCTicket()       backend/api/v1/vnc.go:40
  │    │    ├─ MakeRestyClientFromEnv()   reads PROXMOX_URL, tokens from env
  │    │    ├─ proxmox.GetVMsResty()      resolves node for vmid
  │    │    └─ proxmox.GetVNCProxyResty() POST proxmox /vncproxy
  │    │         └─ returns VNCProxyResponse{Ticket, Port, Cert, Upid}
  │    │    → responds VNCTicketResponse{ticket, port, node}
  │    │
  │    ├─ dynamic import('/noVNC-1.6.0/core/rfb.js')
  │    │    RFB class from bundled noVNC 1.6.0
  │    │
  │    └─ buildWebSocketURL(vmid, ticket, port, node)
  │         → wss://<origin>/api/v1/vms/:id/console/websocket
  │              ?port=&node=&vncticket=
  │
  └─ new RFB(container, wsUrl, {credentials: {password: ticket}})
       ↓ WebSocket upgrade
       ↓
  VNCHandler.ConsoleWebSocket()           backend/api/v1/vnc.go:99
    ├─ validates port in [5900-5999]
    ├─ h.state.GetEnvConfig()  ←─── STATE MANAGER TOUCH POINT
    │    reads: ProxmoxURL, ProxmoxAPITokenName/Value, ProxmoxSSLVerify
    ├─ buildVNCWebSocketURL()            converts https→wss, sets path
    │    /api2/json/nodes/:node/qemu/:vmid/vncwebsocket?port=&vncticket=
    ├─ proxyVNCWebSocketWithToken()
    │    ├─ gorilla/websocket Upgrader    validates Origin == Host
    │    ├─ dialer.Dial(proxmoxWSURL)     Authorization: PVEAPIToken=name=value
    │    └─ 2× forwardVNCMessages()      bidirectional goroutine pump
    └─ RFB protocol frames flow transparently
```

---

## State Manager Role

`VNCHandler` receives `state.StateManager` via constructor injection
(`MakeVNCHandler`, `vnc.go:27`). It reads from `GetEnvConfig()` **only** during
the WebSocket proxy phase — not during ticket issuance.

Fields consumed:

| Field | Source env var | Used for |
|---|---|---|
| `ProxmoxURL` | `PROXMOX_URL` | Build target WebSocket URL |
| `ProxmoxAPITokenName` | `PROXMOX_API_TOKEN_NAME` | Auth header to Proxmox |
| `ProxmoxAPITokenValue` | `PROXMOX_API_TOKEN_VALUE` | Auth header to Proxmox |
| `ProxmoxSSLVerify` | `PROXMOX_VERIFY_SSL` | TLS skip flag for dialer |

The ticket itself (`vncticket` query param) is forwarded verbatim to Proxmox —
it was issued by Proxmox and is validated by Proxmox, not by PVMSS.

---

## Security Properties

**Origin check** (`vnc.go:171-191`): WebSocket upgrader rejects connections
whose `Origin` header hostname doesn't match `X-Forwarded-Host` or `r.Host`.
This prevents cross-origin WebSocket hijacking from a malicious page.

**VNC ticket as password**: `VmConsole.svelte:94` passes the ticket as the RFB
password (`credentials: { password: ticket }`). noVNC uses VNC authentication —
the ticket is short-lived (Proxmox docs: 2 hours) and single-use from
Proxmox's perspective.

**API token never reaches browser**: Proxmox API credentials travel only in
the server→Proxmox WebSocket handshake. The browser sees only the VNC ticket.

**Port range validation** (`vnc.go:113`): port must be 5900–5999. Prevents
using the WebSocket proxy as an arbitrary TCP relay.

---

## Files

| File | Role |
|---|---|
| `frontend/src/lib/components/vm/VmConsole.svelte` | RFB instantiation, ticket fetch, lifecycle |
| `frontend/src/lib/api/console.ts` | `getVNCTicket()`, `buildWebSocketURL()` |
| `backend/api/v1/vnc.go` | `VNCHandler`, ticket endpoint, WS proxy |
| `backend/proxmox/vnc.go` | `VNCProxyResponse`, `VNCProxyOptions` types |
| `frontend/static/noVNC-1.6.0/` | Bundled noVNC; served at `/noVNC-1.6.0/` |

---

## Potential Improvements

1. **Ticket caching**: Each `connect()` call issues a new Proxmox `vncproxy`
   request. If the user reconnects rapidly (retry loop), this generates
   N tickets. A short client-side debounce (already 3-retry cap) is present
   but no server-side dedup. Low priority — Proxmox handles this gracefully.

2. **WebSocket proxy memory**: `forwardVNCMessages` allocates per-message with
   gorilla's `ReadMessage`. For high-framerate consoles this could be improved
   with a fixed-size read buffer. Not a correctness issue.

3. **VNC ticket passed in query string**: `vncticket` appears in server logs
   and proxy access logs. Consider moving to a custom WebSocket subprotocol
   header or short-lived PVMSS-side token that maps to the Proxmox ticket.

4. **noVNC version**: bundled 1.6.0. Latest upstream is 1.5.0 (stable) /
   master. Periodic update check warranted.

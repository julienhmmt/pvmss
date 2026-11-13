# Single sign-on (OIDC)

PVMSS signs users in through the **OpenID Connect realm of the Proxmox
cluster**. It is not an OIDC client itself: Proxmox talks to the identity
provider (IdP), verifies the ID token and issues the ticket. PVMSS only relays
the authorization code. Identity, ACLs and the admin flag stay in Proxmox.

```
browser -> /api/v1/auth/oidc/start  -> Proxmox /access/openid/auth-url -> IdP
IdP -> /api/v1/auth/oidc/callback   -> Proxmox /access/openid/login   -> PVMSS session
```

## Setup

1. **IdP**: create a confidential web client. Allowed redirect URI:
   `https://<pvmss-host>/api/v1/auth/oidc/callback` (the exact origin users
   type in their browser). Scopes `openid email profile`.
2. **Proxmox**: Datacenter > Permissions > Realms > Add > OpenID Connect.
   Issuer URL = the IdP issuer, client ID/secret from step 1, username claim
   (e.g. `preferred_username`), Autocreate on. The Proxmox nodes must reach
   the issuer URL themselves. Note the realm id (e.g. `zitadel`).
   The username claim is exact (`preferred_username`, underscore) and cannot
   be changed afterwards: delete and re-add the realm to fix it.
3. **PVMSS**: Admin > Clusters > edit the cluster > **SSO realm** = the realm
   id. Empty disables SSO for that cluster. `/login` then shows
   "Continue with SSO" for that cluster.

## Who gets what

- The PVMSS user is the Proxmox user `<claim>@<realm>` (e.g. `alice@zitadel`).
- **Admin**: `Permissions.Modify` on `/`, granted in Proxmox, exactly as for
  password sign-in.
- **Pool**: the user's own ACL with the `PVMSSUser` role on a `/pool/pvmss-*`
  path. Group and inherited grants are ignored on purpose. Several matches
  keep the first by name (logged at `warn`). The service token reads `GET /access/acl`, which needs
  `Permissions.Modify` on those pool paths (it already writes them).
- **First login**: a non-admin with no pool gets one at once, no admin step:
  PVMSS creates the managed pool `pvmss-<username slug>` and grants the user
  `PVMSSUser` on it. A taken name gets a hash suffix instead; PVMSS only
  grants on a pool that very login created, never on an existing one. So every account the IdP lets into this client can
  create VMs within the default quota: restrict who may use the client at
  the IdP. If provisioning fails the user is signed in with no pool (logged
  at `warn`) and the next login retries.
- If the pool lookup itself fails (`GET /access/acl`), the SSO login is
  refused rather than treated as "no pool".
- To revoke a user, disable it in Proxmox or at the IdP. Removing only the
  pool ACL is not a revocation: the next login provisions a new pool.
- Deleting the pool in Admin > Pools leaves the Proxmox user; its next login
  provisions a fresh pool. "Reset password" on an SSO pool would create a
  local `@pve` user for it: do not use it there.
- A user unknown to Proxmox with Autocreate off is refused by Proxmox.
- SSO users have no PVMSS password form; the IdP owns the password.

## Behind a reverse proxy

The callback URL is built from the request: `Host`, and `X-Forwarded-Proto` /
`X-Forwarded-Host` only when `PVMSS_TRUSTED_PROXY_HOPS > 0`. A proxy that
rewrites `Host` must send `X-Forwarded-Host`, or the IdP rejects the redirect.

## Security

- The handshake cookie `pvmss_oidc` (HttpOnly, `SameSite=Lax`, 10 minutes,
  path `/api/v1/auth/oidc`) carries the cluster, Proxmox's `state` and the
  `next` path, signed with `SESSION_SECRET`. It is one-shot.
- `next` accepts only an in-app path (no scheme, `//`, backslash or `/api/`).
- Failed callbacks count toward the login failure limiter; a cancel at the
  IdP does not. Code, state and ticket are never logged.
- Failures land on `/login?oidc=error&reason=<code>`.

## Troubleshooting

`/login?oidc=error&reason=oidc_rejected` means Proxmox refused the login. It
answers a generic 401 on purpose; the reason is only in the node's log:
`journalctl -u pvedaemon | grep -i openid`.

- `Invalid audiences: <id> is not a trusted audience`: the IdP adds an
  audience beside the client ID (Zitadel adds its project ID). Trust it:
  `pveum realm modify <realm> --audiences <id>`.
- A missing-claim error: the username claim is misspelled (see step 2).

## Limits

- Logout is local only: the IdP session stays open.
- No group-to-role mapping: admin status comes from Proxmox ACLs.
- Not yet verified on a multi-node cluster behind a load balancer: Proxmox
  may keep the `state` on the node that issued it. Point the cluster URL at
  one node if the callback fails with `oidc_rejected`.

## Dev test IdP

`ZITADEL_DOMAIN=<LAN IP> make up-oidc` starts Zitadel on
`http://<LAN IP>:8080` (console `/ui/console`, admin
`zitadel-admin@zitadel.localhost` / `Password1!`). Use the nginx dev proxy
(`https://localhost:50001`): the haproxy one rewrites `Host`. With
`PVMSS_CLUSTER_SOURCE=fake` the realm is simulated and needs no IdP.

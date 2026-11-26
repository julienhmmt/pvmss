# Tech Debt

Things that are lingering, half-finished, or consciously accepted - each with
where to look and what deciding it would take. Not a backlog of new features.

Last audited: 2026-11-26. Completed historical entries were removed; git
history keeps the old narrative.

## Open

| Item | Where we are | Close it by |
| ---- | ------------ | ----------- |
| SSO behind a multi-node cluster URL is unverified | `docs/oidc.md` warns that Proxmox may keep the OIDC `state` on the node that issued it; this still needs a real multi-node/LB check | Test against a real multi-node PVE cluster and document the required URL or stickiness |
| SSO first-login pool provisioning is process-local | `server/internal/pools/sso.go` serializes list-create-grant with `ssoMu`; correct for the single-replica deployment | Use an atomic create/upsert or shared locking if PVMSS ever runs several replicas |
| VM name reservations live in process memory | `server/internal/policy/reserve.go` uses a 15-minute in-process map; correct for one replica. A VM deleted or renamed outside the app can hold its name until the TTL | Move coordination to SQLite if replicas > 1; sweep on inventory refresh if outside renames matter |
| Dead guest-agent cloud-init surface remains | `cluster.Writer` still requires `SetCloudInitPassword`, `PingGuestAgent` and `AddSSHKey`; implementations and tests exist, but no production caller remains after the cloud-init and console-password removals | Delete the dead surface, or keep it only when a new VM-access feature wires it |

## Accepted limitations

These are deliberate rulings, documented so they are not reopened accidentally.

- **`Identity.IsAdmin` is a login-time snapshot.** It is read from Proxmox once
  at login and stored in the session. A user whose admin rights are revoked
  keeps them until the session ends - at most `auth.MaxSessionAge` (24h; the
  idle TTL is 8h). Re-checking live would need broader service-token rights on
  a timer, and 8h is short enough.
- **The SSH connection card shows agent-reported info only - no probe.** It
  does not test sshd liveness. A TCP probe would trust guest-reported addresses
  (SSRF risk); a guest-agent probe is intrusive. The card presents the reported
  address and user without claiming liveness.
- **OIDC logout is local only.** The IdP session stays open, as documented in
  `docs/oidc.md`. Proxmox owns the SSO session semantics.
- **Pool password reset recreates the Proxmox user.** PVE refuses
  `PUT /access/password` to API tokens, so `SetUserPassword` deletes and
  recreates the pool user and re-grants the ACL (`cluster/proxmox_pools.go`,
  `pools/reset.go`). The managed pool user carries no settings worth preserving.
- **Template clones keep secondary volumes where the template has them.** A
  template disk such as `efidisk0` on unapproved storage stays there; PVMSS only
  places the boot disk on approved storage. Operators are expected to keep
  templates on approved storage (`vm/create_template.go`).
- **Removed-feature tables stay inert in old databases.** Fresh databases do not
  create `vm_cloudinit_*` or `profile_ssh_keys`; databases that already have
  them keep them inert under the no-drop precedent. Pre-removal cloud-init doc
  pages remain until an admin disables or deletes them.

## Implementation shortcuts

Low-level implementation ceilings are marked in code with `shortcut:` (older
comments may say `ponytail:`) and are not duplicated here unless they cross a
product or deployment boundary. To list the current ledger:

```bash
git grep --no-index --exclude-standard -nE '(#|//|/\*) ?(shortcut|ponytail):'
```

## Related

`ROADMAP.md` · `AGENTS.md` · `CONTEXT.md`

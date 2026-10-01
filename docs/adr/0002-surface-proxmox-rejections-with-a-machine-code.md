# 0002 - Surface Proxmox rejections with a stable machine code

Status: accepted (implemented)

## Context

When Proxmox refuses a request (a locked VM, a storage without snapshot support,
a duplicate snapshot name), the portal used to answer with a generic 500 or a
fixed 502 message. Users could not tell a retryable lock from a permanent
refusal, and the frontend had nothing stable to key its own text on.

## Decision

- `cluster.RejectionError` (wrapped by `cluster.ErrClusterRejected`) carries the
  Proxmox status and message. `httpapi.clusterRejectionResponse` maps it to a
  `(code, message)` pair and handlers answer 502 with both.
- The code is a hint derived loosely from Proxmox's message text:
  `snapshot_storage_unsupported`, `vm_locked`, `snapshot_name_exists`, otherwise
  `cluster_rejected`. The frontend switches on the code and renders its own
  translated text; the raw Proxmox message is the fallback content
  (`web/src/lib/features/chrome/errorMessage.ts`).
- For 401 and 403 from Proxmox the message is suppressed and replaced by a fixed
  one: an authentication error body can name the API token.
- A lock that does not clear within the retry budget is reported under the same
  `vm_locked` code.

## Consequences

- Codes are best-effort: a Proxmox wording change can degrade a code to
  `cluster_rejected`, never to a wrong action, because the message is still shown.
- Adding a new code means one branch in `clusterRejectionCode` and one entry in
  the frontend mapping; unknown codes fall back to the raw message.

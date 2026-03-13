# Specs

This folder contains speckit specifications for the four major projects in this repository.

## Execution Order

The specs are designed to be executed sequentially. Each builds on a cleaner codebase
than the previous, and some share symbols that must be resolved in the right order.

| Order | Folder | Description | Effort |
|-------|--------|-------------|--------|
| **1** | `001-remove-deprecated-functions` | Remove the `NewHandlerContext` deprecated wrapper (25 call sites) | Small — hours |
| **2** | `001-constructor-rename` | Rename all `New...` constructors to `Make...` (45 functions, 81 files) | Medium — 1–2 days |
| **3** | `001-telmate-removal` | Fully remove `github.com/Telmate/proxmox-api-go`, migrate all remaining calls to Resty | Large — several days |
| **4** | `001-frontend-spa-rewrite` | Replace Go templ/HTMX/Vue hybrid with SvelteKit SPA + full REST API | X-Large — weeks |

## Why This Order

1. **Remove deprecated first** — eliminates `NewHandlerContext` before the rename runs, so
   the rename never touches a function that is about to be deleted.

2. **Rename before Telmate removal** — a clean `Make...` naming base makes the Telmate
   migration easier to read and review. The three Telmate-specific constructors renamed in
   step 2 (`MakeClient`, `MakeClientCookieAuth`, `MakeLRUCache`) are then immediately
   deleted in step 3 — that is expected and fine.

3. **Telmate removal before frontend rewrite** — the frontend SPA spec's backend work
   (new `/api/v1/*` endpoints) must be built on the clean, single-client Resty foundation.
   The console API (`vnc_resty.go`) written in step 3 is reused by the frontend console
   feature in step 4.

4. **Frontend rewrite last** — the largest and most independent project. All backend
   cleanup from steps 1–3 must be complete so the new API layer is built on a clean base.

## Cross-Spec Dependencies

| Spec | Depends On | Reason |
|------|-----------|--------|
| `001-constructor-rename` T006 | `001-remove-deprecated-functions` | `NewHandlerContext` excluded from rename — it is deleted, not renamed |
| `001-constructor-rename` T004 | `001-telmate-removal` | `MakeClient`, `MakeClientCookieAuth`, `MakeLRUCache` are short-lived; deleted in Telmate removal |
| `001-telmate-removal` T011 | (creates `backend/proxmox/vnc_resty.go`) | Used by `001-frontend-spa-rewrite` T045 |
| `001-frontend-spa-rewrite` T045 | `001-telmate-removal` T011 | Console handler references `vnc_resty.go` |

## Folder Naming

All four folders are prefixed `001-` reflecting when they were created.
The execution order is defined by this README, not by the folder names.

## Validation Gates (All Specs)

After every spec completes, run:

```bash
make go-fmt
make test-offline
make go-lint
```

The frontend rewrite additionally requires:

```bash
cd frontend-svelte && npm run build
make build
```

# Backend/Frontend dead code cleanup

> **CLOSED 2026-08-13 — superseded by the v0.4 cutover.**
> Batches 1, 2, 4, 5 were applied as planned. Batch 3 was resolved by
> deletion of the entire legacy stack in commit `a7a26f7a`
> ("merge: cutover v0.4 → main (T16) — delete legacy backend/frontend"):
> `backend/` and `frontend/` no longer exist on `v0.4` or `main`, so
> `state/interface.go`, `app/app.go`, `handlers/test_handlers.go` and
> `tests/integration_test.go` are gone with them. No work remains.
> `server/`/`web/` were already clean at audit time.

Source: `/ponytail-audit` repo-wide scan, 2026-08-06. Scope: `backend/` +
`frontend/` (old stack, still production) and `helm/`. `server/`/`web/`
(v0.4 rewrite) came back clean — nothing to cut there.

Each item below was confirmed via grep/deadcode for zero remaining callers
before listing. Re-verify at delete time (code moves fast) — one
`grep -rn` per item before removing.

## Order of operations

Delete leaf/no-dependency items first, run `make test-offline` + `make
go-lint` + `bun run check` after each batch, commit per logical group (not
one giant commit). Stop and re-scope if a "zero callers" claim turns out
stale.

## Batch 1 — backend, isolated dead files (no cross-file risk)

- [x] `backend/utils/generics.go` + its test — Optional/Result/Cache/
      Filter/Map/Reduce toolkit, 874 lines, zero production callers.
- [x] `backend/utils/errors.go` + its test — ErrorWrapper/MakeErrorWrapper/
      WrapSimple/Must, 253 lines, zero external callers.
- [x] `backend/tests/helpers.go` — hand-rolled AssertEqual/RequestBuilder/
      MockHandler/RunTableTests, 280 lines, dupes testify (already a dep).
- [x] `backend/logger/middleware.go` — RequestIDMiddleware/
      CorrelationIDMiddleware/LoggingMiddleware, fully dead; real router
      wiring uses `backend/middleware` instead. ~101 lines.
- [x] `backend/security/validation.go` — empty stub, package decl only.

## Batch 2 — backend, needs grep-before-delete (partial-use files)

- [x] `backend/logger/*` — unused exports Sampler/StackTrace/
      WithRequestID/GenerateRequestID/ErrorWithStack/Sampled/AuthFailure/
      VMFailure/AdminEvent/ConsoleEvent/APIEvent/APIFailure/DatabaseEvent/
      DatabaseFailure (~261 lines). Remove only the unused symbols, keep
      the file if anything else in it is live.
- [x] `backend/errors/errors.go` — unused half: Wrap/VMErr/WrapVM/
      ProxmoxErr/WrapProxmox/AuthErr/WrapAuth/Is/As/IsNotFound/
      IsUnauthorized/IsValidation/IsVMError/IsProxmoxError/IsAuthError
      (~120 lines). `Is`/`As` are pure pass-throughs to stdlib `errors.Is`/
      `errors.As` — replace call sites (if any survive grep) with the
      stdlib call directly, then delete the wrappers.
- [x] `backend/proxmox/vms.go` — dead exports GetVMCloudInitConfigResty,
      SetCookieAuth, GetTimeout, GetBaseURL, GetVMSnapshotResty,
      UpdateVMSnapshotResty, FormatSnaptime, CloseSharedTransports,
      GetAllNetworkInterfacesResty, ClearVMCache, ExtractNetworkBridges,
      GetVMsForNodeResty, GetVMsForNodeRestyFresh,
      ExecuteQemuAgentCommandResty, plus `middleware.Limiter.GetStats`
      (~180 lines total). Re-run `deadcode ./...` to confirm before
      cutting — this file is high-traffic, highest chance of drift.

## Batch 3 — backend, structural (touches call sites)

- [x] (done by stack deletion, `a7a26f7a`) `backend/state/interface.go` — `StateManager` 40-method interface
      (135 lines), one production impl (`appState`), one hand-rolled test
      mock (56 lines). Plan: replace interface type with concrete
      `*state.Manager` pointer at every consumer, delete the interface,
      delete the mock, use the real `appState` (or a lighter fake) in
      tests instead. Bigger diff — do this last, own commit, expect
      touches across `api/v1/` and `handlers/`.
- [x] (done by stack deletion, `a7a26f7a`) `backend/app/app.go` + `backend/handlers/test_handlers.go` +
      `backend/tests/integration_test.go` — fake TestApp/stub handlers
      (269 lines) exercised only by an integration test that asserts
      against hardcoded canned responses and never touches the real
      router/state/proxmox code. Either delete the whole fake harness, or
      (better, if integration coverage matters) rewrite
      `integration_test.go` to hit the real router + `PVMSS_OFFLINE=true`
      instead — flag to user which is wanted before deleting test
      coverage outright.

## Batch 4 — frontend, orphaned components

- [x] `frontend/src/lib/composables/usePolling.svelte.ts` + test — 798
      lines, zero importers.
- [x] `frontend/src/lib/components/vm/VmConsole.svelte` + DataTable/VmCard/
      VmActionButtons/3 skeleton components/TagInput — 526 lines, zero
      importers; console UI now lives in route-local
      `_components/ConsoleBanner.svelte`. (LoadingSkeleton kept — still
      used by 20+ routes.)
- [x] `frontend/src/lib/components/layout/AuthGuard.svelte` +
      AdminGuard/UserMenu/NavLinks/MobileMenu/PageHeader — 486 lines, zero
      importers; `Navbar.svelte`/`Footer.svelte` already cover nav.

## Batch 5 — helm

- [x] `helm/values.yaml` — drop top-level `replicaCount` (shadowed by
      `deployment.replicaCount`, the one actually read) and
      `podAnnotations`/`podLabels`/`podSecurityContext`/`securityContext`
      (no template reads them; security context is hardcoded in
      `deployment.yaml`). Grep every `*.yaml` under `helm/templates/`
      first to confirm before removing — values files are easy to get
      wrong silently.

## Net

~4506 lines removable, 0 deps removable (testify duplication is a
same-dep case, not a new-dep removal).

## Not in scope here

Correctness bugs, security holes, performance — out of scope for this
pass per `/ponytail-audit` boundaries. File separately if found during
cleanup.

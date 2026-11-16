# Spec: admin onboarding checklist

Status: accepted, not implemented. Branch `cc/design-admin-onboarding-6eed18`.

## Problem

After `/setup` (WORKFLOWS.md "First-run setup") the admin lands on `/admin`
with one registered cluster and nothing approved. Users cannot create a VM
until nodes, a storage, a bridge, a template or ISO and a profile are
approved, and nobody can sign in until a PVMSS user (pool) exists or SSO is
configured. Today the admin finds these ~15 pages alone through the sidebar.

## Recommendation

Add a server-computed `onboarding` block to the existing
`GET /api/v1/admin/dashboard` payload, and render it as a card at the top of
`DashboardPage.svelte`, above the alerts. Every step is derived from real
state; there is no tick-box and no new table. The card disappears once every
required step is done.

Why the dashboard payload and not a new endpoint: the handler
(`server/internal/httpapi/admin_dashboard.go`, `ServeDashboard`) already loops
over every cluster with the store at hand, and the page already loads,
refreshes (`refreshIfStale`) and handles errors for it. One extra endpoint
would duplicate all of that for ~6 counts.

Why not `dashboard-alerts.ts`: alerts are "something broke" with severity and
sort order; onboarding is "something was never set up". Mixing them would
push real incidents (cluster unreachable) below "approve a bridge". Keep the
link-building pattern (`withQuery` / `alertHref`) and reuse it.

### `server/internal/checklist/` does not overlap

That package is a dev-only CLI (`pvmss-checklist`) that generates the v0.4
legacy-parity checklist from fiche filenames. It reads no database and has
nothing to do with runtime onboarding. Do not reuse or extend it; name the new
code `onboarding` to avoid the confusion.

## Steps

Evaluated per active cluster (`store.ListClusters`). A step is done for the
dashboard when it is done on **at least one** cluster (decision 1).

| # | Key | Required | Done when | Source | Link |
|---|-----|----------|-----------|--------|------|
| 1 | `nodes` | yes | >= 1 approved node | `catalog.ApprovedResources(...).Nodes` | `/admin/nodes?cluster=` |
| 2 | `storages` | yes | >= 1 approved storage | `.Storages` | `/admin/storages?cluster=` |
| 3 | `bridges` | yes | >= 1 approved bridge | `.Bridges` | `/admin/bridges?cluster=` |
| 4 | `images` | yes | >= 1 template **or** >= 1 ISO | `catalog.Templates` / `.ISOs` | `/admin/templates?cluster=` (ISO link in the hint) |
| 5 | `profiles` | yes | >= 1 profile | `catalog.Profiles` | `/admin/profiles` |
| 6 | `users` | yes | >= 1 managed pool, or the cluster has an OIDC realm | `store.ManagedPools` / `ClusterRow.OIDCRealm` | `/admin/pools?cluster=` |
| 7 | `policy` | no | an `admin.policy.update` audit row exists for the cluster (a `vm_limits` row is seeded at cluster creation, so it cannot tell) | `store.ListAuditLog` | `/admin/policy?cluster=` |

Notes:
- Step 4 merges templates and ISOs: either is enough to create a VM. Two
  separate steps would leave a "not done" item for an admin who deliberately
  uses only templates.
- Step 7 is optional because `policy.policyRowOrDefault` already applies
  `DefaultGabarit()` when no row exists; it is a "review the defaults" nudge,
  shown but not blocking the card's completion.
- Tags, docs, TLS, node capacity, lease length are deliberately left out:
  they have working defaults and would turn the card into the sidebar again.
- If `VMCreationEnabled` is false on every cluster, steps 1-5 are moot; still
  show them (decision 3).

Order is the dependency order (a template lives on an approved node and
storage), so the first undone step is the "next" one and gets the primary
button.

## Payload change (server)

Add to `dashboardDTO`:

```go
Onboarding *onboardingDTO `json:"onboarding,omitempty"` // nil once complete
```

```go
type onboardingStepDTO struct {
    Key        string `json:"key"`        // nodes|storages|bridges|images|profiles|users|policy
    Done       bool   `json:"done"`
    Required   bool   `json:"required"`
    Count      int    `json:"count"`      // e.g. approved nodes, for "2 approved"
    ClusterKey string `json:"clusterKey,omitempty"` // cluster to deep-link when not done
}

type onboardingDTO struct {
    Steps []onboardingStepDTO `json:"steps"`
}
```

- Omit the block (nil) when all required steps are done, so the web side has
  one rule: no `onboarding`, no card. Optional step 7 never keeps it alive.
- `ClusterKey` = first cluster (sorted) where the step is not done, so the
  link opens the list pre-filtered, matching `alertHref`.
- Compute it in a new `onboarding.go` next to `admin_dashboard.go`,
  `func onboardingSummary(ctx, st, log, clusters) *onboardingDTO`. Use the
  store only, not the inventory, so it still renders while a cluster is
  unreachable (in that case `ServeDashboard` currently 503s with
  `inventory_not_ready` when no index exists at all; compute onboarding before
  that check is out of scope, the card just appears on the next load).
- Store errors: log at `Warn` with `component=dashboard` and skip that step
  (mark it done) rather than failing the dashboard, as
  `quotaRequestAlerts` already does.
- Cost: ~5 small SQLite reads per cluster per dashboard load; fine for the
  handful of clusters PVMSS targets. No cache.

## Web

- `dashboard.svelte.ts`: add `onboarding?: { steps: OnboardingStep[] }` to
  `DashboardSummary`.
- New `features/admin-dashboard/onboarding.ts`: `onboardingHref(step)` and
  `onboardingLabel(step)` (Paraglide keys, EN/FR/DE/IT), same shape as
  `alertHref` / `alertMessage`, with a unit test like `dashboard-alerts.test.ts`.
- New `DashboardOnboarding.svelte`: a card titled "Finish setting up PVMSS",
  progress "4 of 6", one row per step (check icon when done, link button when
  not; first undone step is the primary button). Uses `shared/ui` components;
  follows `.devin/rules/ui-quality.md`.
- `DashboardPage.svelte`: render it above `DashboardAttention` when
  `summary.onboarding` is present.
- No dismiss button (decision 2).
- `/setup`: navigate to `/admin` after the cluster is registered (decision 5).

## Tests

- Go: table test on `onboardingSummary` with the fake store: empty catalog
  (all undone, block present), fully seeded demo catalog
  (`testfixture` demo seed, block nil), templates-only (step 4 done), OIDC
  realm without pools (step 6 done), policy row missing (block still nil).
- Web: `onboarding.test.ts` for hrefs/labels; a `DashboardPage` test that the
  card shows with an undone step and is absent without `onboarding`.

## Docs to update when implemented

`WORKFLOWS.md` (First-run setup: what happens after step 5; Dashboard row),
`docs/FEATURES.md` (`/admin`), `messages/*.json`.

## Decisions (were open questions, settled 2026-11-16)

1. **Multi-cluster rule:** a step is done when it is done on at least one
   cluster. A cluster added later is a deliberate admin action, not first run.
2. **Dismiss:** none. The card is purely derived; no settings row.
3. **VM creation off everywhere:** keep showing steps 1-5 unchanged. The
   admin likely turns creation on later; no special state.
4. **Users step and SSO:** a non-empty `OIDCRealm` on the cluster counts as
   done; no sign-in evidence required.
5. **After `/setup`:** the setup flow sets no explicit destination today (no
   `goto` in `features/setup/` or `routes/setup/`). Once the cluster is
   registered, navigate to `/admin` so the card is the first thing seen.
   Part of this ticket.

---
target: vms
total_score: 26
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 2
target_identity: "file:/Users/jh/git/gh/pvmss/web/src/routes/vms/+page.svelte"
target_fingerprint: "sha256:b1100bdb93e28a6a8096f87b2a41b0abcefe1a2f0ac3520710bacc7315679be0"
target_path: /Users/jh/git/gh/pvmss/web/src/routes/vms/+page.svelte
timestamp: 2026-09-29T05-49-21Z
slug: web-src-routes-vms-page-svelte
---

Method: dual-agent (A: 02f3b1ad · B: eb51ea72)

Target: `web/src/routes/vms/+page.svelte` — the machine list (signed-in landing), Operate mode.

## Design Health Score

| # | Heuristic | Score | Key Issue |
| --- | ----------- | ------- | ----------- |
| 1 | Visibility of System Status | 3 | 7-state pill + skeletons + optimistic flip are strong, but the pill is a bare `<span>` with no live region — a row flipping to running after "Démarrer" is never announced. "En cours" reads as *in progress*, not *running*. |
| 2 | Match System / Real World | 3 | Proxmox terms are hidden ("RESSOURCES", "Voir les détails") and copy is warm; but "VM 102" and "Cluster / Demo Cluster Alpha" leak infrastructure onto the beginner path. |
| 3 | User Control and Freedom | 3 | "Effacer les filtres", "Effacer la sélection", pagination present; but bulk power actions have **no undo** and the list has **no sort control** despite the store supporting it. |
| 4 | Consistency and Standards | 2 | The same state is labelled two ways ("En marche" filter vs "En cours" pill; "Arrêtées" vs "Arrêtée"). The machine list is the **only collection in the app without sorting**. |
| 5 | Error Prevention | 2 | "Forcer l'arrêt" / "Éteindre" in the bulk bar fire on one click of **Appliquer** with no confirmation dialog. The single-VM path does confirm. |
| 6 | Recognition Rather Than Recall | 2 | Identity/resources/status/hint are on the row, but the "OS mark" carries **zero OS meaning** (it is the machine *name*'s initials), and on mobile the resource line is deleted. |
| 7 | Flexibility and Efficiency | 2 | `setSort` + `SORTABLE_COLUMNS` exist but are **unreachable from the UI**. "Select all" covers only the current page. No keyboard accelerators on the list. |
| 8 | Aesthetic and Minimalist Design | 3 | Genuinely restrained — orange measured at **0.46%** of full-page pixels, well under the ≤10% rule — but "12 machines" is printed twice and cross-cluster duplicates make the list look padded. |
| 9 | Error Recovery | 3 | Unreachable/error states reassure and offer Retry; failed/partial rows explain themselves ("Rien n'a été décompté de votre quota"); but those rows **cannot be filtered to**. |
| 10 | Help and Documentation | 3 | Quiet help aside, "Ouvrir le guide", teaching empty state, per-state hints — never a banner above the list. Solid. |
| **Total** | | **26/40** | **Acceptable — significant improvements needed** |

## Design Specificity Verdict

**LLM assessment.** The *frame* is product-grounded: the "Calm workspace" concept is real — a quiet card-row collection, warm paper, one accent, and copy that talks like a colleague ("Un endroit pour vos projets. Tout ce qu'il faut, rien de plus."). But the *composition* of the list — monogram tile + name + resources + status pill + one secondary action, inside search/filter/paginate chrome — is a generic admin-collection pattern an unrelated SaaS could lift unchanged. The two elements that claim to be product-specific both under-deliver: the "OS mark" is not an OS mark (it is the first letters of the machine's *name*, with a hash-derived colour), and multi-cluster identity is unresolved (two machines render as "DB / db-01 / VM 102" with no cue beyond a 12px muted subtitle). Specificity here is carried by microcopy and the quota meter, not by the row's interaction or visual language.

**Deterministic scan.** The bundled CLI detector ran clean: **0 findings** across `web/src/routes/vms`, `web/src/lib/features/vms`, and `web/src/lib/shared/ui` (exit 0 on all three). Sanity-checked against planted bad markup (it does fire `low-contrast` / `tiny-text` / `overused-font` when present), so the zero is real. **Important coverage caveat:** the detector's HTML static-analysis rules are extension-gated to `.html`, and this surface is entirely `.svelte` — so contrast and font-size analysis was **not applied** to it by the CLI; only regex rules were. The clean CLI result is therefore weaker evidence than it looks. The in-page detector, injected into a real rendered DOM, did fire (see below).

**Visual overlays.** Script injection **succeeded** — the live server ran on port 8400, `detect.js` loaded (200), `window.impeccableDetect` and the `.impeccable-overlay` DOM were present, and the detector ran against the real page. Five in-page findings on `/vms`: `kicker-above-heading` (eyebrow above the h1), `layout-transition` (`transition: width`), `dark-glow` (zero-offset `#ffba00` glow), and `nested-cards` ×2 (column-header row, quota footer). **This ran in the assessment's headless browser, not yours — there is no user-visible overlay in your browser tab.** Three of the five look like false positives: the eyebrow is a deliberate repo-wide design-system element; the `transition: width` only exists in `Meter.svelte` / `NodeUsageBar.svelte` / admin dashboard files, none rendered on `/vms`; and `#ffba00` appears nowhere in `web/src` — the app's only zero-offset shadows are the accessibility focus rings. The two `nested-cards` hits are arguably true by the detector's heuristic (the machine collection is card-styled) even though the flagged inner nodes are plain `div`s.

## Overall Impression

Not a weak surface — a well-built one with a specific consistency/error-prevention hole. The palette, contrast discipline, and state honesty are genuinely engineered rather than decorated, and the responsive collapse is flawless. What's missing is the *operational* layer: on a screen whose real job is "find the machine that needs you", there is no sort, no "needs attention" filter, and no safety net on the one action with the largest blast radius. The biggest single opportunity is making the destructive bulk path as careful as the single-VM path already is.

## What's Working

1. **Colour and contrast discipline is real, not aspirational.** Measured live: name link 17.3:1, running pill 7.9:1, stopped pill 6.0:1, VMID 4.9:1, quota source 6.5:1, primary button label 4.7:1 — every one clears AA. Orange covers 0.46% of full-page pixels, honouring the One Accent Rule.
2. **State honesty is engineered.** `machine-row.ts` refuses to claim "Connect" without an address (falls back to "Voir les détails"), and `rowHint` gives each state a real sentence — failed → "Rien n'a été décompté de votre quota", partial → "Ne créez pas de doublon". `display-status.ts` derives the 7 states with a documented priority order and never fabricates an address.
3. **The responsive collapse is clean.** At 390px and 320px `scrollWidth === clientWidth` — no horizontal overflow — the toolbar stacks, the column-header row hides, and each row becomes a stacked card.

## Priority Issues

### 1. [P1] Bulk destructive actions have no safety net

- **What:** In select mode, choosing "Forcer l'arrêt" or "Éteindre" and pressing **Appliquer** immediately POSTs to `/api/v1/vms/bulk-action` (`bulk.svelte.ts:138-151`; `VmBulkActionBar.svelte:54-73`). No confirm dialog, no undo, no "these are running VMs" warning. A hard power-off on N machines is one click.
- **Why it matters:** This is exactly the high-stakes moment PRODUCT principle 4 and DESIGN.md's Don't list call out. The single-VM path *does* confirm a graceful shutdown; the bulk path — higher blast radius — is the one without it.
- **Fix:** Require a confirmation step for `stop`/`reset` (and arguably `shutdown`) that names the count and the affected machines, using the existing warning-tone confirm pattern from the detail page. Consider a 5-second undo toast.
- **Suggested command:** `/impeccable harden`

### 2. [P1] You cannot find the machines that need attention

- **What:** The status filter offers only **Toutes / En marche / Arrêtées**, and there is **no sort affordance at all** — the header row is three static `aria-hidden` spans (`MachineList.svelte:246-255`). `VmListStore.setSort` and `SORTABLE_COLUMNS` exist (`list.svelte.ts:64,220`) but nothing in the UI calls them.
- **Why it matters:** `failed`, `provisioning` and `partial` machines carry a server status of `stopped`, so they land under "Arrêtées" indistinguishable from a healthy stopped VM. With 12 machines and no sort, triage is a manual scan — the opposite of scanability for an Operate surface. It is also the only collection screen in the app that cannot be sorted.
- **Fix:** Add a status filter value for the derived states (or at least "Needs attention"), and expose sorting on the column header via the existing `SortButton`/`TableHeader` primitives so the already-URL-backed `sortBy`/`sortDir` become reachable.
- **Suggested command:** `/impeccable shape`

### 3. [P2] Cross-cluster rows are indistinguishable

- **What:** `db-01` renders twice (Alpha `default/102` and Beta `secondary/102`) with identical name, identical **VM 102**, and both "En cours"; only the 12px muted subtitle differs. The link's accessible name is just `db-01` (`MachineList.svelte:293-299`), and `machineInitials`/`machineTone` are name-hashed, so both rows get the same "DB" mark and the same tone.
- **Why it matters:** On a multi-cluster portal this is a correctness-of-identity problem, not a cosmetic one — a user can open, start, or bulk-select the wrong machine and never know. It also breaks `aria-label` disambiguation: two listitems are both named "db-01".
- **Fix:** Promote the cluster into the row identity (a small chip next to the name, or fold it into the link's accessible name: "db-01, Demo Cluster Beta"), and derive the mark's tone from the cluster or offering rather than the name hash.
- **Suggested command:** `/impeccable clarify`

### 4. [P2] The "OS mark" is a name monogram, not an OS anchor

- **What:** `machineInitials(name)` returns the first letters of the *machine name* (`db-01` → "DB") and `machineTone(name)` hashes the name into accent/subtle/success (`machine-row.ts:14-28`). DESIGN.md §8 specifies a tile showing the *offering's* initial with Ubuntu → accent-soft, Debian → subtle, Rocky → success-soft. The source comment admits the DTO has no OS, so the intent was abandoned silently.
- **Why it matters:** The mark is the row's designed recognition anchor and it currently encodes nothing — a user cannot tell Ubuntu from Debian at a glance, and similar names get arbitrary colours. Clearest case of design-doc-vs-implementation drift on this surface.
- **Fix:** Either carry the offering/OS on the list DTO and implement the documented mapping, or drop the tile. Don't ship a "recognition anchor" that is a hash.
- **Suggested command:** `/impeccable shape`

### 5. [P2] Mobile deletes the only technical facts on the row

- **What:** `MachineList.svelte:310-313` marks the resources paragraph `max-[699px]:hidden`. At 390px it is `display:none`, so **vCPU, RAM and VMID vanish**. The row reduces to name / cluster·tag / status / action.
- **Why it matters:** "Is my VM big enough / which one is it" is the core question a developer opens this list to answer. On a phone the list cannot answer it — a task failure, not a density trade-off. DESIGN.md §6.1 lists Resources as a row part with no mobile exception.
- **Fix:** Keep a compact resources line on mobile (e.g. "2 vCPU · 4 GiB" under the name) or move it into the collapsed card body; the VMID can stay hidden.
- **Suggested command:** `/impeccable adapt`

## Persona Red Flags

**Alex (power user / developer who lives in this list)**

- No sort at all — clicking "MACHINE"/"RESSOURCES"/"STATUT" does nothing; the header is `aria-hidden` static text. The URL supports `sortBy`/`sortDir` but there is no way to set them.
- "Select all" is per-page only (`bulk.selectPage(items)`), so a 12-machine bulk action takes two rounds; there is no "select all matching filter".
- Select mode is invisible — "Sélectionner" is a ghost button whose only state cue is `aria-pressed`; nothing tells Alex he is still in select mode after scrolling.
- Two links per row to the same URL (stretched name link + "Voir les détails") make middle-click / copy-link ambiguous.
- No keyboard accelerator on the list, though a global `ShortcutsDialog` exists.

**Sam (accessibility-dependent user)**

- Duplicate accessible names: two listitems both expose `link "db-01"` to different clusters. A screen-reader user cannot tell them apart.
- Status changes are silent: the pill is a plain `<span>` with no `role="status"`/`aria-live`; pressing "Démarrer" flips the row but nothing is announced.
- The column header is `aria-hidden="true"`, including its `sr-only` "ACTIONS" label — hidden from everyone. Screen readers get a `<ul>` with no column context.
- Redundant landmark name: `<section aria-label="Machines virtuelles">` and the inner `<ul aria-label="Machines virtuelles">` share the same name, so AT announces the region twice.
- The stretched-link focus ring spans the whole 1114px row — technically visible, practically unreadable as "which control am I on".

**Jordan (first-timer, doesn't know Proxmox)**

- The "OS mark" lies: Jordan sees "DB" for `db-01` and reasonably assumes an OS; it is just the name.
- The filter and the pill disagree: filter "En marche"/"Arrêtées" vs pill "En cours"/"Arrêtée". Same state, two words, one screen.
- Infrastructure leaks: "Cluster" + "Demo Cluster Alpha / Beta / Offline Demo" and "VM 102" appear on the beginner path, contradicting DESIGN.md's "Don't expose raw Proxmox terminology".
- "Sélectionner" doesn't say what it does — the explanation lives only in a `title` tooltip, invisible on touch.
- Filters are split: the cluster picker is top-right in the header; search and status filter are inside the card.

## Minor Observations

- **FR/EN copy inconsistency (P2-worthy on its own):** `messages/fr.json:1604` has `machine.status.running = "En cours"` while `:1742` has `vms.list.filterRunning = "En marche"`; stopped is "Arrêtée" vs "Arrêtées". English is consistent. Same state, two labels, one view.
- **"12 machines" is printed twice** — toolbar `meta` and the allowance footer.
- **`rounded-2xl` in the empty-state icon tile** is a third radius, against the Two-Radius Rule (illustration chrome, minor).
- **Bulk bar uses `backdrop-blur`** (`VmBulkActionBar.svelte:47`); DESIGN.md bans backdrop-blur on cards. It's a floating bar, so defensible, but it is drift.
- **DESIGN.md §Layout says the sidebar becomes a top bar below 940px; `Sidebar.svelte:11` says a drawer below 900px.** The code wins; the doc is stale.
- **Pagination has no page-size control or jump** — fine at 12 machines, will bite at 100.
- **`.pv-responsive-table` is documented as the one table pattern, but the machine list doesn't use it** — it is a custom grid + `<ul>`, so the claim doesn't cover the landing page.
- **The status banner is an environment artifact:** the SPA fetches `/health` (`chrome/status.svelte.ts:48`) but the Vite dev proxy forwards only `/api`, so the banner "Impossible de joindre le service pour vérifier son état." renders on every signed-in screen in dev. The Go server does serve `GET /health`. Production should not show it, but it is what renders today.

## Questions to Consider

1. The list's job is "find the machine that needs you." Why is there no sort and no "needs attention" filter, when the store already supports both and every admin collection exposes sorting?
2. If the "OS mark" encodes nothing, what is the row's actual recognition anchor? With two `db-01`s sharing a name, a VMID, a monogram and a tone, the only disambiguator is a 12px grey line.
3. Why does the higher-blast-radius path (bulk) have fewer safety nets than the lower one (single-VM shutdown confirm)?
4. What is the landing page for on a phone? Today it shows a name and a state, but not size or ID.
5. The design system promises warmth as the differentiator. On this list, warmth currently lives entirely in strings. Which structural decision here would a competitor have to copy deliberately — and which is just a well-themed generic collection?

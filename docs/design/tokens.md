# Tokens — Layer B (PVMSS App mockup)

Source of truth for the visual language imported by `002-design-import`. This
file records **Layer B only** — the PVMSS mockups — never the Modernist kit
(red `#ec3013`, 0-radius). The running app reads CSS variables from
`web/src/app.css`; this document is the implementer's map from mockup tokens to
those variables.

Reference: the internal `002-design-import` feature spec (not vendored in
this repo — `specs/` is gitignored). The contract and port plan live there;
this document is the implementer-facing extract that stays in the repo.

## 1. Light ramp (Layer B)

| Role | Mockup token | CSS variable (`app.css`) | Light value |
| --- | --- | --- | --- |
| Accent | `--or` | `--primary` | `oklch(66% 0.185 44deg)` |
| Accent hover | `--or-600` | primary hover | `oklch(59% 0.17 44deg)` |
| Accent active | `--or-700` | primary active / nav current text | `oklch(51% 0.15 44deg)` |
| Accent tint | `--or-tint` | `--sidebar-accent` (selected nav fill) | `oklch(96.5% 0.022 44deg)` |
| Accent tint 2 | `--or-tint2` | option-card selected fill | `oklch(93% 0.045 44deg)` |
| Ink | `--ink` | `--foreground` | `#1c1a19` |
| Ink 2 | `--ink-2` | `--muted-foreground` | `#5f5854` |
| Ink 3 | `--ink-3` | `--muted-foreground-subtle` | `#8d8681` (contrast-checked on `--card`, see §5) |
| Ground | `--bg` | `--background` | `#f7f6f4` |
| Surface | `--surf` | `--card` / `--popover` | `#ffffff` |
| Line | `--line` | `--border` | `#e8e4e0` |
| Line 2 | `--line-2` | `--border-subtle` | `#f1eeeb` |
| Radius | `--r` | `--radius` | `0.75rem` (12 px) |
| Radius sm | `--r-sm` | `--radius-sm` | `0.5rem` (8 px) |
| Radius lg | `--r-lg` | `--radius-xl` | `1.125rem` (18 px) |
| Mono | `--mono` | `--font-mono` | `ui-monospace, "SF Mono", "JetBrains Mono", Menlo, monospace` |
| Sans | Archivo | `--font-sans` | `'Archivo Variable', sans-serif` (self-hosted via `@fontsource-variable/archivo`) |

Semantic status (mockup `--ok*` / `--warn*` / `--off*`) maps onto the existing
`success` / `warning` / `muted` soft triples already in `app.css`. The existing
`destructive` and `info` triples are **kept** (the mockup omits them; the app
needs them). No semantic palette is deleted.

## 2. Dark ramp (hand-derived — R1/R3)

The mockup has no dark mode. These values are derived by hand from the light
ramp so the warm-paper character survives at night. Goal: keep the orange
accent perceptually similar, drop ground/surface to warm near-blacks, and keep
the two mute steps distinct.

| Role | CSS variable | Dark value | Derivation note |
| --- | --- | --- | --- |
| Ground | `--background` | `oklch(17% 0.006 49deg)` | warm near-black, same hue as light ink |
| Surface | `--card` / `--popover` | `oklch(21% 0.006 56deg)` | one step lighter than ground |
| Ink | `--foreground` | `oklch(96% 0.004 95deg)` | warm off-white |
| Ink 2 | `--muted-foreground` | `oklch(72% 0.01 56deg)` | one mute step down |
| Ink 3 | `--muted-foreground-subtle` | `oklch(60% 0.012 56deg)` | second mute step (contrast-checked, §5) |
| Accent | `--primary` | `oklch(72% 0.17 44deg)` | lifted L so orange reads on dark ground |
| Accent hover | primary hover | `oklch(78% 0.15 44deg)` | |
| Accent active | primary active | `oklch(82% 0.13 44deg)` | |
| Accent tint | `--sidebar-accent` | `oklch(26% 0.03 44deg)` | low-L orange-tinted fill for selected nav |
| Line | `--border` | `oklch(100% 0 0deg / 10%)` | white hairline on dark |
| Line 2 | `--border-subtle` | `oklch(100% 0 0deg / 6%)` | fainter hairline |
| Secondary / muted / accent fills | `--secondary` / `--muted` / `--accent` | `oklch(25% 0.006 56deg)` | shared dark surface |

## 3. Mockup → `app.css` variable map

| Mockup class | Role | Implementation |
| --- | --- | --- |
| `.crd` | Card | `Card.svelte` primitive (surface, 1 px `--border`, `--radius`, soft shadow) |
| `.pill` + `.p-ok` `.p-off` `.p-w` | Status chip | `Pill.svelte` (ok/off/warn via success/warning/muted soft; text name + dot) |
| `.mt` | Quota/usage meter | `Meter.svelte` (`role="meter"` when bounded; no fake 0–100 for unlimited) |
| `.nvi` `[data-on="1"]` | Sidebar nav item | `Sidebar.svelte` row; active = `--sidebar-accent` fill + `--or-700` text + `aria-current="page"` |
| `.opt` `[data-on="1"]` | Selectable profile card | `OptionCard.svelte` (native radio, visually restyled) |
| `.b` + variants | Button | retune existing `Button.svelte` (≈10 px radius, `font-weight:600`, primary/secondary/ghost/sm; keep destructive) |
| `.inp` `.lbl` `.hint` | Form field | retune existing input styling via tokens (no new class) |
| `.tb` | Table | retune existing table styling via tokens (no vertical rules, `--border-subtle` row tops) |
| `.m` | Mono run | `.mono` utility + `--font-mono` token |

## 4. Chrome geometry (desktop)

- Sidebar column: **236 px**, sticky.
- Header: sticky, translucent.
- Main: **28 px padding** (`p-7`), `max-width: 1180px`, **left-aligned** (not centred).
- Viewport **< 900 px**: sidebar becomes a drawer; the geometry above does not
  apply as min-widths. `ChromeState.sidebarOpen` is forced false when the
  viewport crosses 900 px upward so desktop cannot get stuck "closed".

## 5. Contrast checks

`--muted-foreground-subtle` is the third text step, used for tertiary captions
on `--card`. Checked against WCAG AA (4.5:1 for body text):

- **Light**: `#8d8681` on `#ffffff` → ratio ≈ 3.5:1. This is below AA for body
  text but **above AA for large text / UI components (3:1)**. The token is
  reserved for ≥ 18 px / ≥ 14 px bold captions and non-essential metadata,
  never body copy. Acceptable; no change.
- **Dark**: `oklch(60% 0.012 56deg)` on `oklch(21% 0.006 56deg)` → ratio ≈ 3.6:1.
  Same usage rule as light. Acceptable; no change.

If a future surface needs `--muted-foreground-subtle` for body text, lift it to
`--muted-foreground` (`--ink-2`) instead — do not weaken the step.

## 6. Unported-surface walk (T025)

Surfaces the mockup forgot, confirmed to inherit Layer B tokens by extension
(no per-surface overrides added):

- `/admin/clusters` and the rest of `/admin/*` — inherit via `Card` / table
  retuning and the global sidebar (T034 folds the admin rail into the global
  sidebar).
- Snapshots tab (`VmSnapshotsTab`), cloud-init tab (`CloudInitTab`), disks /
  hardware / network sub-features — inherit token retune; no feature cuts in
  this MVP.
- Docs routes (`/docs`, `/admin/docs`) — inherit.
- Profile / tokens (`/profile`, `/profile/tokens`) — inherit.
- Multi-cluster surfaces — inherit.

No hard-coded hex or `oklch(...)` outside `app.css` was left in `web/src` after
T019, with one documented exception:

- `web/src/lib/features/admin-tags/TagsPage.svelte` keeps `#4f46e5` as the
  default value of a tag entity's color (`<input type="color">` default). This
  is a **data value** persisted via the API, not a UI chrome token — moving it
  to `app.css` would be wrong (it is not a CSS variable). It is the only hex
  literal in a `.svelte` file and is intentional.

## 7. Forbidden

- Modernist red `#ec3013`, 0-radius, Archivo-from-Google-CDN.
- Hard-coded hex in `.svelte` files. Tokens live in `web/src/app.css`.
- Third-party font or DS CDN requests at runtime.
- The lone word "limit" / "limite" in UI copy.

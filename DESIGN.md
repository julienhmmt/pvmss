# Design System: PVMSS

---

name: PVMSS
description: Proxmox VM Self-Service portal — warm, human infrastructure management
colors:
  primary: "#d9742e"
  primary-dark: "#e88a3e"
  background: "#f7f6f4"
  foreground: "#1c1a19"
  card: "#ffffff"
  muted: "#f1eeeb"
  muted-foreground: "#5f5854"
  border: "#e8e4e0"
  destructive: "#c0392b"
  success: "#3ba55c"
  warning: "#d4a017"
  info: "#3b7dd8"
  sidebar: "#ffffff"
  background-dark: "#2a2826"
  card-dark: "#363330"
typography:
  body:
    fontFamily: "'Archivo Variable', sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: "1.25rem"
  title:
    fontFamily: "'Archivo Variable', sans-serif"
    fontSize: "1.5rem"
    fontWeight: 600
    lineHeight: "2rem"
  heading:
    fontFamily: "'Archivo Variable', sans-serif"
    fontSize: "1.875rem"
    fontWeight: 600
    lineHeight: "2.25rem"
  label:
    fontFamily: "'Archivo Variable', sans-serif"
    fontSize: "0.875rem"
    fontWeight: 500
    lineHeight: "1.25rem"
  mono:
    fontFamily: "ui-monospace, 'SF Mono', 'JetBrains Mono', Menlo, monospace"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: "1.25rem"
rounded:
  sm: "0.5rem"
  md: "0.6rem"
  lg: "0.75rem"
  input: "0.625rem"
  xl: "1.125rem"
spacing:
  content-max: "87.5rem"
  navbar-height: "3.5rem"
  sidebar-width: "236px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "#fff8f0"
    rounded: "{rounded.lg}"
    padding: "0.5rem 1rem"
  button-primary-hover:
    backgroundColor: "#c96a2a"
  button-secondary:
    backgroundColor: "{colors.muted}"
    textColor: "{colors.foreground}"
    rounded: "{rounded.lg}"
    padding: "0.5rem 1rem"
  button-ghost:
    backgroundColor: "transparent"
    textColor: "{colors.muted-foreground}"
    rounded: "{rounded.lg}"
    padding: "0.5rem 1rem"
  button-destructive:
    backgroundColor: "{colors.destructive}"
    textColor: "#fff"
    rounded: "{rounded.lg}"
    padding: "0.5rem 1rem"
  input:
    backgroundColor: "{colors.background}"
    textColor: "{colors.foreground}"
    rounded: "{rounded.input}"
    padding: "0.5rem 0.75rem"
  card:
    backgroundColor: "{colors.card}"
    textColor: "{colors.foreground}"
    rounded: "{rounded.lg}"
    padding: "1.5rem"
---

## 1. Overview

Creative North Star: "The Warm Console"

PVMSS is infrastructure management made human. The interface should feel like a competent colleague's desk: organized, warm, slightly informal, never clinical. The warm paper background and orange accent are the signature — they distinguish PVMSS from every cold, gray infrastructure dashboard.

The system is restrained: one accent color (orange), warm neutral surfaces, and semantic colors used only for state (success, warning, destructive, info). Density is welcome when it serves the task — tables, admin forms, policy fields — but decoration is not. Every surface earns its visual weight.

This system explicitly rejects generic SaaS aesthetics: no indigo gradients, no glassmorphism, no hero-metric templates, no identical card grids. It also rejects cluttered enterprise dashboards with excessive panels and gauges. The warmth comes from the palette and microcopy, not from illustrations or animation.

**Key Characteristics:**

- Warm paper background (#f7f6f4) with white card surfaces
- Single orange accent (oklch 66% 0.185 44deg) used sparingly
- OKLCH color space for all semantic colors with soft background variants
- Archivo Variable as the sole typeface (headings, body, labels, data)
- 12px base radius (0.75rem) with 10px input radius (0.625rem)
- Two-layer soft card shadows, not hard drop shadows
- Responsive tables that collapse to stacked cards on mobile
- Reduced-motion support on all animations
- Dark mode with warm near-black ground (not pure black)

## 2. Colors: The Warm Paper Palette

A warm, low-chroma neutral foundation with a single orange accent. Semantic colors carry state, not decoration.

### Primary

- **Blaze Orange** (oklch(66% 0.185 44deg) / #d9742e): Primary actions, active nav state, focus rings, links. Used on <=10% of any screen. Its rarity is the point.
- **Blaze Orange Dark** (oklch(72% 0.17 44deg) / #e88a3e): Dark mode primary, lifted for contrast.

### Neutral

- **Warm Paper** (#f7f6f4): App background. The signature warmth — never use pure white or gray-50 here.
- **Card White** (#ffffff): Card and popover surfaces. Slightly lifted from the paper ground.
- **Warm Muted** (#f1eeeb): Muted backgrounds, secondary surfaces, accent backgrounds.
- **Ink** (#1c1a19): Primary text. Near-black with warm undertone — never use #000.
- **Muted Ink** (#5f5854): Secondary text, labels, hints.
- **Subtle Ink** (#8d8681): Tertiary text, placeholders, metadata.
- **Warm Border** (#e8e4e0): Borders, dividers, input borders. Subtle but visible.

### Semantic

- **Destructive** (oklch(57.7% 0.245 27.325deg)): Delete actions, error states, invalid fields.
- **Success** (oklch(64% 0.17 145deg)): Positive confirmations, running status, completed tasks.
- **Warning** (oklch(75% 0.16 75deg)): Caution states, TLS insecure, capacity warnings.
- **Info** (oklch(62% 0.17 245deg)): Informational toasts, info banners.

Each semantic color has a soft variant (background), soft-foreground, and soft-border for use in banners, badges, and soft-state surfaces.

### Dark Mode

Warm near-black ground (oklch(17% 0.006 49deg)), lifted card surfaces (oklch(21% 0.006 56deg)), and lifted orange accent. Never use pure black (#000) or cool gray for dark mode backgrounds.

### Named Rules

**The One Accent Rule.** Blaze Orange is used on <=10% of any screen. It appears on primary buttons, active nav, focus rings, and links — never as decoration, background fill, or large area color.

**The Warm Neutral Rule.** All neutrals are warm-tinted (hue 49-56deg in OKLCH). Never use pure gray, cool gray, or #000/#fff for surfaces or text.

## 3. Typography

**Display Font:** Archivo Variable (sans-serif)
**Body Font:** Archivo Variable (sans-serif)
**Mono Font:** ui-monospace, SF Mono, JetBrains Mono, Menlo

**Character:** One family for everything. Archivo Variable is a workhorse sans with enough personality to avoid feeling generic. No display/body pairing — product UI doesn't need it. The mono font is reserved for technical values (VMIDs, node names, sizes, UPIDs) and uses tabular numbers.

### Hierarchy

- **Heading** (600, 1.875rem / 2.25rem line-height): Page titles, admin section headers.
- **Title** (600, 1.5rem / 2rem line-height): Card titles, dialog titles, section headers.
- **Body** (400, 0.875rem / 1.25rem line-height): Default text, table cells, form values.
- **Label** (500, 0.875rem / 1.25rem line-height): Form labels, nav items, button text.
- **Mono** (400, 0.875rem, tabular numbers): Technical values, code, JSON previews.

### Rules

**The One Family Rule.** Archivo Variable carries headings, body, labels, and data. No display fonts in UI labels, buttons, or data. The only exception is the mono font for technical values.

**The Tabular Numbers Rule.** Use the `.mono` class (or `font-mono` token) for VMIDs, node names, disk sizes, memory values, and UPIDs. It enables `font-feature-settings: "tnum" 1` for aligned numeric columns.

## 4. Elevation

The system uses a hybrid approach: tonal layering for most surfaces, soft shadows for cards and dialogs.

### Shadow Vocabulary

Three steps, no more. Each is a token (`--elev-rest`, `--elev-raised`,
`--elev-overlay`) defined once per theme, so light and dark cannot drift:

- **`.shadow-card`** (`--elev-rest`): the resting step. Cards, tiles, panels.
- **`.shadow-raised`** (`--elev-raised`): hover on an interactive card, and the
  bulk-action bar. Reserved for "this element is lifting toward you".
- **`.shadow-overlay`** (`--elev-overlay`): dialogs, popovers, toasts —
  anything floating over the page.

Dark mode redefines the same three tokens with higher opacity to compensate for
the dark ground.

### Named rules

**The Flat-By-Default Rule.** Surfaces are flat at rest. Shadows appear only on cards, dialogs, and elevated elements. Never add shadows to buttons, inputs, or nav items.

## 5. Components

### Buttons

- **Shape:** `--radius-control` (0.625rem) — the same radius as inputs, so a
  button next to a field reads as one control set. inline-flex, items-center,
  gap-2, fixed heights (sm 2rem / md 2.5rem / lg 2.75rem, plus square `icon`
  and `icon-sm` sizes) so a row of mixed controls aligns without hand-tuning.
- **Primary:** Blaze Orange background, near-white foreground. Hover: slightly darker orange. Focus: 2px ring offset by 2px background. Loading: spinner icon + disabled state.
- **Secondary:** Card background with a 1px border. Hover: border darkens, muted fill.
- **Outline:** Transparent with a 1px border — the quieter bordered form on tinted grounds.
- **Ghost:** Transparent background, muted-foreground text. Hover: muted background.
- **Subtle:** Filled neutral, no border — for dense rows where a border grid would be noisy.
- **Destructive:** Destructive background, white foreground. Used for delete and revoke actions.
- **Press:** every variant translates down 1px on `:active`. Depth comes from
  that, never from a resting shadow (see the flat-by-default rule).

Never hand-roll a button. `Button.svelte` and `ButtonLink.svelte` are the same
component in two semantics — which one a call site needs is a semantics
decision (does it navigate?), never a visual one.

### Inputs / Fields

- **Style:** 1px solid warm border, warm paper background, 0.625rem radius, 0.5rem 0.75rem padding. Applied via `.pv-input` class.
- **Focus:** Border shifts to ring color, 2px box-shadow ring offset by 2px background. Matches Button focus ring.
- **Invalid:** Border shifts to destructive, focus ring matches.
- **Disabled:** 50% opacity, not-allowed cursor.
- **Select:** Native select with `.pv-select` (appearance: none, custom chevron icon).

### Cards / Containers

- **Corner Style:** Radius lg (0.75rem)
- **Background:** Card White (#ffffff) in light, oklch(21% 0.006 56deg) in dark
- **Shadow Strategy:** `.shadow-card` — two-layer soft, warm-tinted
- **Border:** 1px solid warm border
- **Internal Padding:** `p-6` (1.5rem) default, `p-4` (1rem) compact

### Navigation

- **Sidebar:** 236px fixed width, white background, sticky. Nav items: rounded-lg, px-3 py-1.5, text-sm font-medium. Active: sidebar-accent background + aria-current. Admin groups: collapsible with chevron rotation.
- **Header:** Sticky, 3.5rem height, translucent. Contains: menu button (mobile), docs link, activity button with badge, language switcher, theme toggle.

### Tables

Two classes, applied together: `.pv-table` owns the look, `.pv-responsive-table`
owns the mobile collapse. Cells carry no spacing utilities of their own — that
is what let admin tables drift away from the VM list.

- **Header:** sticky band on `--muted`, 11px uppercase with 0.04em tracking, a
  hairline under it.
- **Rows:** 0.75rem/1rem cells, a `--border-subtle` rule between them, a muted
  hover fill plus a 2px accent rail inset on the first cell.
- **Figures:** `.num` on a `<th>`/`<td>` gives tabular mono digits, right
  aligned, so VMIDs, core counts and sizes line up down the column. Headers
  stay in the sans face; only cells go mono.
- **Mobile:** collapses to stacked cards with label/value pairs driven by the
  `data-label` attribute on each `<td>`; the desktop cell metrics are handed
  back to the card layout below 640px.
- **Sort indicators:** `SortButton.svelte` — an arrow whose space is reserved
  permanently, so the column never reflows when the direction changes. Inactive
  columns reveal a faint arrow on hover.

### Forms

- **`FormField`** owns the label, hint, error and the aria wiring. Mark the
  rarer side of requiredness: `required` prints the asterisk, `optional` prints
  a muted tag. Using both in one form is the mistake this makes visible.
- **`FormSection`** groups fields under a real `<fieldset>`/`<legend>`, with an
  optional step number for wizards and a `panel` variant for advanced or
  secondary settings. Any form past about six controls needs chapters.
- **`Toolbar`** is the filter row above a list: search (capped at ~22rem),
  filters, then `meta` and `actions` pushed right.

### Dialogs

- **Container:** `Dialog.svelte` — backdrop blur, centered card, focus trap, escape to close, focus restoration.
- **Max width:** `max-w-lg` default, `max-w-2xl` for wide forms.
- **Animation:** 160ms ease-out fade-in.

### Toasts

- **Position:** Fixed bottom-right (desktop), fixed bottom-full-width (mobile).
- **Variants:** Success (success-soft), Error (destructive-soft), Info (info-soft).
- **Auto-dismiss:** 5 seconds, manual dismiss button.
- **ARIA:** `role="alert"` for errors, `role="status"` for success/info.

### Skeletons

- **Style:** `.skeleton` — muted background, pulse animation, reduced-motion safe.
- **TableSkeleton:** Configurable rows/columns, matches real table structure.

### Empty States

- **Style:** `EmptyState.svelte` — icon in a tinted disc, title, description, optional action snippet. `tone="error"` swaps the disc to the destructive triple for unreachable-cluster states.
- **Character:** Teaching, not scolding. "Create your first VM" not "No VMs found."

## 6. Do's and Don'ts

### Do

- **Do** use the warm paper background (#f7f6f4) for the app background and white (#ffffff) for card surfaces.
- **Do** use Blaze Orange sparingly — primary buttons, active nav, focus rings, links only.
- **Do** use OKLCH for all semantic colors with soft variants for backgrounds.
- **Do** use Archivo Variable for all text. Use the mono font only for technical values.
- **Do** use `.pv-input` and `.pv-select` classes on all form controls for consistent styling.
- **Do** use `.shadow-card` / `.shadow-raised` / `.shadow-overlay` for elevation, and `.pv-table .pv-responsive-table` for data tables.
- **Do** use `Button` / `ButtonLink`, `TextField`, `Select`, `FormField`, `FormSection`, `Toolbar`, `Pill`, `StatCard` and `EmptyState` instead of re-styling their markup by hand.
- **Do** use `Dialog.svelte` for all modal dialogs — it has focus trap, escape handling, and focus restoration.
- **Do** use skeleton loading states, not spinners in the middle of content.
- **Do** use teaching empty states with actionable next steps.
- **Do** respect `prefers-reduced-motion` on all animations.

### Don't

- **Don't** use pure black (#000) or pure white (#fff) for surfaces or text. Use warm-tinted neutrals.
- **Don't** use indigo, violet, or blue gradients. This is not a generic SaaS template.
- **Don't** use glassmorphism, backdrop-blur on cards, or decorative shadows.
- **Don't** use border-left or border-right greater than 1px as a colored accent stripe.
- **Don't** use gradient text (background-clip: text with a gradient).
- **Don't** use display fonts in UI labels, buttons, or data.
- **Don't** use spinners inside content areas. Use skeleton states instead.
- **Don't** use hand-rolled modal divs (`fixed inset-0 z-50`) instead of `Dialog.svelte`.
- **Don't** use identical card grids with icon + heading + text repeated endlessly.
- **Don't** add decorative motion that doesn't convey state. Motion is for feedback, not choreography.

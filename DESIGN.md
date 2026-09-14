# Design System: PVMSS

---

name: PVMSS
description: Proxmox VM Self-Service portal - warm, human infrastructure management
colors:
primary: "#ba5100"
primary-solid: "#ba5100"
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
backgroundColor: "{colors.primary-solid}"
textColor: "#fff8f0"
rounded: "{rounded.lg}"
padding: "0.5rem 1rem"
button-primary-hover:
backgroundColor: "#a24200"
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

PVMSS is infrastructure management made human. The interface should feel like a
competent colleague's desk: organized, warm, slightly informal, never clinical.
The warm paper background and orange accent are the signature - they distinguish
PVMSS from every cold, gray infrastructure dashboard.

The system is restrained: one accent color (orange), warm neutral surfaces, and
semantic colors used only for state (success, warning, destructive, info).
Density is welcome when it serves the task - tables, admin forms, policy fields
 - but decoration is not. Every surface earns its visual weight.

This system explicitly rejects generic SaaS aesthetics: no indigo gradients, no
glassmorphism, no hero-metric templates, no identical card grids. It also rejects
cluttered enterprise dashboards with excessive panels and gauges. The warmth
comes from the palette and microcopy, not from illustrations or animation.

**Application direction (chosen concept: "Calm workspace"):** the signed-in app
is a single, quiet workspace - a fixed sidebar, a slim context header, and one
roomy content column. The machine list is the landing page, not a metrics
dashboard. Creation is a single-page form with a live summary rail. The machine
detail page is connection-first: state, SSH, and the browser console lead;
configuration and activity follow. The user never has to understand Proxmox to
get a usable machine.

**Key Characteristics:**

- Warm paper background (#f7f6f4) with white card surfaces
- Single orange accent (oklch 56% 0.16 51deg) used sparingly
- OKLCH color space for all semantic colors with soft background variants
- Archivo Variable as the sole typeface (headings, body, labels, data)
- 12px base radius (0.75rem) with 10px input radius (0.625rem)
- Two-layer soft card shadows, not hard drop shadows
- Responsive tables that collapse to stacked cards on mobile
- Reduced-motion support on all animations
- Dark mode with warm near-black ground (not pure black)

## 2. Colors: The Warm Paper Palette

A warm, low-chroma neutral foundation with a single orange accent. Semantic
colors carry state, not decoration.

### Primary

- **Blaze Orange** (oklch(56% 0.16 51deg) / #ba5100): the accent. Active nav
  state, focus rings, links, tints (`bg-primary/10`), status dots, meters and
  chart strokes. Used on <=10% of any screen. Its rarity is the point. Deepened
  from the original 66% lightness so accent text clears AA on the paper ground
  (4.56:1); at 66% it sat at 3.09:1.
- **Blaze Orange Solid** (oklch(56% 0.16 51deg) / #ba5100): the fill under a
  label - primary buttons, active tabs, step badges. Deliberately *not* the same
  token as the accent, because the two roles want opposite things in dark mode:
  a fill needs to be dark enough for a white label (>=4.5:1), an accent needs to
  be light enough to read against a near-black ground. One token cannot be both,
  so this one is fixed across themes and the accent moves instead.
- **Blaze Orange Dark** (oklch(72% 0.17 44deg) / #e88a3e): the dark mode
  *accent* only - lifted for contrast against the dark ground (7.21:1). The
  solid fill stays #ba5100 in dark mode, so a primary button is the same button
  in both themes, still reading as a control at 3.59:1 against a dark card.

### Neutral

- **Warm Paper** (#f7f6f4): App background. The signature warmth - never use pure
  white or gray-50 here.
- **Card White** (#ffffff): Card and popover surfaces. Slightly lifted from the
  paper ground.
- **Warm Muted** (#f1eeeb): Muted backgrounds, secondary surfaces, accent
  backgrounds.
- **Ink** (#1c1a19): Primary text. Near-black with warm undertone - never use
  #000.
- **Muted Ink** (#5f5854): Secondary text, labels, hints.
- **Subtle Ink** (#8d8681): Tertiary text, placeholders, metadata.
- **Warm Border** (#e8e4e0): Borders, dividers, input borders. Subtle but
  visible.

### Semantic

- **Destructive** (oklch(57.7% 0.245 27.325deg)): Delete actions, error states,
  invalid fields.
- **Success** (oklch(64% 0.17 145deg)): Positive confirmations, running status,
  completed tasks.
- **Warning** (oklch(75% 0.16 75deg)): Caution states, TLS insecure, capacity
  warnings.
- **Info** (oklch(62% 0.17 245deg)): Informational toasts, info banners.

Each semantic color has a soft variant (background), soft-foreground, and
soft-border for use in banners, badges, and soft-state surfaces.

### Dark Mode

Warm near-black ground (oklch(17% 0.006 49deg)), lifted card surfaces (oklch(21%
0.006 56deg)), and a lifted orange *accent*. Never use pure black (#000) or cool
gray for dark mode backgrounds.

Only the accent is lifted. The solid fill (Blaze Orange Solid) is the same
#ba5100 as light mode, so primary buttons keep their warm-white label rather
than flipping to dark ink - the label colour is a property of the button, not
of the theme.

### Named Rules

**The One Accent Rule.** Blaze Orange is used on <=10% of any screen. It
appears on primary buttons, active nav, focus rings, and links - never as
decoration, background fill, or large area color.

**The Warm Neutral Rule.** All neutrals are warm-tinted (hue 49-56deg in OKLCH).
Never use pure gray, cool gray, or #000/#fff for surfaces or text.

## 3. Typography

**Display Font:** Archivo Variable (sans-serif)
**Body Font:** Archivo Variable (sans-serif)
**Mono Font:** ui-monospace, SF Mono, JetBrains Mono, Menlo

**Character:** One family for everything. Archivo Variable is a workhorse sans
with enough personality to avoid feeling generic. No display/body pairing - 
product UI doesn't need it. The mono font is reserved for technical values
(VMIDs, node names, sizes, UPIDs) and uses tabular numbers.

### Hierarchy

- **Heading** (600, 1.875rem / 2.25rem line-height): Page titles, admin section
  headers.
- **Panel Title** (600, 1.25rem / 1.75rem line-height, tight tracking): Bare
  (non-`Card`) admin panel headers - one `<h2>` per panel, stacked directly on
  the page (AuditRetentionPanel, AuditLogPanel, ExportPanel, ImportPanel). De
  facto standard already; named here so it stops reading as an off-scale
  one-off.
- **Title** (600, 1.5rem / 2rem line-height): Card titles, dialog titles,
  section headers.
- **Dialog Title** (600, 1.125rem / 1.75rem line-height): The heading inside a
  `Dialog` - smaller than `Title` because a dialog's own chrome (backdrop,
  close affordance, max-width) already sets it apart from the page; used as-is
  across ~20 dialogs (DeleteVmDialog, AddDiskDialog, ClusterFormDialog,
  ShortcutsDialog, ConfirmDialog, and others).
- **Body** (400, 0.875rem / 1.25rem line-height): Default text, table cells,
  form values.
- **Label** (500, 0.875rem / 1.25rem line-height): Form labels, nav items,
  button text.
- **Mono** (400, 0.875rem, tabular numbers): Technical values, code, JSON
  previews.

`Card.svelte`'s own `title` prop renders smaller still (`text-sm font-semibold`)
 - a banded header sitting flush against the card's border, not a peer of the
tiers above. No panel in the app has adopted it yet; don't reach for it as a
substitute for **Panel Title** on a component that isn't already a `Card`.

The anonymous home page's hero (`text-4xl`) and its two marketing sections'
centered headers (`text-xl`, HomeCapabilities/HomeHowItWorks) sit outside this
scale on purpose - a landing pitch, shown only to logged-out visitors, is
allowed a heavier register than the app's own internal chrome.

### Rules

**The One Family Rule.** Archivo Variable carries headings, body, labels, and
data. No display fonts in UI labels, buttons, or data. The only exception is
the mono font for technical values.

**The Tabular Numbers Rule.** Use the `.mono` class (or `font-mono` token) for
VMIDs, node names, disk sizes, memory values, and UPIDs. It enables
`font-feature-settings: "tnum" 1` for aligned numeric columns.

## 4. Elevation

The system uses a hybrid approach: tonal layering for most surfaces, soft
shadows for cards and dialogs.

### Shadow Vocabulary

Three steps, no more. Each is a token (`--elev-rest`, `--elev-raised`,
`--elev-overlay`) defined once per theme, so light and dark cannot drift:

- **`.shadow-card`** (`--elev-rest`): the resting step. Cards, tiles, panels.
- **`.shadow-raised`** (`--elev-raised`): hover on an interactive card, and the
  bulk-action bar. Reserved for "this element is lifting toward you".
- **`.shadow-overlay`** (`--elev-overlay`): dialogs, popovers, toasts - anything
  floating over the page.

Dark mode redefines the same three tokens with higher opacity to compensate for
the dark ground.

### Named rules

**The Flat-By-Default Rule.** Surfaces are flat at rest. Shadows appear only on
cards, dialogs, and elevated elements. Never add shadows to buttons, inputs, or
nav items.

## 5. Application Shell - "Calm Workspace"

The signed-in application is a **single quiet workspace**, not a dashboard. The
chosen concept ("Calm workspace") fixes three structural decisions:

1. **Sidebar-first navigation.** A fixed left rail carries the brand, primary
   navigation, a reassurance note, preferences, and the account link. It never
   collapses into a hamburger on desktop; it reflows to a top bar only on
   narrow viewports.
2. **The machine list is the landing page.** No metrics dashboard, no hero
   tiles. A signed-in user lands on their machines and acts from there.
3. **One roomy content column.** A slim context header names the current
   location; below it, the content column holds one task at a time.

### Layout grid

```text
┌─────────┬──────────────────────────────────────────┐
│         │ context header  (slim, breadcrumb-style) │
│ sidebar ├──────────────────────────────────────────┤
│  236px  │                                          │
│ fixed   │ main content  (max 1290px, centered)      │
│         │                                          │
│         ├──────────────────────────────────────────┤
│         │ page footer  (tagline + context)          │
└─────────┴──────────────────────────────────────────┘
```

- **Sidebar:** 236px fixed (248px above 1500px, 204px below 1180px). White
  surface, sticky, full viewport height. Right border separates it from the
  page.
- **Context header:** 66px tall, translucent over the page. Shows
  `Workspace / <current screen>` on the left and a private-workspace marker on
  the right. Hidden below 700px.
- **Main content:** `max-width: 1290px`, centered, `padding: 44px 44px 20px`.
  Each screen owns its own internal layout; the shell does not impose a grid.
- **Page footer:** thin, two-line - tagline left, context label right.

### Sidebar anatomy (top to bottom)

1. **Brand** - the PVMSS mark (orange rounded square with the "P" glyph) and
   wordmark, with a one-line caption ("Your own space to build.").
2. **Workspace label** - a faint, tracked, uppercase label ("PERSONAL
   WORKSPACE") that names the section below it.
3. **Primary navigation** - three items, each a rounded link with an icon and a
   count chip:
   - **My machines** (icon: machines) - count of non-failed machines. Active
     when the current screen is `machines`, `create`, or `detail`.
   - **Activity** (icon: clock) - count of in-flight operations (provisioning,
     starting, stopping) when > 0, rendered in the accent color.
   - **Help & guides** (icon: help).
4. **Sidebar bottom** - pinned to the bottom:
   - A short reassurance note ("Approved by your team. Ready for your next
     idea.") with a shield icon.
   - Preferences row: theme toggle (sun/moon icon button) and language button
     (EN/FR with a chevron).
   - Account link: avatar initials, name, "Personal account" subtitle, and a
     trailing chevron.

### Responsive collapse

- **Below 940px:** the sidebar becomes a horizontal top bar - brand left,
  navigation centered, preferences + account right. The workspace label and
  reassurance note hide.
- **Below 700px:** the bar wraps to two rows - brand + preferences on row one,
  full-width navigation on row two. The context header hides. Content padding
  shrinks to `28px 20px`.
- **Below 370px:** nav count chips and OS marks hide to preserve room.

### Navigation semantics

- The active nav item uses `aria-current="page"` and the accent-soft background
  with accent-text color.
- `create` and `detail` are not separate nav items - they are states of "My
  machines" and highlight that item.
- Navigation is client-side; the page heading receives focus on screen change
  (`#page-heading`, `tabindex="-1"`) and the scroll position resets to top.

### Production mapping

| Prototype element     | Production component                         |
| --------------------- | -------------------------------------------- |
| Sidebar shell         | `web/src/lib/features/chrome/Sidebar.svelte` |
| Brand mark + wordmark | `web/src/lib/shared/ui/Logo.svelte`          |
| Primary nav items     | `sidebar-navigation.svelte.ts`               |
| Theme toggle          | `chrome/ThemeToggle.svelte`                  |
| Language switcher     | `chrome/LanguageSwitcher.svelte`             |
| Context header        | `chrome/AppHeader.svelte` (slimmed)          |
| Account link          | new - fold into `Sidebar.svelte` bottom slot |

## 6. Screen Catalog

Six screens compose the end-user workspace. Each has a fixed heading pattern:
an **eyebrow** (tracked uppercase, muted), an **`<h1 id="page-heading">`**
(focusable), and an optional **page description** (muted, max ~65ch). The
primary action sits at the top-right of the heading row.

### 6.1 My machines (`machines`)

The signed-in landing page. A readable list, not a metrics grid.

**Heading:** eyebrow "YOUR WORKSPACE", title "My machines", description "A place
for your projects. Everything you need, nothing you don't." Primary action:
"Create a machine" (orange, plus icon).

**Body, in priority order:**

1. **Quota notice** (only when allowance is full) - a warning notice: "Your
   machine allowance is full" with guidance to ask an administrator. Never
   blocks use of existing machines.
2. **State surface** - exactly one of:
   - **Unavailable** - an error-toned empty state: "We can't reach your
     workspace." Reassures that machines are not deleted; offers "Try again".
   - **Loading** - three skeleton rows (square + name + line + action) with a
     "Finish simulated loading" affordance in the prototype; in production, a
     `TableSkeleton`.
   - **Empty (first visit)** - a teaching empty state: illustration (machines
     icon in a rounded tile with a "+" badge), eyebrow "ROOM FOR YOUR NEXT
     IDEA", title "Your first machine starts here.", body explaining approved
     configurations, primary action "Create your first machine", and a muted
     reassurance line ("Guided choices · Approved configurations · Your own
     space").
   - **List** - the machine collection (below).
3. **Machine collection** - a single bordered container (`machine-collection`)
   with three parts:
   - **Toolbar** - search field (icon + input, capped ~300px), status filter
     select (All / Running / Stopped), and a right-aligned count
     ("`n` machines").
   - **Column labels** - a grid header row (MACHINE / RESOURCES / STATUS / ·),
     hidden below 700px.
   - **Rows** - each row is a 4-column grid:
     - **Identity** - OS mark (rounded tile with the offering's initial, tone
       -tinted), machine name (link to detail), and OS + size subtitle.
     - **Resources** - vCPU · GB RAM on one line, GB storage on the next.
       Figures are tabular-num.
     - **Status** - a status pill (dot + label). Tones: running → success-soft,
       provisioning/starting/stopping/partial → warning-soft, failed →
       error-soft, stopped → muted.
     - **Actions** - stopped machines show "Start" (secondary); others show
       "Connect" (running) or "View details" (other states), both secondary.
   - **Row hint** - a single muted line below the row when the state needs
     explanation (provisioning, failed, partial, no-address, stopped). Failed
     and partial hints use the error color.
   - **No results** - when filters match nothing: "No matching machines" with a
     "Clear filters" text button.
   - **Allowance row** - a footer band: "`n / 5` machines used", a 5-segment
     meter (`role="meter"`, filled segments in accent), and "Set by your
     administrator".
4. **Quiet help** - below the collection, a low-key help prompt: book icon,
   "A little help, if you need it.", one line of body, and a "Open the guide"
   text button. Never a banner; never above the list.

### 6.2 Create a machine (`create`)

A **single-page form** with a **live summary rail**. No multi-step wizard in the
chosen concept - all sections are visible at once, the summary updates live, and
the submit button lives in the summary.

**Heading:** back link "My machines", eyebrow "A NEW PLACE TO BUILD", title
"Create a machine", description "A few choices. Your own environment." A
"Team-approved choices" caption with a shield icon sits at the right.

**Layout:** a two-column grid - `creation-form` (fluid) + `creation-summary`
(272px, sticky). Below 700px the summary stacks under the form and becomes
static.

**Form sections** (each a `<fieldset>` with a numbered legend, a description,
and the controls):

1. **01 - Choose your starting point** - offering options as full-width radio
   cards. Each card: OS mark, title + edition, an optional "Good first choice"
   badge (Ubuntu only), a one-line description, and a radio indicator that fills
   with a check when selected. A field note below: "Includes SSH and
   browser-console support. Network and storage are preconfigured."
2. **02 - Give it room to work** - size options as a 3-column grid of radio
   cards (Small / Standard / Large). Each card: name, radio indicator, a
   purpose line, a spec line ("`n` vCPU / `n` GB RAM"), and a storage line. A
   field note: "Not sure? Small is a good place to experiment."
3. **03 - Make it yours** - name + access:
   - **Machine name** - text input, hostname pattern, maxlength 63, with a
     hint and an inline error (shown only after submit). Errors: invalid
     pattern, duplicate name.
   - **Access grid** (2 columns) - login username (text, pattern-validated) and
     SSH public key (select). Each has its own hint.
   - **Access note** - a top-bordered note: "Use the computer that holds your
     SSH private key."
4. **Draft note** - below the last section: "Your choices stay while you
   browse." In production, this is the draft-auto-save affordance.

**Summary rail** (sticky, `creation-summary`):

- Title row: machines icon + "Your new machine".
- **Name** - the entered name, or "Waiting for a name" (muted).
- **OS** - offering name + edition.
- **Details** (`<dl>`) - Size, Processor, Memory, Storage, Login (mono), SSH
  key.
- **Included** - two check rows: "Approved network & storage", "Starts after
  setup".
- **Quota** - "After creation: `n+1 / 5` machines".
- **Submit** - "Create this machine" (orange, full-width, plus icon), disabled
  when blocked. A 10px disclaimer below: "Simulated creation only." (In
  production, replaced by the real provisioning note.)

**Blocked states** (replace the form with an empty state):

- **Quota reached** - "You've reached your machine allowance."
- **No catalog** - "Your catalog isn't ready yet." Explains the admin must
  approve an offering first.
- **Loading** - "Your catalog is still loading."
- **Unavailable** - "We can't load the available choices." Offers "Try again".

**Validation behavior:**

- Native constraint validation (`required`, `pattern`, `maxlength`).
- Duplicate-name check against existing non-failed machines.
- Errors appear only after a submit attempt; the offending field receives
  focus.
- Entered values are preserved across recoverable errors (draft retention).

### 6.3 Machine detail (`detail`)

**Connection-first.** The page answers "is it ready, and how do I connect?"
before anything else.

**Heading:** back link "My machines". A detail identity row: large OS mark,
`<h1>` machine name, and a description line (OS · size · `VM {id}` in mono). At
the right: a status pill and the primary power action - "Shut down" (secondary,
running) or "Start machine" (primary, stopped).

**State banners (above the tabs, in priority order):**

- **Shutdown confirmation** - a warning notice that appears inline when the
  user clicks "Shut down": "Shut down this machine?" with a graceful-shutdown
  explanation, "Keep running" (secondary) and "Confirm shutdown"
  (warning-toned). Never a forced power-off.
- **Provisioning** - a provisioning panel: eyebrow "WE'RE ON IT", title
  "Making room for your next project.", a reassurance line, a 4-step progress
  list (Request accepted → Preparing the disk → Configuring access → Starting
  the machine) with complete/current/pending states, and a "Back to my
  workspace" link.
- **Failed / partial** - an error notice: explains what happened (storage
  unavailable for failed; access config failed for partial), explicitly says
  "Do not create a duplicate" for partial, offers "Review the request" (failed)
  or "Get help from your administrator" (partial), and a collapsible
  "Technical details for your administrator" with a code line.
- **Busy (starting/stopping)** - a neutral notice: "Starting your machine.
  Connection options will appear once it is running." / "Waiting for a graceful
  shutdown. Your files will be kept."

**Tabs** (underline-style, accent on active):

1. **Connect** (default) - a two-column layout: SSH section (fluid) + browser
   console aside (260px).
   - **SSH section** - eyebrow "FROM YOUR COMPUTER", title "Connect with SSH",
     one line of body, then the command in a bordered bar: `$ ssh user@address`
     with a copy icon button. A demo-address note (prototype) / a real-address
     note (production). A `<dl>` of connection facts (Username, IP address,
     Network). A connection-help note about the private key and team network.
   - **Address unavailable** - when running but no address: "The address isn't
     available yet" with guidance to use the console or ask an admin. "We won't
     guess an address." When not running: "SSH isn't available right now."
   - **Console aside** - "Or stay in your browser.", one line of body, "Open
     browser console" (secondary, full-width, disabled unless running), and a
     hint.
   - **Resource strip** - below the layout: vCPU / memory / storage, each with
     an icon, a bold value, and a one-line caption ("Allocated, not live
     usage").
2. **Configuration** - a read-only `<dl>`: OS, approved size, disk, network &
   placement ("Managed by your team"), machine identifier (mono). Hardware
   editing and snapshots are designed later.
3. **Activity** - a per-machine timeline: rows with a marker, a bold message, a
   "Simulated operation" subtitle, and a time. Empty state: "No operations in
   this demo session yet."

**Browser console dialog** (`<dialog>`):

- Header: eyebrow "SIMULATED SESSION", title "`{name} / console`", close button.
- Terminal surface: dark, mono, a `<pre>` transcript (aria-live polite) and a
  prompt input (`user@name:~$`) with a "Send" button.
- Footer: "This is a design demonstration, not a real terminal. Try 'help'."

### 6.4 Activity (`activity`)

**Heading:** eyebrow "NOTHING LOST IN THE BACKGROUND", title "Activity",
description "Know what's happening, even after you leave a page." A "This demo
session" label at the right.

**Body:**

- **In progress** (only when operations are running) - a section listing
  in-flight machines as rows (marker + name + status label + arrow), each
  linking to its detail page.
- **Recent updates** - a timeline of activity rows (marker + bold machine name +
  message + time), each linking to the relevant detail. Empty state: "All quiet
  for now." with a "Go to my machines" link.

### 6.5 Help & guides (`help`)

**Heading:** eyebrow "A LITTLE GUIDANCE", title "You don't need to know
Proxmox.", description "Start here. The infrastructure details can wait."

**Layout:** a two-column grid - articles (fluid) + aside (260px).

- **Articles** - a stack of `<details>` accordions, each with a numbered
  summary ("01 - Create your first machine", "02 - Connect with SSH or your
  browser", …). A `+`/`−` indicator sits at the right. The first article is
  open by default. Each article body is plain prose with inline text-button
  links to the relevant screen.
- **Aside** - a muted card: "New to virtual machines?" with a short
  explanation and a bulleted list of concepts.

### 6.6 Account (`account`)

**Heading:** eyebrow "YOUR PREFERENCES", title "Your account", description "A
workspace that feels comfortable to use."

**Body** (a single `account-panel`, max 780px):

- **Identity** - large avatar (initials), name, "Personal account" subtitle.
- **Preference rows** (top-bordered, space-between):
  - **Appearance** - "Warm light or a quieter dark workspace." with a
    "Switch to light/dark" secondary button.
  - **Language** - "The complete interface is available in English and French."
    with a language select.
- **Field note** - "Authentication, passwords and API tokens are intentionally
  outside this prototype." (In production, this is where token management
  lives - see `profile/tokens`.)

## 7. State and Feedback Model

The design distinguishes seven operational states for a machine, and the UI
never treats a successful API response as "ready to connect" unless connection
data actually exists.

### Machine states

| State        | Pill tone    | Meaning                                            |
| ------------ | ------------ | -------------------------------------------------- |
| running      | success-soft | Up; address known. SSH + console available.        |
| stopped      | muted        | Down; files kept. Start available.                 |
| provisioning | warning-soft | Request accepted; setup in progress. Leave freely. |
| starting     | warning-soft | Power-on in progress. Connect info pending.        |
| stopping     | warning-soft | Graceful shutdown in progress. Files will be kept. |
| failed       | error-soft   | No VM allocated. Allowance not consumed. Review.   |
| partial      | warning-soft | Created, but access config failed. Stopped.        |

### Safety rules

- **No guessed addresses.** A running machine without a reported address shows
  "The address isn't available yet" and points to the console. The UI never
  fabricates an address.
- **No duplicate creation on uncertain outcomes.** A `partial` machine
  explicitly says "Do not create a duplicate" and routes to help.
- **No readiness claim without connection data.** The "Connect" action on the
  list and the SSH section on detail only render when `status === 'running' &&
address` is truthy.
- **Graceful shutdown, not force-off.** The shutdown action opens an inline
  confirmation that names the behavior ("graceful shutdown, not a forced
  power-off").
- **Draft retention.** Entered form values survive recoverable errors and
  variant switches; reloading clears the draft (production uses real
  draft-auto-save).

### Feedback surfaces

- **Toasts** - fixed bottom-right (desktop) / bottom-full-width (mobile).
  Variants: success, error, info. Auto-dismiss 5s, manual dismiss. ARIA:
  `role="alert"` for errors, `role="status"` for success/info.
- **Notices** - inline banners within the content flow. Three tones: warning
  (warning-soft), error (error-soft), neutral (subtle). Each has an icon, a
  bold title, a body, and optional actions. Never modal.
- **Empty states** - teaching, not scolding. "Create your first machine" not
  "No VMs found." An illustration, an eyebrow, a title, a body, a primary
  action, and an optional reassurance line.
- **Skeletons** - `.skeleton` with a breathing pulse (reduced-motion safe).
  Used for loading inside content, never spinners.

## 8. Components

### Buttons

- **Shape:** `--radius-control` (0.625rem) - the same radius as inputs, so a
  button next to a field reads as one control set. inline-flex, items-center,
  gap-2, fixed heights (sm 2rem / md 2.5rem / lg 2.75rem, plus square `icon`
  and `icon-sm` sizes) so a row of mixed controls aligns without hand-tuning.
- **Primary:** Blaze Orange Solid background, near-white foreground, in both
  themes. Hover: softened fill. Focus: 2px ring offset by 2px background.
  Loading: spinner icon + disabled state.
- **Secondary:** Card background with a 1px border. Hover: border darkens,
  muted fill.
- **Outline:** Transparent with a 1px border - the quieter bordered form on
  tinted grounds.
- **Ghost:** Transparent background, muted-foreground text. Hover: muted
  background.
- **Subtle:** Filled neutral, no border - for dense rows where a border grid
  would be noisy.
- **Destructive:** Destructive background, white foreground. Used for delete and
  revoke actions.
- **Text button:** Accent-colored, underline on hover, used for low-emphasis
  in-content links ("Open the guide", "Clear filters").
- **Press:** every variant translates down 1px on `:active`. Depth comes from
  that, never from a resting shadow (see the flat-by-default rule).

Never hand-roll a button. `Button.svelte` and `ButtonLink.svelte` are the same
component in two semantics - which one a call site needs is a semantics decision
(does it navigate?), never a visual one.

### Inputs / Fields

- **Style:** 1px solid warm border, warm paper background, 0.625rem radius,
  0.5rem 0.75rem padding. Applied via `.pv-input` class.
- **Focus:** Border shifts to ring color, 2px box-shadow ring offset by 2px
  background. Matches Button focus ring.
- **Invalid:** Border shifts to destructive, focus ring matches.
- **Disabled:** 50% opacity, not-allowed cursor.
- **Select:** Native select with `.pv-select` (appearance: none, custom chevron
  icon).

### Radio cards (offering / size selectors)

The creation form uses **radio cards** - full-width `<label>` wrappers around a
visually-hidden radio input. This is the pattern for any "choose one of N
prepared options" control.

- **Rest:** 1px warm border, card surface, 8px radius.
- **Selected:** accent border, accent-soft background, radio indicator fills
  with accent and a check icon.
- **Focus:** `:focus-within` draws a 3px focus ring offset 3px (so keyboard
  focus on the hidden radio is visible on the card).
- **Hover (not selected):** border darkens to faint.

### Cards / Containers

- **Corner Style:** Radius lg (0.75rem)
- **Background:** Card White (#ffffff) in light, oklch(21% 0.006 56deg) in dark
- **Shadow Strategy:** `.shadow-card` - two-layer soft, warm-tinted
- **Border:** 1px solid warm border
- **Internal Padding:** `p-6` (1.5rem) default, `p-4` (1rem) compact

### Navigation

- **Sidebar:** 236px fixed width, white background, sticky. Nav items:
  rounded-lg, px-3 py-1.5, text-sm font-medium. Active: sidebar-accent
  background + aria-current. Admin groups: collapsible with chevron rotation.
- **Header:** Sticky, 3.5rem height, translucent. Contains: menu button
  (mobile), docs link, activity button with badge, language switcher, theme
  toggle.

### Tables

Two classes, applied together: `.pv-table` owns the look, `.pv-responsive-table`
owns the mobile collapse. Cells carry no spacing utilities of their own - that
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
- **Sort indicators:** `SortButton.svelte` - an arrow whose space is reserved
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

- **Container:** `Dialog.svelte` - backdrop blur, centered card, focus trap,
  escape to close, focus restoration.
- **Max width:** `max-w-lg` default, `max-w-2xl` for wide forms.
- **Animation:** 160ms ease-out fade-in.
- **Vertical rhythm:** `Dialog.svelte` owns no spacing below its own title - 
  each caller hand-rolls the gap after its `<h2>`, and three different values
  (`mb-2`, `mb-3`, `mb-4`) had accumulated for the same "title → body" gap
  across ~20 dialogs with no reason to differ. **Title → body is `mb-4`/`mt-4`
  (1rem)**, the same step as the gap between two form fields - a dialog's title
  and its content are read as one continuous block, not two separate sections.
  Where a dialog interposes a supporting hint line between the title and its
  form (`TemplateEditForm`, `NodeCapacityForm`), the hint itself sits at `mt-2`
  (0.5rem, title → hint is the tighter "same idea" gap) and the form below it
  opens a real section at `mt-6` (1.5rem).

### Toasts

- **Position:** Fixed bottom-right (desktop), fixed bottom-full-width (mobile).
- **Variants:** Success (success-soft), Error (destructive-soft), Info
  (info-soft).
- **Auto-dismiss:** 5 seconds, manual dismiss button.
- **ARIA:** `role="alert"` for errors, `role="status"` for success/info.

### Skeletons

- **Style:** `.skeleton` - muted background, pulse animation, reduced-motion
  safe.
- **TableSkeleton:** Configurable rows/columns, matches real table structure.

### Empty States

- **Style:** `EmptyState.svelte` - icon in a tinted disc, title, description,
  optional action snippet. `tone="error"` swaps the disc to the destructive
  triple for unreachable-cluster states.
- **Character:** Teaching, not scolding. "Create your first VM" not "No VMs
  found."

### Status pills

A dot + label pill used on the machine list and detail. Tones map to the
semantic soft variants (running → success-soft, provisioning/starting/stopping/
partial → warning-soft, failed → error-soft, stopped → muted). The dot is
`currentColor`, 5px, so the pill reads as one colored unit.

### OS marks

A rounded tile (38×42px on the list, 53×58px on detail) showing the offering's
initial. Tone-tinted: Ubuntu → accent-soft, Debian → subtle, Rocky →
success-soft. The tile is an identity anchor, not a logo - it lets a user
recognize their machine at a glance without a real OS logo.

### Allowance meter

A 5-segment bar (`role="meter"`, `aria-valuenow/min/max`) with filled segments
in the accent color. Used in the machine-list footer and the creation summary.
The label always names the source ("Set by your administrator").

## 9. Accessibility

Target: WCAG 2.1 AA.

- **Skip link** - "Skip to content" appears on focus, jumps to `#main-content`.
- **Focus management** - the page heading receives focus on screen change;
  scroll resets to top. `:focus-visible` draws a 3px accent ring offset 4px.
- **Semantics** - `<nav>`, `<main>`, `<header>`, `<footer>`, `<dialog>`,
  `<fieldset>`/`<legend>`, `<dl>`, `role="meter"`, `aria-current="page"`,
  `aria-pressed` for tabs, `aria-live="polite"` for the console transcript and
  provisioning panel, `role="alert"`/`role="status"` for notices and toasts.
- **Keyboard** - every action is reachable by keyboard; the console dialog
  traps focus and restores it on close; escape closes dialogs.
- **Reduced motion** - `prefers-reduced-motion: reduce` disables all
  animations and transitions.
- **Language** - `<html lang>` is set to the active locale (en/fr).
- **Contrast** - body text ≥ 4.5:1, large text ≥ 3:1, focus rings ≥ 3:1.

## 10. Internationalization

- **Locales:** English (default) and French. Both are complete for every
  screen, state, and microcopy string.
- **Mechanism:** production uses Paraglide (`web/messages/` +
  `web/project.inlang/`, with generated output in `web/src/lib/paraglide/`).
  The prototype used inline `[en, fr]` tuples for throwaway convenience;
  production must not.
- **Layout:** French strings are ~20–30% longer than English. Layouts must not
  hard-code widths that break under French text; the summary `<dl>` and the
  status pills use `overflow-wrap: anywhere` and `white-space: nowrap`
  respectively to absorb this.
- **Numbers** - disk/memory sizes use the mono face with tabular numbers. The
  unit (`GB`/`Go`) is translated.

## 11. Do's and Don'ts

### Do

- **Do** use the warm paper background (#f7f6f4) for the app background and
  white (#ffffff) for card surfaces.
- **Do** use Blaze Orange sparingly - primary buttons, active nav, focus rings,
  links only.
- **Do** use OKLCH for all semantic colors with soft variants for backgrounds.
- **Do** use Archivo Variable for all text. Use the mono font only for
  technical values.
- **Do** use `.pv-input` and `.pv-select` classes on all form controls for
  consistent styling.
- **Do** use `.shadow-card` / `.shadow-raised` / `.shadow-overlay` for
  elevation, and `.pv-table .pv-responsive-table` for data tables.
- **Do** use `Button` / `ButtonLink`, `TextField`, `Select`, `FormField`,
  `FormSection`, `Toolbar`, `Pill`, `StatCard` and `EmptyState` instead of
  re-styling their markup by hand.
- **Do** use `Dialog.svelte` for all modal dialogs - it has focus trap, escape
  handling, and focus restoration.
- **Do** use skeleton loading states, not spinners in the middle of content.
- **Do** use teaching empty states with actionable next steps.
- **Do** respect `prefers-reduced-motion` on all animations.
- **Do** make the machine list the signed-in landing page, not a dashboard.
- **Do** put connection instructions (SSH + console) first on the detail page.
- **Do** distinguish running, provisioning, failed, partial, and
  no-address states explicitly.
- **Do** preserve entered form values across recoverable errors.

### Don't

- **Don't** use pure black (#000) or pure white (#fff) for surfaces or text.
  Use warm-tinted neutrals.
- **Don't** use indigo, violet, or blue gradients. This is not a generic SaaS
  template.
- **Don't** use glassmorphism, backdrop-blur on cards, or decorative shadows.
- **Don't** use border-left or border-right greater than 1px as a colored
  accent stripe.
- **Don't** use gradient text (background-clip: text with a gradient).
- **Don't** use display fonts in UI labels, buttons, or data.
- **Don't** use spinners inside content areas. Use skeleton states instead.
- **Don't** use hand-rolled modal divs (`fixed inset-0 z-50`) instead of
  `Dialog.svelte`.
- **Don't** use identical card grids with icon + heading + text repeated
  endlessly.
- **Don't** add decorative motion that doesn't convey state. Motion is for
  feedback, not choreography.
- **Don't** expose raw Proxmox terminology (node, storage, bridge, ISO) in the
  beginner path unless necessary. Use "starting point", "size", "access".
- **Don't** claim a machine is "ready" unless connection data actually exists.
- **Don't** guess an address. Show "address not available yet" and point to the
  console.
- **Don't** let a user create a duplicate of a `partial` machine. Surface the
  existing one and route to help.

## 12. Production Migration Notes

The chosen concept is a **direction**, not a copy. The prototype
(`/Users/jh/git/gh/pvmss-design-prototypes`) is throwaway validation code. When
migrating into `web/`:

- **Reuse production components** - `Button`, `ButtonLink`, `TextField`,
  `Select`, `FormField`, `FormSection`, `Toolbar`, `Pill`, `Dialog`,
  `EmptyState`, `TableSkeleton`, `Sidebar`, `ThemeToggle`, `LanguageSwitcher`.
  Do not re-implement them from the prototype's hand-rolled classes.
- **Reuse production state** - the existing `vm-create/draft.svelte.ts`
  (draft auto-save), `vms/list.svelte.ts`, `vms/detail.svelte.ts`, and
  `tasks/` stores. The prototype's `PrototypeState` is a mock; the real state
  model already exists.
- **Use Paraglide** (`web/messages/` + `web/project.inlang/`, output in
  `web/src/lib/paraglide/`) for all strings, not inline tuples.
- **Use the existing token system** in `web/src/app.css` - the prototype's
  `--page`/`--surface`/`--accent` variables are the same warm identity expressed
  in OKLCH; production already has them as `--background`/`--card`/`--primary`.
- **Map the screens to routes:**
  - `machines` → `/vms` (becomes the signed-in landing)
  - `create` → `/vms/create`
  - `detail` → `/vms/[cluster]/[id]`
  - `activity` → `/tasks` (or a new `/activity` route)
  - `help` → `/docs` (existing in-app docs)
  - `account` → `/profile`
- **Add production tests** - Vitest for stores, Playwright for the creation
  flow and state scenarios, accessibility checks, responsive checks, and
  bilingual checks as the design is migrated.
- **Update `WORKFLOWS.md`** for any approved user-facing workflow changes
  (creation model, connection-first detail, activity surface).

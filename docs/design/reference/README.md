# Design reference — PVMSS App (Layer B)

This directory is the offline anchor for the `002-design-import` feature. The
**Layer B** mockups (the PVMSS screens, not the Modernist kit) are the visual
source of truth for every token and primitive in this feature.

## Source

The mockups live in a Claude design project (read-only, network-only):

- Design project: `https://claude.ai/design/p/8ddb2888-53da-4813-8809-e56b8beba8cc`
- Two Layer-B HTML exports: `PVMSS App.dc.html` (all signed-in screens) and
  `Login.dc.html` (the sign-in card).

These `.dc.html` files are **not vendored here**. They are external to the repo
and were not committed with the spec. If you need to view source offline, fetch
them from the design project above and drop them into this directory; do not
edit them — they are reference, not source code.

## Screen inventory (offline, local-only)

`web/prototype/PVMSS App.pdf` is a **screen inventory only** — a flattened PDF
of the mockup screens. Use it to compare layout and rhythm; it does not carry
the token `:root` or component class definitions. The authoritative token
values are in `docs/design/tokens.md`.

Note: `web/prototype/` is gitignored, so the PDF is **not in the repo** — it
lives only on developer machines that fetched it from the design project. A
fresh clone will not have it. The authoritative token values remain
`docs/design/tokens.md`, which is committed.

The legacy per-screen HTML in `web/prototype/` (`index.html`, `mes-vms.html`,
`creer-vm.html`, `fiche-vm.html`) predates this feature and is **not** the
Layer-B reference — it is the v0.3-era static prototype. Ignore it for token
work.

## What to port, what to ignore

- **Port**: Layer B (the PVMSS mockups). Tokens in `visual-language.md §1`,
  component classes in `design-import-svelte-port.md §1`.
- **Ignore**: Layer A — the `_ds/modernist-…/` "Modernist" kit (red `#ec3013`,
  0-radius, brutalist). Take Archivo from it; leave the rest.
- **Forbidden at runtime**: any third-party font or DS CDN request (R2). Fonts
  are self-hosted via `@fontsource-variable/archivo`.

# Frontend Rewrite Plan

## Design

See [2026-03-06-frontend-rewrite-design.md](./2026-03-06-frontend-rewrite-design.md) for the full approved design.

## Stack

- **SvelteKit** (SPA mode, adapter-static)
- **TypeScript**
- **Vite**
- **shadcn-svelte** (Mira preset)
- **Tailwind CSS v4**
- **Geist font** (sans + mono)
- **Phosphor icons**
- **@novnc/novnc** (npm, for VM console)

## Theme Config

```bash
preset: mira
baseColor: stone
theme: orange
iconLibrary: phosphor
font: geist
menuAccent: subtle
menuColor: default
radius: small
```

Preview: [https://ui.shadcn.com/create?base=base&preset=a1DMDThI](https://ui.shadcn.com/create?base=base&preset=a1DMDThI)

preset : `--preset a1DMDThI`.

## Quick Start

```bash
# Scaffold
npm create svelte@latest frontend -- --template skeleton --types typescript
cd frontend
npx shadcn-svelte@latest init --preset a1DMDThI
npx shadcn-svelte@latest add button card dialog table form input select tabs sidebar badge dropdown-menu sheet sonner

# Dev
npm run dev                      # Vite dev server
```

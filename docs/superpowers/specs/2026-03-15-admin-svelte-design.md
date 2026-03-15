# Design : Modernisation Frontend — Pages Admin (Phase 1)

**Date** : 2026-03-15
**Statut** : Approuvé
**Périmètre** : Pages d'administration uniquement (phase 1 du rewrite frontend complet)
**Approche** : SvelteKit SPA complet, livraison admin en premier (Approche B)

---

## Contexte et objectifs

Le frontend actuel est un hybride Go templ + Alpine.js + HTMX + Vue 3 avec Bulma CSS. L'objectif est de le remplacer par un SPA moderne, fluide et modulaire, en commençant par les pages d'administration.

**Objectifs de cette phase :**
- Créer une bibliothèque de composants réutilisables (admin d'abord, pages utilisateur plus tard)
- Rendre l'application plus fluide, réactive et esthétique
- Exposer une API JSON complète pour toutes les opérations admin
- Maintenir exactement le même niveau de fonctionnalité que le frontend templ actuel

---

## Stack technique

| Élément | Choix | Version |
|---------|-------|---------|
| Framework | SvelteKit (adapter-static, SPA mode) | 2.x |
| Build tool | Vite | 5.x |
| Langage | Svelte 5 + TypeScript | 5.x |
| Design system | shadcn-svelte (Mira preset) | latest |
| CSS | Tailwind CSS | v4 |
| Font | Geist (sans + mono) | latest |
| Icônes | Phosphor | latest |
| Thème base | Stone / accent Orange | — |
| Radius | Small | — |

### Preset shadcn-svelte
```
preset: mira / --preset a1DMDThI
baseColor: stone
theme: orange
iconLibrary: phosphor
font: geist
menuAccent: subtle
menuColor: default
radius: small
```

---

## Architecture générale

### Coexistence pendant la phase admin

```
Go :50000
  /api/v1/admin/*    → nouveaux JSON handlers admin (JWT isAdmin)
  /api/v1/*          → handlers JSON existants (inchangés)
  /admin/*           → frontend-svelte/build/ (SPA catch-all → index.html)
  /css/*, /js/*...   → assets legacy (inchangés)
  /* autres          → handlers templ existants (inchangés)
```

Les pages utilisateur continuent de fonctionner via templ pendant toute cette phase. L'extension aux pages utilisateur se fera dans une phase ultérieure en élargissant le catch-all Go.

### Auth SPA admin (sans refactoring JWT)

1. L'utilisateur se connecte via la page login templ existante (`/login`)
2. Au chargement de `/admin`, le SPA appelle `/api/v1/auth/exchange` (endpoint existant)
3. L'exchange retourne un JWT à partir du cookie de session actif
4. Le JWT est conservé en mémoire Svelte (jamais en localStorage)
5. Si l'exchange échoue (session invalide) → redirect vers `/login`
6. Tous les appels `/api/v1/admin/*` portent le JWT en header `Authorization: Bearer`

---

## Bibliothèque de composants

### Principe

- Les composants ne contiennent **aucune logique de fetching** — les pages gèrent le data fetching
- Props **TypeScript strictement typées** via interfaces
- Events Svelte 5 (`$props`, `$bindable`, snippets) — pas d'ancienne syntaxe
- Deux niveaux : primitives UI (shadcn) et composants métier réutilisables

### Niveau 1 — Primitives UI (shadcn-svelte CLI)

```
src/lib/components/ui/
  button, card, dialog, table, form, input, select,
  badge, sidebar, sheet, sonner, skeleton, separator,
  tooltip, dropdown-menu, tabs, switch
```

### Niveau 2 — Composants métier

```
src/lib/components/
  layout/
    AppShell.svelte         ← navbar + sidebar + slot contenu principal
    AdminSidebar.svelte     ← navigation admin, active state automatique
    PageHeader.svelte       ← titre + icône Phosphor + bouton action optionnel
    ThemeToggle.svelte      ← bascule dark/light mode
  data/
    DataTable.svelte        ← table générique triable/filtrable (TypeScript generics)
    ResourceCard.svelte     ← carte stat (titre, valeur, icône, sous-texte)
    StatusBadge.svelte      ← badge coloré selon statut (running/stopped/error/ok)
    EmptyState.svelte       ← état vide avec icône + message + CTA optionnel
    LoadingSkeleton.svelte  ← squelette de chargement
  forms/
    ConfirmDialog.svelte    ← dialog de confirmation pour actions destructives
    InlineEdit.svelte       ← édition inline click-to-edit
    TagInput.svelte         ← input multi-tags avec chips
  feedback/
    ErrorBanner.svelte      ← erreur API avec message + bouton retry
```

### Exemple de contrat TypeScript

```typescript
// DataTable — générique, réutilisable partout
interface Column<T> {
  key: keyof T
  label: string
  sortable?: boolean
  render?: (value: T[keyof T], row: T) => string
}

interface Props<T> {
  data: T[]
  columns: Column<T>[]
  loading?: boolean
  emptyMessage?: string
  onRowClick?: (row: T) => void
}
```

---

## Pages admin

### Layout partagé

`AppShell` + `AdminSidebar` + `PageHeader`. Navigation latérale avec icônes Phosphor, active state via `$page.url.pathname`. Dark mode toggle dans la navbar top.

### 11 pages

| Page | Route | Affichage | Actions |
|------|-------|-----------|---------|
| Dashboard | `/admin` | `ResourceCard` agrégées : nodes, VMs, stockage, état Proxmox | — |
| Nodes | `/admin/nodes` | Grid `ResourceCard` : nom, CPU%, RAM%, uptime, statut | — |
| Storage | `/admin/storage` | `DataTable` : nom, type, total/utilisé/libre + barre | — |
| VMs | `/admin/vms` | `DataTable` : VMID, nom, node, statut, owner, pool | start/stop/reboot par ligne |
| Pools | `/admin/pools` | Liste + forms | Créer (dialog), supprimer (ConfirmDialog) |
| Tags | `/admin/tags` | Chips + `TagInput` | Créer, supprimer (confirmation) |
| Limites | `/admin/limits` | Formulaire : CPU/RAM/disk min-max, max VMs/user, max snapshots | Sauvegarder (PUT) |
| VMBR | `/admin/vmbr` | `DataTable` : nom bridge, VLAN-aware, ports | — |
| Cloud-Init | `/admin/cloudinit` | `DataTable` + toggle activé/désactivé | Créer, éditer (dialog), supprimer, toggle |
| ISO | `/admin/iso` | `DataTable` : nom, taille, storage | — |
| App Info | `/admin/appinfo` | Cards diagnostics : version, env, connexion Proxmox | — |

### Patterns UX communs à toutes les pages

- **Loading** : `LoadingSkeleton` pendant le fetch initial
- **Erreur** : `ErrorBanner` avec retry
- **Vide** : `EmptyState` avec CTA contextuel
- **Actions destructives** : toujours via `ConfirmDialog` avant exécution
- **Feedback** : toast Sonner sur chaque action (succès + erreur)
- **Refresh** : re-fetch automatique de la liste après chaque mutation

---

## API backend Go (nouveaux endpoints)

### Middleware admin

```go
// backend/api/v1/admin_middleware.go
// Vérifie le claim isAdmin dans le JWT — retourne 403 sinon
```

### Endpoints

```
GET  /api/v1/admin/nodes
GET  /api/v1/admin/storage
GET  /api/v1/admin/vms
POST /api/v1/admin/vms/:id/action

GET    /api/v1/admin/pools
POST   /api/v1/admin/pools
DELETE /api/v1/admin/pools/:id

GET    /api/v1/admin/tags
POST   /api/v1/admin/tags
DELETE /api/v1/admin/tags/:name

GET  /api/v1/admin/limits
PUT  /api/v1/admin/limits

GET  /api/v1/admin/vmbr

GET    /api/v1/admin/cloudinit
POST   /api/v1/admin/cloudinit
PUT    /api/v1/admin/cloudinit/:id
DELETE /api/v1/admin/cloudinit/:id
POST   /api/v1/admin/cloudinit/:id/toggle

GET  /api/v1/admin/iso
GET  /api/v1/admin/appinfo
```

### Organisation des fichiers Go

```
backend/api/v1/
  admin_middleware.go   ← vérification claim isAdmin (JWT)
  admin_handlers.go     ← handlers lecture (nodes, storage, vmbr, iso, appinfo)
  admin_mutations.go    ← handlers écriture (pools, tags, limits, cloudinit)
  admin_vms.go          ← handler VMs admin + actions
  admin_mapper.go       ← conversion types Proxmox → types JSON réponse
```

### Catch-all Go pour le SPA admin

```go
// Ajouté dans backend/main.go ou backend/app/app.go
// Assets statiques Svelte
router.ServeFiles("/admin/assets/*filepath", http.Dir("frontend-svelte/build/assets"))
// Toutes les routes /admin/* → index.html (SPA routing)
router.GET("/admin/*path", spaAdminFallback)
```

---

## Structure frontend-svelte/

```
frontend-svelte/
  src/
    lib/
      api/
        client.ts               ← fetch wrapper : JWT header, retry sur 401→exchange
        admin/
          nodes.ts
          storage.ts
          vms.ts
          pools.ts
          tags.ts
          limits.ts
          vmbr.ts
          cloudinit.ts
          iso.ts
          appinfo.ts
      components/
        ui/                     ← shadcn-svelte (générés)
        layout/
          AppShell.svelte
          AdminSidebar.svelte
          PageHeader.svelte
          ThemeToggle.svelte
        data/
          DataTable.svelte
          ResourceCard.svelte
          StatusBadge.svelte
          EmptyState.svelte
          LoadingSkeleton.svelte
        forms/
          ConfirmDialog.svelte
          InlineEdit.svelte
          TagInput.svelte
        feedback/
          ErrorBanner.svelte
      stores/
        auth.svelte.ts          ← Svelte 5 runes : token, isAdmin, exchange()
        theme.svelte.ts         ← dark/light mode persisté en localStorage
      types/
        admin.ts                ← Node, Storage, VM, Pool, Tag, Limits, VMBR, CloudInit, ISO
        api.ts                  ← ApiError, ApiResponse<T>
      utils/
        format.ts               ← formatBytes, formatCpu, formatUptime
    routes/
      +layout.svelte            ← root layout : init auth, thème
      +layout.ts                ← bootstrap exchange → redirect /login si échec
      +page.svelte              ← stub (pages user, phase ultérieure)
      admin/
        +layout.svelte          ← AdminGuard + AppShell + AdminSidebar
        +layout.ts              ← vérification isAdmin → redirect / si non-admin
        +page.svelte            ← Dashboard admin
        nodes/+page.svelte
        storage/+page.svelte
        vms/+page.svelte
        pools/+page.svelte
        tags/+page.svelte
        limits/+page.svelte
        vmbr/+page.svelte
        cloudinit/+page.svelte
        iso/+page.svelte
        appinfo/+page.svelte
    app.html
    app.css
  static/
    favicon.ico
  svelte.config.js              ← adapter-static, fallback index.html
  vite.config.ts                ← proxy /api/* → :50000
  tailwind.config.ts
  components.json               ← shadcn-svelte Mira preset
  tsconfig.json
  package.json
```

---

## Build & développement

### Dev local

```bash
# Terminal 1
make dev-api              # go run ./backend — API sur :50000

# Terminal 2
cd frontend-svelte
npm run dev               # Vite dev server sur :5173
                          # proxy /api/* → localhost:50000
```

### Makefile — nouvelles cibles

```makefile
frontend-install:   cd frontend-svelte && npm ci
frontend-build:     cd frontend-svelte && npm run build
frontend-dev:       cd frontend-svelte && npm run dev
dev:                make dev-api & make frontend-dev   # concurrently
build:              make frontend-build && go build && docker build
```

### Dockerfile

```dockerfile
FROM node:24-alpine AS frontend
WORKDIR /app
COPY frontend-svelte/ .
RUN npm ci && npm run build

# ... étape Go builder existante ...

FROM gcr.io/distroless/static-debian13:nonroot AS final
COPY --from=frontend /app/build /app/frontend-svelte/build
# ... reste inchangé ...
```

---

## Ce qui ne change pas dans cette phase

- `frontend/` reste intact (renommé `frontend-legacy/` pour référence, inchangé fonctionnellement)
- Tous les handlers templ existants (pages utilisateur, login)
- Session auth middleware, CSRF, session cookies
- Endpoints `/api/v1/auth/login`, `/api/v1/auth/exchange`, `/api/v1/vms`, `/api/v1/vms/:id`
- Tests Go existants (offline + integration)

---

## Critères de succès de la phase

- 100% des pages admin actuelles sont disponibles dans le SPA Svelte avec le même niveau de fonctionnalité
- Les composants `DataTable`, `ResourceCard`, `ConfirmDialog`, `EmptyState` sont génériques et réutilisables sans modification pour les pages utilisateur
- Aucune régression sur les pages utilisateur existantes (templ)
- Dark mode fonctionnel sur toutes les pages admin
- Responsive sur desktop et tablette

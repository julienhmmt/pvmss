# Frontend Rewrite — Gaps & Remaining Work

**Date:** 2026-04-10
**Référence:** [design](./2026-03-06-frontend-rewrite-design.md) · [implémentation](./2026-03-06-frontend-rewrite-implementation.md) · [modernisation](./2026-04-10-frontend-modernisation.md)
**Périmètre:** Ce document liste uniquement ce qui est **absent** ou **incomplet** par rapport au plan de design approuvé.

---

## État résumé

| Phase | Description | État |
|-------|-------------|------|
| 1 — Foundation | SvelteKit + API client + auth | ✅ Fait (partiel, voir gaps) |
| 2 — Core user pages | Home, VM details, VM create, Search, Profile | 🔶 Partiel |
| 3 — VM Console | noVNC intégré | 🔶 Partiel |
| 4 — Admin pages | Toutes les pages admin | ✅ Fait (structure présente) |
| 5 — Polish & Cutover | Migration finale, suppression legacy | ❌ Non démarré |

---

## Gap 1 — Backend API manquants

### 1.1 Health endpoints (publics)

```bash
GET  /api/v1/health          → { status, version }
GET  /api/v1/health/proxmox  → { connected, url }
```

Utilisés par le frontend pour afficher l'état de connexion Proxmox avant login.

### 1.2 Recherche VM

```bash
GET  /api/v1/search/vms?q=&filter=vmid|name|tags  → [VM]
```

La page `/search` existe côté frontend mais fait appel à un endpoint absent.

### 1.3 Changement de mot de passe

```bash
PUT  /api/v1/auth/me/password  body: { current, new }  → 204
```

Nécessaire pour la page profil.

### 1.4 Toggle NIC

```bash
POST /api/v1/vms/:id/network/:iface/toggle  → 204
```

Présent dans le design, absent dans `router.go`.

---

## Gap 2 — Routes frontend manquantes

### 2.1 Page profil

Fichier absent : `src/routes/(app)/profile/+page.svelte`

Contenu attendu (selon design) :

- Info utilisateur (username, pool, nombre de VMs)
- Liste des VMs de l'utilisateur (réutiliser VmCard)
- Formulaire changement de mot de passe

### 2.2 Page d'erreur

Fichier absent : `src/routes/+error.svelte`

Page 404 / 500 avec message propre et lien retour accueil.

### 2.3 Structure route console

**Actuel :** `/console/+page.svelte` (route racine indépendante)
**Attendu (design) :** `vm/[id]/console/+page.svelte` (imbriquée sous VM)

Impact : les paramètres `vmid` et `node` doivent être passés via URL ou query string. La structure imbriquée permet d'accéder au contexte VM directement depuis `$page.params`.

---

## Gap 3 — Stores manquants

### 3.1 `src/lib/stores/vms.svelte.ts`

Store global pour la liste de VMs :

```typescript
// État attendu
let vms = $state<VM[]>([]);
let loading = $state(false);
let error = $state<string | null>(null);

// Méthodes attendues
async function fetchVMs(): Promise<void>
async function vmAction(id: number, action: VMAction): Promise<void>
```

Actuellement la liste VM est rechargée localement dans chaque page.

### 3.2 `src/lib/stores/settings.svelte.ts`

Cache des settings applicatifs (nodes, storages, VMBRs, limites) pour éviter des appels répétés :

```typescript
let settings = $state<AppSettings | null>(null);
async function fetchSettings(): Promise<void>
```

---

## Gap 4 — Types TypeScript manquants

### 4.1 `src/lib/types/vm.ts`

Type `VM` général (list + details) absent en tant que fichier dédié. Actuellement éparpillé entre `vm-create.ts` et `vm-details.ts`. Consolider :

```typescript
interface VM {
  vmid: number; name: string; node: string;
  status: 'running' | 'stopped' | 'paused';
  cpu: number; maxcpu: number;
  mem: number; maxmem: number;
  disk: number; maxdisk: number;
  uptime: number; tags: string[]; description: string; pool: string;
  disks: Disk[]; networks: NetworkCard[]; cloudinit?: CloudInitConfig;
}
```

### 4.2 `src/lib/types/auth.ts`

Types d'authentification absents :

```typescript
interface User { username: string; isAdmin: boolean; pool?: string; vmCount?: number; }
interface LoginRequest { username: string; password: string; }
interface LoginResponse { access_token: string; user: User; }
```

### 4.3 `src/lib/types/settings.ts`

```typescript
interface AppSettings { nodes: string[]; storages: Storage[]; vmbrs: VMBR[]; limits: Limits; }
```

---

## Gap 5 — Clients API manquants

### 5.1 `src/lib/api/snapshots.ts`

```typescript
listSnapshots(vmid: number): Promise<Snapshot[]>
createSnapshot(vmid: number, body: { name: string; description: string }): Promise<{ task: string }>
deleteSnapshot(vmid: number, name: string): Promise<{ task: string }>
rollbackSnapshot(vmid: number, name: string): Promise<{ task: string }>
```

### 5.2 `src/lib/api/console.ts`

```typescript
getVNCTicket(vmid: number): Promise<{ ticket: string; port: number }>
buildWebSocketURL(vmid: number, ticket: string, port: number): string
```

### 5.3 `src/lib/api/search.ts`

```typescript
searchVMs(q: string, filter?: 'vmid' | 'name' | 'tags'): Promise<VM[]>
```

### 5.4 `src/lib/api/admin/settings.ts`

```typescript
getSettings(): Promise<AppSettings>
```

---

## Gap 6 — Composants manquants

### 6.1 Guards de navigation

- `src/lib/components/layout/AuthGuard.svelte` — redirige vers `/login` si non authentifié
- `src/lib/components/layout/AdminGuard.svelte` — redirige vers `/` si non admin

Actuellement la protection est probablement dans les layouts mais sans composant dédié réutilisable.

### 6.2 `Footer.svelte`

`src/lib/components/layout/Footer.svelte` — version, liens utiles.

### 6.3 Composants VM dédiés (pour VM Create et VM Details)

Ces composants sont inlinés dans les pages (monolithiques). Le plan de modernisation `2026-04-10-frontend-modernisation.md` couvre la découpe de VM Create et VM Details. Le présent document note que les composants réutilisables entre pages sont également absents :

| Composant | Usage |
| --------- | ----- |
| `VmCard.svelte` | Home + Profile + Search |
| `VmStatusBadge.svelte` | Partout où le statut est affiché |
| `VmActionButtons.svelte` | Home + VM Details |
| `VmConsole.svelte` | Route console |

---

## Gap 7 — Phase 5 Cutover (non démarrée)

### 7.1 Activation du SPA dans Go

Le backend Go ne sert pas encore le build SvelteKit. Il faut :

- [ ] Handler Go qui sert `frontend-svelte/build/` comme assets statiques
- [ ] Catch-all route : tout chemin non-API → `index.html`
- [ ] S'assurer que `/api/v1/*` a priorité sur le catch-all

### 7.2 Migration legacy

- [ ] Renommer `frontend/` → `frontend-legacy/`
- [ ] Mettre à jour Makefile : cibles `frontend-build`, `frontend-dev`, `dev`, `build`
- [ ] Mettre à jour `Dockerfile` pour inclure `frontend-svelte/build/`

### 7.3 Suppression code mort Go

- [ ] Tous les fichiers `.templ` dans `backend/components/`
- [ ] `backend/templates/` (template loading)
- [ ] Session middleware (`backend/middleware/session.go`)
- [ ] CSRF middleware (`backend/middleware/csrf.go`, `backend/security/csrf.go`)
- [ ] Handlers form-based (remplacés par les handlers API)
- [ ] Routes statiques legacy : `/css/*`, `/js/*`, `/webfonts/*`, `/vendor/*`, `/src/*`

### 7.4 Documentation

- [ ] Mettre à jour `CLAUDE.md` avec la nouvelle architecture
- [ ] Mettre à jour `README.md`

---

## Ordre de traitement recommandé

```bash
Gap 1 (API backend) → Gap 5 (clients API) → Gap 4 (types) → Gap 3 (stores)
                                                              ↓
Gap 2.1 (profile) → Gap 2.2 (error page) → Gap 2.3 (console route)
                                                              ↓
Gap 6.1 (guards) → Gap 6.3 (VmCard etc.) → Gap 7 (cutover)
```

Gap 7 (cutover) est le dernier — il ne peut être fait qu'une fois toutes les pages fonctionnelles.

---

## Ce plan ne couvre PAS

- Améliorations de qualité de code des pages existantes → voir [2026-04-10-frontend-modernisation.md](./2026-04-10-frontend-modernisation.md)
- Tests E2E Playwright
- WebSocket temps réel pour métriques

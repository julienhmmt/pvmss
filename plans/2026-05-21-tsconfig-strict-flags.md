# Plan : Activer `exactOptionalPropertyTypes` + `noUncheckedIndexedAccess`

> Date : 2026-05-21
> Périmètre : `frontend/tsconfig.json` + fixes appel par appel
> Stack : Svelte 5 + SvelteKit 2 + TypeScript strict
> Prérequis : Phases 1-4 du plan `2026-05-20-typescript-type-safety.md` terminées
>
> **Statut (vérifié 2026-06-09) : NON commencé.** Aucun des deux flags n'est dans `frontend/tsconfig.json`.
> ⚠️ Le décompte d'erreurs (79 sur 24 fichiers) date du 2026-05-21 — re-mesurer avant de commencer :
> activer les deux flags temporairement puis `npx svelte-check` pour obtenir le profil à jour.
> Note : si le découpage de `vm/create/+page.svelte` (plan frontend-modernisation Phase 5) est fait d'abord,
> les ~4 erreurs d'accès indexé de cette page seront à corriger dans les nouveaux fichiers `_steps/`.

---

## Contexte

Le plan `2026-05-20-typescript-type-safety.md` (Phase 5.3) recommande d'ajouter :

```json
{
  "compilerOptions": {
    "exactOptionalPropertyTypes": true,
    "noUncheckedIndexedAccess": true
  }
}
```

Activation immédiate des deux options sur `frontend/tsconfig.json` produit **79 erreurs** réparties sur **24 fichiers**. Ces erreurs sont des bugs latents que le typage actuel masque, mais le volume nécessite un travail de nettoyage dédié.

---

## Profil des erreurs

| Catégorie | Cas | Cause |
| --- | --- | --- |
| 28 | `exactOptionalPropertyTypes` — propriétés `?:` recevant `T \| undefined` explicite | Construction d'objets via spread / assignations conditionnelles |
| 17 | `noUncheckedIndexedAccess` — `Object is possibly 'undefined'` | Accès indexé `arr[i]` / `record[key]` |
| 2 | `Expression produces a union type that is too complex to represent` | Composants `bits-ui` (progress, dropdown-menu-radio-group) |
| ~32 | Mix : params optionnels d'API, `URL.searchParams.get`, déstructuration de tuples | Voir détail ci-dessous |

---

## Fichiers affectés

### Couche API (3)

- `src/lib/api/client.ts` — `body: string \| undefined` passé à `fetch(RequestInit)` (3 occurrences)
- `src/lib/api/console.ts` — `consoleToken?: string` passé avec valeur `undefined` explicite
- `src/lib/api/admin/db.ts` — extraction `regex.match()[1]` peut être `undefined`

### Composants (4)

- `src/lib/components/admin/AuditLog.svelte` — `table: string \| undefined` → `AuditLogParams.table?: string`
- `src/lib/components/data/Paginator.svelte` — accès indexé sur tableau
- `src/lib/components/ui/dropdown-menu/dropdown-menu-radio-group.svelte` — union complexity (upstream `bits-ui`)
- `src/lib/components/ui/progress/progress.svelte` — union complexity (upstream `bits-ui`)
- `src/lib/components/vm/VMDiskAddModal.svelte` — assignation `string \| undefined`

### Routes (15)

| Route | Erreurs typiques |
| --- | --- |
| `(app)/profile/+page.svelte` | `VMPaginationParams.search` reçoit `string \| undefined` |
| `(app)/search/+page.svelte` | `VMSearchParams.q/status/node` idem |
| `(app)/vm/[id]/+page.svelte` | `params.id` (`string \| undefined`) passé à `parseInt` |
| `(app)/vm/[id]/console/+page.svelte` | `$page.url.searchParams.get('name')` peut être `null` |
| `(app)/vm/create/+page.svelte` | 4× accès indexé sur tableaux (profiles, storages) |
| `admin/+page.svelte` | Accès indexé sur sections |
| `admin/appinfo/+page.svelte` | Accès indexé |
| `admin/cloudinit/+page.svelte` | Champs optionnels SFTP |
| `admin/iso/+page.svelte` | Accès indexé |
| `admin/limits/+page.svelte` | `Limits.nodes[name]` |
| `admin/profiles/+page.svelte` | Champs `?:` étendus |
| `admin/tags/+page.svelte` | `Tag.color?: string` |
| `admin/userpool/+page.svelte` | Spread conditionnel |
| `admin/vmbr/+page.svelte` | Accès indexé |
| `admin/vms/+page.svelte` | `AdminVMPaginationParams.search/node` idem profile |
| `docs/[type]/+page.svelte` | `params.type` peut être `undefined` |

---

## Stratégie de fix

### Pattern 1 — Params d'API paginés (8 fichiers, ~12 erreurs)

Objets `VMPaginationParams`, `VMSearchParams`, `AuditLogParams`, `AdminVMPaginationParams` sont construits comme :

```typescript
// Avant — `search: string | undefined` viole exactOptionalPropertyTypes
const params = { page, limit, search, sortBy, sortOrder };

// Option A — omettre la clé quand undefined
const params: VMPaginationParams = { page, limit, sortBy, sortOrder };
if (search) params.search = search;

// Option B — élargir le type côté définition
export interface VMPaginationParams {
  search?: string | undefined; // accepte les deux
}
```

**Recommandation :** Option A (plus propre). Centralise la construction dans un helper si répété.

### Pattern 2 — `URL.searchParams.get()` / `params.id` (4 fichiers)

```typescript
// Avant
const id = parseInt($page.params.id, 10);

// Après — guard explicite
const idStr = $page.params.id;
if (!idStr) throw new Error('Missing id');
const id = parseInt(idStr, 10);
```

### Pattern 3 — Accès indexé sur tableaux/records (10 fichiers, 17 erreurs)

```typescript
// Avant — first peut être undefined
const first = items[0];
first.id;

// Après — guard ou non-null assertion contrôlée
const first = items[0];
if (!first) return;
first.id;
```

### Pattern 4 — `body: string | undefined` dans fetch (1 fichier, 3 erreurs)

```typescript
// Avant
fetch(url, { method: 'POST', body: payload ? JSON.stringify(payload) : undefined });

// Après — spread conditionnel
fetch(url, { method: 'POST', ...(payload && { body: JSON.stringify(payload) }) });
```

### Pattern 5 — Composants `bits-ui` (2 erreurs, upstream)

Les types `bits-ui` ne sont pas compatibles avec `exactOptionalPropertyTypes` (Svelte HTML attributes typings utilisent `T | undefined | null` au lieu de `?:`). Options :

- **A** — Attendre fix upstream `bits-ui` (recommandé, vérifier issues)
- **B** — Wrapper local avec `@ts-expect-error` ciblé
- **C** — Désactiver `exactOptionalPropertyTypes` uniquement pour ces fichiers via override (non supporté par TS)

**Recommandation :** B avec commentaire pointant vers issue upstream.

---

## Ordre d'exécution

1. **Activer `noUncheckedIndexedAccess` seul** dans `tsconfig.json`
   - Réduit à ~17 erreurs (Pattern 2, 3)
   - Fix tous les accès indexés + URL params
   - Vérifier `make frontend-build` clean
   - Commit isolé

2. **Activer `exactOptionalPropertyTypes`** dans un second commit
   - Fix Pattern 1 (API params) en masse
   - Fix Pattern 4 (fetch body)
   - Traiter cas `bits-ui` en dernier (Pattern 5)
   - Commit isolé

3. **Vérification finale** — `npx svelte-check` → 0 erreur

---

## Risques

- **Volume** : ~79 fixes ponctuels, chacun trivial mais long
- **bits-ui upstream** : 2 erreurs hors de notre contrôle — peuvent nécessiter `@ts-expect-error`
- **Régression runtime** : pattern 3 (accès indexé avec guard) peut changer le comportement si du code dépendait implicitement de `undefined` propagation — tester chaque route après fix

---

## Estimation

- Phase 1 (`noUncheckedIndexedAccess`) : ~2h, ~17 fixes
- Phase 2 (`exactOptionalPropertyTypes`) : ~3h, ~62 fixes
- Total : ~5h de travail concentré

---

## Bénéfice

Les options détectent deux classes de bugs réellement présents :

1. **Accès indexé non vérifié** — `vms[0].name` quand `vms` peut être vide → runtime `TypeError`
2. **Props optionnelles ambiguës** — passer `undefined` explicite vs omettre la clé change le comportement de l'API consommatrice (ex. spread, `Object.assign`)

Le typage actuel laisse passer ces bugs ; ces flags les rendent visibles à la compilation.

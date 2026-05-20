# Plan : TypeScript Type Safety — Frontend

> Date : 2026-05-20
> Périmètre : couche API, composants, routes, types partagés
> Stack : Svelte 5 + SvelteKit 2 + TypeScript strict

---

## Diagnostic

### Ce qui est bien

- `"strict": true` activé dans `tsconfig.json`
- Types centralisés sous `src/lib/types/` (`vm.ts`, `admin.ts`, `auth.ts`, `settings.ts`, `vm-create.ts`)
- `$props()` rune avec `interface Props` dans chaque composant
- Utilitaire `transformKeysToCamelCase<T>()` correctement génériqué
- Stores typés avec interfaces d'état explicites

### Ce qui manque

| Problème | Impact | Fichiers concernés |
| --- | --- | --- |
| `Record<string,unknown>` comme type intermédiaire dans tous les appels API | Aucune sécurité de type sur les réponses backend | Tous les `src/lib/api/**/*.ts` |
| `status: string` au lieu de `VMStatus` dans les composants | États invalides possibles à l'exécution | `VmActionButtons`, `StatusBadge`, `VmCard` |
| `action: string` au lieu d'un type union dans les callbacks | Valeurs d'action arbitraires non détectées à la compilation | `VmCard`, `VmActionButtons` |
| `e as Error` dans les routes (assertion non sécurisée) | Crash si la valeur lancée n'est pas une `Error` | Toutes les routes `(app)` |
| `catch {}` silencieux (9 occurrences dans vm/[id]) | Erreurs avalées sans trace | `routes/(app)/vm/[id]/+page.svelte` |
| `icon: any` dans `NavLink` | Les composants icône ne sont pas vérifiés | `src/lib/types/navbar.ts` |
| `[key: string]: any` dans `env.d.ts` | Tous les composants Svelte acceptent n'importe quelle prop | `src/env.d.ts` |
| Interface `Snapshot` dupliquée dans `snapshots.ts` | Désynchronisation silencieuse possible | `src/lib/api/snapshots.ts` |

---

## Phase 1 — Couche API : éliminer `Record<string,unknown>` (CRITIQUE)

### Contexte

Chaque appel API utilise actuellement un cast intermédiaire qui annule la sécurité de type :

```typescript
// Avant — aucune valeur au niveau générique
const res = await api.get<Record<string, unknown>>("/api/v1/vms");
return transformKeysToCamelCase<PaginatedVMListResponse>(res);

// Après — le type cible est déclaré dès l'appel
const res = await api.get<PaginatedVMListResponse>("/api/v1/vms");
return transformKeysToCamelCase<PaginatedVMListResponse>(res);
```

### Fichiers à mettre à jour

Remplacer `api.get<Record<string, unknown>>`, `api.post<Record<string, unknown>>`, etc. par le type de réponse cible dans chacun des fichiers suivants :

| Fichier | Occurrences estimées |
| --- | --- |
| `src/lib/api/vms.ts` | 3 |
| `src/lib/api/vm-details.ts` | 4 |
| `src/lib/api/vm-create.ts` | 3 |
| `src/lib/api/snapshots.ts` | 3 |
| `src/lib/api/tasks.ts` | 2 |
| `src/lib/api/search.ts` | 2 |
| `src/lib/api/console.ts` | 2 |
| `src/lib/api/auth.ts` | 3 |
| `src/lib/api/setup.ts` | 2 |
| `src/lib/api/admin/appinfo.ts` | 1 |
| `src/lib/api/admin/audit.ts` | 2 |
| `src/lib/api/admin/cloudinit.ts` | 2 |
| `src/lib/api/admin/db.ts` | 1 |
| `src/lib/api/admin/iso.ts` | 2 |
| `src/lib/api/admin/limits.ts` | 2 |
| `src/lib/api/admin/nodes.ts` | 2 |
| `src/lib/api/admin/profiles.ts` | 2 |
| `src/lib/api/admin/settings-overview.ts` | 1 |
| `src/lib/api/admin/settings.ts` | 3 |
| `src/lib/api/admin/storage.ts` | 2 |
| `src/lib/api/admin/tags.ts` | 2 |
| `src/lib/api/admin/userpool.ts` | 2 |
| `src/lib/api/admin/vmbr.ts` | 2 |
| `src/lib/api/admin/vms.ts` | 3 |

Total : ~55 occurrences

---

## Phase 2 — Composants : renforcer les types de props (CRITIQUE)

### 2.1 `status: string` → `VMStatus`

`VMStatus = "running" | "stopped" | "paused"` existe déjà dans `src/lib/types/vm.ts`.
Il suffit de l'importer et de l'utiliser dans les `interface Props` :

```typescript
// Avant
interface Props {
  status: string;
}

// Après
import type { VMStatus } from '$lib/types/vm';
interface Props {
  status: VMStatus;
}
```

**Fichiers à mettre à jour :**

- `src/lib/components/data/VmActionButtons.svelte`
- `src/lib/components/data/StatusBadge.svelte`
- `src/lib/components/data/VmCard.svelte`

### 2.2 `action: string` → type union `VMAction`

Le type union `VMAction` doit être créé (ou vérifié dans `vm.ts`) puis importé :

```typescript
// À ajouter dans src/lib/types/vm.ts si absent
export type VMAction = "start" | "stop" | "reboot" | "shutdown" | "suspend" | "resume" | "delete" | "clone" | "console";

// Dans les composants
import type { VMAction } from '$lib/types/vm';
interface Props {
  onAction: (action: VMAction) => void;
}
```

**Fichiers à mettre à jour :**

- `src/lib/components/data/VmCard.svelte`
- `src/lib/components/data/VmActionButtons.svelte`

---

## Phase 3 — Routes : corriger la gestion d'erreurs (CRITIQUE)

### 3.1 Remplacer `e as Error` par un guard `instanceof`

```typescript
// Avant — assertion non sécurisée
} catch (e) {
  error = e as Error;
}

// Après — guard sûr (pattern déjà utilisé dans les stores)
} catch (err: unknown) {
  error = err instanceof Error ? err : new Error(String(err));
}
```

### 3.2 Remplacer les `catch {}` silencieux

```typescript
// Avant — erreur avalée
} catch { }

// Après — log minimal ou propagation contrôlée
} catch (err: unknown) {
  console.error("Action failed:", err instanceof Error ? err.message : String(err));
}
```

**Fichiers à mettre à jour :**

- `src/routes/(app)/home/+page.svelte`
- `src/routes/(app)/vm/[id]/+page.svelte` (9 occurrences)
- `src/routes/(app)/profile/+page.svelte`
- `src/routes/(public)/setup/+page.svelte`

---

## Phase 4 — Types partagés : nettoyer les `any` et doublons (HAUTE)

### 4.1 `NavLink.icon: any` → `ComponentType`

```typescript
// Avant — src/lib/types/navbar.ts
export interface NavLink {
  icon: any;
}

// Après
import type { Component } from 'svelte';
export interface NavLink {
  icon: Component;
}
```

**Fichier :** `src/lib/types/navbar.ts`

### 4.2 Supprimer `[key: string]: any` dans `env.d.ts`

Ce `declare module "*.svelte"` avec `[key: string]: any` annule la vérification des props sur tous les composants. La ligne doit être supprimée — SvelteKit génère correctement les types des composants sans elle.

**Fichier :** `src/env.d.ts`

### 4.3 Supprimer le doublon `Snapshot` dans `snapshots.ts`

L'interface `Snapshot` définie localement dans `src/lib/api/snapshots.ts` duplique celle de `src/lib/types/vm.ts`. Supprimer la définition locale et importer depuis les types partagés.

**Fichier :** `src/lib/api/snapshots.ts`

---

## Phase 5 — Améliorations optionnelles (MOYENNE)

### 5.1 Créer `src/lib/types/errors.ts`

Centraliser les types d'erreur pour éviter les imports depuis la couche API dans les routes :

```typescript
// src/lib/types/errors.ts
export type AppError = Error & { status?: number; code?: string };

export function toError(err: unknown): AppError {
  return err instanceof Error ? err : new Error(String(err));
}
```

Les stores et routes importent `toError` au lieu de répéter le ternaire `instanceof`.

**Fichier à créer :** `src/lib/types/errors.ts`

### 5.2 Créer `src/lib/types/callbacks.ts`

Les types de callback VM sont répétés dans plusieurs composants. Les centraliser :

```typescript
// src/lib/types/callbacks.ts
import type { VMSummary } from './vm';
import type { VMAction } from './vm';

export type VMActionCallback = (action: VMAction) => void;
export type VMCardActionCallback = (vm: VMSummary, action: VMAction) => void;
```

**Fichier à créer :** `src/lib/types/callbacks.ts`

### 5.3 Renforcer `tsconfig.json`

`"strict": true` couvre `noImplicitAny` et `strictNullChecks`. Ajouter :

```json
{
  "compilerOptions": {
    "exactOptionalPropertyTypes": true,
    "noUncheckedIndexedAccess": true
  }
}
```

Ces deux options détectent une classe supplémentaire de bugs liés aux accès par index et aux props optionnelles.

**Fichier :** `tsconfig.json`

---

## Récapitulatif des fichiers

| Action | Fichier(s) | Phase |
| --- | --- | --- |
| **Mettre à jour** — remplacer le générique `Record<string,unknown>` | Tous `src/lib/api/**/*.ts` | 1 |
| **Mettre à jour** — props `status` et `onAction` | `VmActionButtons`, `StatusBadge`, `VmCard` | 2 |
| **Mettre à jour** — ajouter `VMAction` dans `vm.ts` si absent | `src/lib/types/vm.ts` | 2 |
| **Mettre à jour** — gestion d'erreurs dans les routes | `routes/(app)/**/*.svelte` + setup | 3 |
| **Mettre à jour** — `icon: any` → `Component` | `src/lib/types/navbar.ts` | 4 |
| **Mettre à jour** — supprimer `[key: string]: any` | `src/env.d.ts` | 4 |
| **Supprimer** — interface `Snapshot` locale | `src/lib/api/snapshots.ts` | 4 |
| **Créer** — types d'erreur centralisés | `src/lib/types/errors.ts` | 5 |
| **Créer** — types de callback centralisés | `src/lib/types/callbacks.ts` | 5 |
| **Mettre à jour** — options tsconfig supplémentaires | `tsconfig.json` | 5 |

---

## Ordre d'exécution recommandé

1. **Phase 1** en premier — le plus d'impact, pas de risque de régression (les types de destination existent déjà)
2. **Phase 4** avant Phase 2 — supprimer `env.d.ts` `any` pour que les erreurs de Phase 2 soient visibles par le compilateur
3. **Phase 2** — les union types seront immédiatement vérifiés une fois `env.d.ts` nettoyé
4. **Phase 3** — gestion d'erreurs indépendante, peut se faire en parallèle avec Phase 2
5. **Phase 5** — optionnel, après que toutes les phases critiques passent `make frontend-build` sans erreur

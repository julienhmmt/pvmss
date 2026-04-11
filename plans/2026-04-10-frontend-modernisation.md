# Plan : Modernisation Frontend Svelte 5 — Zone Utilisateur

> Date : 2026-04-10
> Périmètre : pages utilisateur (home, create VM, VM details, search, docs)
> Stack confirmée : Svelte 5 + SvelteKit 2 + TypeScript + Tailwind 4 + shadcn/bits-ui

---

## Diagnostic

### Ce qui est bien

- Svelte 5 runes utilisés (`$state`, `$derived`, `@render`)
- API client centralisé et modulaire (`/lib/api/`)
- Composants UI shadcn complets (dialog, select, table, toast…)
- i18n EN/FR intégré
- Immutabilité respectée sur les actions (spread `{ ...actionLoading }`)

### Ce qui manque

| Manque                                          | Impact                                       | Fichiers concernés       |
| ----------------------------------------------- | -------------------------------------------- | ------------------------ |
| `$effect` absent, `onMount`/`onDestroy` partout | Cleanup fragile, fuites mémoire potentielles | Tous les pages           |
| Zéro `transition:` / `animate:`                 | App statique, pas moderne                    | Partout                  |
| Zéro actions Svelte (`use:`)                    | Pas de debounce, pas de focus trap           | Search, modals           |
| Validation formulaire absente                   | UX médiocre, données invalides               | VM Create                |
| VM Create = 1 fichier de 1 093 lignes           | Illisible, non testable                      | `vm/create/+page.svelte` |
| VM Details = 807 lignes monolithiques           | Onglets non isolés                           | `vm/[id]/+page.svelte`   |
| Styles `:global()` dupliqués entre pages        | Incohérences visuelles                       | home, search, vm/[id]    |
| `strict: false` TypeScript                      | Types implicites non vérifiés                | `tsconfig.json`          |
| `page` store utilisé à la place de runes        | Pattern obsolète dans Svelte 5               | search, docs             |

---

## Phase 1 — Fondations transversales (prerequis aux phases suivantes)

### 1.1 Actions Svelte réutilisables

Créer `/lib/actions/index.ts` avec :

```typescript
// use:debounce={{ handler: fn, delay: 300 }}
export function debounce(node, { handler, delay }) { ... }

// use:clickOutside={callback}
export function clickOutside(node, callback) { ... }

// use:autofocus
export function autofocus(node) { ... }

// use:intersect={callback}  (pour TOC actif dans docs)
export function intersect(node, callback) { ... }
```

**Fichiers à créer** : `frontend/src/lib/actions/index.ts`

### 1.2 Styles globaux partagés

Les classes `.pv-*` (badge, table, tab, action-btn…) sont dupliquées dans le `<style>` de chaque page. Les centraliser dans un seul fichier CSS importé par le layout.

**Fichiers** :

- Créer `frontend/src/lib/styles/pv-components.css`
- Modifier `frontend/src/routes/+layout.svelte` pour l'importer
- Supprimer les blocs `<style>` dupliqués dans chaque page

### 1.3 Remplacer `onMount`/`onDestroy` par `$effect`

Pattern cible :

```svelte
// Avant
onMount(() => {
  load();
  metricsInterval = setInterval(refreshMetrics, 5000);
});
onDestroy(() => clearInterval(metricsInterval));

// Après
$effect(() => {
  load();
  const id = setInterval(refreshMetrics, 5000);
  return () => clearInterval(id);
});
```

**Fichiers concernés** : home, vm/[id], search, docs/[type]

### 1.4 TypeScript strict

Activer `"strict": true` dans `tsconfig.json` et corriger les types implicites.

---

## Phase 2 — Animations & transitions

### 2.1 Transitions de page (layout)

Dans `routes/(app)/+layout.svelte`, ajouter un `transition:fade` sur le slot principal pour que chaque navigation soit fluide.

```svelte
<div transition:fade={{ duration: 150 }}>
  {@render children()}
</div>
```

### 2.2 Transitions dans les composants existants

| Composant                      | Transition recommandée                        |
| ------------------------------ | --------------------------------------------- |
| `LoadingSkeleton` → contenu    | `transition:fade` sur l'article/table         |
| Onglets VM Details             | `transition:fade` sur le contenu actif        |
| Formulaire snapshot            | `transition:slide` sur le panneau de création |
| Description editingDescription | `transition:slide` sur le textarea            |
| Résultats de recherche         | `transition:fade` sur la table entière        |
| Empty state → résultats        | `transition:fade`                             |
| ErrorBanner                    | `transition:fly` (vient d'en haut)            |

### 2.3 Animations de liste

Sur les listes qui changent (résultats search, snapshots) :

```svelte
{#each items as item (item.id)}
  <tr animate:flip={{ duration: 200 }}>...</tr>
{/each}
```

---

## Phase 3 — Search (`/search`)

### Problèmes actuels

- Recherche manuelle sur `Enter` seulement
- Pas de debounce : chaque frappe ne fait rien, puis soumission brutale
- URL sync manuel avec `goto()` + `URLSearchParams` à la main
- `knownNodes` accumulé manuellement avec un `for` loop
- Pattern `$page.url.searchParams` (non réactif sans `$derived`)

### Plan

**3.1 Debounce automatique**
Remplacer le handler `onkeydown` + bouton manuel par `use:debounce` sur l'input. La recherche se déclenche automatiquement 400ms après la dernière frappe. Le bouton reste pour déclencher manuellement.

**3.2 Filtres réactifs avec `$derived`**
Les nœuds connus doivent être `$derived` depuis `results`, pas un `$state` accumulé manuellement :

```typescript
const knownNodes = $derived([
  ...new Set(results.map((v) => v.node).filter(Boolean)),
]);
```

**3.3 URL sync propre**
Remplacer la sync manuelle par un `$effect` qui observe `q`, `filterStatus`, `filterNode` et met à jour l'URL :

```typescript
$effect(() => {
  const params = new URLSearchParams();
  if (q) params.set("q", q);
  if (filterStatus) params.set("status", filterStatus);
  if (filterNode) params.set("node", filterNode);
  goto(`/search${params.size ? "?" + params : ""}`, {
    replaceState: true,
    noScroll: true,
  });
});
```

**3.4 Page `$page` → rune**
Remplacer `$page.url.searchParams.get(...)` par l'accès via `page` rune de SvelteKit 2 :

```typescript
import { page } from "$app/state"; // Svelte 5 / SvelteKit 2
```

**3.5 Transitions**

- `transition:fade` sur la table de résultats
- `animate:flip` sur les lignes lors du rechargement
- Indicateur de chargement inline dans le bouton search (spinner)

**Fichier** : `frontend/src/routes/(app)/search/+page.svelte`
**Taille estimée finale** : ~150 lignes (vs 241 actuellement)

---

## Phase 4 — VM Details (`/vm/[id]`)

### Problèmes actuels

- 807 lignes, tous les onglets dans un seul fichier
- `setInterval` dans `onMount` / `onDestroy`
- États éparpillés (13 `$state` variables)
- Description edit : textarea en ligne sans transition
- Snapshot form : visible/caché sans animation

### Plan de découpage

```
routes/(app)/vm/[id]/
  +page.svelte                  ← orchestrateur (< 150 lignes)
  _tabs/
    TabOverview.svelte           ← table infos générales
    TabDisks.svelte              ← table disques
    TabNetwork.svelte            ← table réseau
    TabSnapshots.svelte          ← liste + formulaire création
    TabCloudInit.svelte          ← table cloud-init
  _components/
    VMActionBar.svelte           ← boutons start/stop/reboot/delete
    VMStatCards.svelte           ← 4 cartes métriques
    ConsoleBanner.svelte         ← bannière console
    EditableDescription.svelte   ← champ description éditable
```

**Page orchestratrice** (`+page.svelte`) :

- Gère uniquement : chargement, état global, `$effect` pour polling metrics
- Passe config/metrics aux onglets via props
- `$effect` remplace `onMount`/`onDestroy` pour le polling

**VMStatCards.svelte** :

- Les 4 stat cards (CPU, RAM, Disk, Network)
- Transition `transition:fade` sur les valeurs qui changent
- Compteur CPU animé (CSS `transition: width` sur la barre)

**TabSnapshots.svelte** :

- `transition:slide` sur le formulaire de création
- `animate:flip` sur la liste de snapshots

**EditableDescription.svelte** :

- Composant standalone avec `transition:slide` sur le textarea
- Interface : `value`, `onSave`, `loading`

**VMActionBar.svelte** :

- Boutons d'action VM
- Tooltips via composant `Tooltip` existant (shadcn)

---

## Phase 5 — VM Create (`/vm/create`)

### Problèmes actuels

- 1 093 lignes, impossible à maintenir
- ~25 `$state` variables pour le formulaire
- Validation absente : on peut soumettre n'importe quoi
- Pas de feedback visuel entre les steps
- Pas de persistance draft

### Plan

**5.1 Découpage en composants**

```
routes/(app)/vm/create/
  +page.svelte                  ← orchestrateur wizard (< 200 lignes)
  _steps/
    StepBase.svelte             ← nom, node, storage, ISO, tags
    StepHardware.svelte         ← CPU, RAM, bus, EFI, TPM
    StepDisk.svelte             ← liste disques + ajout/suppression
    StepNetwork.svelte          ← liste cartes réseau
    StepCloudInit.svelte        ← cloud-init config
    StepReview.svelte           ← récapitulatif + warnings
  _components/
    WizardProgress.svelte       ← barre de progression des steps
    DiskCard.svelte             ← carte disque individuelle (avec suppression)
    NetworkCard.svelte          ← carte réseau individuelle
```

**5.2 State management : store dédié**
Extraire l'état du formulaire dans un store Svelte réactif :

```typescript
// lib/stores/vm-create.svelte.ts
export function createVMFormStore() {
  const form = $state<VMCreateFormState>({ ... });
  const errors = $state<Partial<Record<keyof VMCreateFormState, string>>>({});

  function validate(step: Step): boolean { ... }
  function reset() { ... }

  return { form, errors, validate, reset };
}
```

**5.3 Validation par step**
Chaque step a sa propre fonction de validation appelée avant `next()` :

| Step      | Règles validées                                                        |
| --------- | ---------------------------------------------------------------------- |
| Base      | Nom requis, alphanumeric+tirets, node sélectionné, storage sélectionné |
| Hardware  | CPU ≥ 1 ≤ quota, RAM ≥ 512 MB ≤ quota                                  |
| Disk      | Au moins 1 disque, taille > 0                                          |
| Network   | Au moins 1 réseau, bridge sélectionné                                  |
| CloudInit | Si activé : user requis                                                |

**5.4 Transitions entre steps**
Utiliser `transition:fly` pour simuler une navigation forward/backward :

```svelte
{#key currentStep}
  <div
    in:fly={{ x: direction * 30, duration: 200 }}
    out:fly={{ x: direction * -30, duration: 200 }}
  >
    <!-- contenu du step -->
  </div>
{/key}
```

Variable `direction = +1` (forward) ou `-1` (backward).

**5.5 WizardProgress.svelte**
Barre de progression visuelle avec icônes par step, étape courante mise en valeur, étapes complètes cochées.

**5.6 Draft localStorage**
`$effect` qui persiste le formulaire :

```typescript
$effect(() => {
  localStorage.setItem(
    "vm-create-draft",
    JSON.stringify($state.snapshot(form)),
  );
});
// Au chargement : restore depuis localStorage si présent
```

---

## Phase 6 — Documentation (`/docs/[type]`)

### Problèmes actuels

- `{@html html}` directement sans DOMPurify (risque XSS si backend compromis)
- TOC extrait par regex depuis le HTML rendu (fragile)
- Section active dans le TOC non trackée
- Pas de scroll indicator

### Plan

**6.1 DOMPurify**
Ajouter `dompurify` et wrapper le rendu HTML :

```typescript
import DOMPurify from "dompurify";
// ...
const safeHtml = $derived(DOMPurify.sanitize(html));
// template : {@html safeHtml}
```

**6.2 TOC actif avec IntersectionObserver**
Action `use:intersect` créée en Phase 1 :

```svelte
// Sur chaque titre dans le prose
<h2 use:intersect={(id) => activeSection = id} id="...">
```

Ou post-rendu, observer les headings générés et mettre à jour `activeSection`.

**6.3 Section active dans le TOC**

```svelte
<button class="pv-doc-toc-entry {entry.id === activeSection ? 'active' : ''}">
```

Transition `transition:fade` sur l'indicateur actif.

**6.4 Bouton copy-code**
Post-rendu : scanner les `<pre><code>` dans l'article et injecter un bouton copier via une action `use:addCopyButtons`.

---

## Résumé des fichiers

### Nouveaux fichiers

```
frontend/src/lib/actions/index.ts
frontend/src/lib/styles/pv-components.css
frontend/src/lib/stores/vm-create.svelte.ts
frontend/src/routes/(app)/vm/[id]/_tabs/TabOverview.svelte
frontend/src/routes/(app)/vm/[id]/_tabs/TabDisks.svelte
frontend/src/routes/(app)/vm/[id]/_tabs/TabNetwork.svelte
frontend/src/routes/(app)/vm/[id]/_tabs/TabSnapshots.svelte
frontend/src/routes/(app)/vm/[id]/_tabs/TabCloudInit.svelte
frontend/src/routes/(app)/vm/[id]/_components/VMActionBar.svelte
frontend/src/routes/(app)/vm/[id]/_components/VMStatCards.svelte
frontend/src/routes/(app)/vm/[id]/_components/ConsoleBanner.svelte
frontend/src/routes/(app)/vm/[id]/_components/EditableDescription.svelte
frontend/src/routes/(app)/vm/create/_steps/StepBase.svelte
frontend/src/routes/(app)/vm/create/_steps/StepHardware.svelte
frontend/src/routes/(app)/vm/create/_steps/StepDisk.svelte
frontend/src/routes/(app)/vm/create/_steps/StepNetwork.svelte
frontend/src/routes/(app)/vm/create/_steps/StepCloudInit.svelte
frontend/src/routes/(app)/vm/create/_steps/StepReview.svelte
frontend/src/routes/(app)/vm/create/_components/WizardProgress.svelte
frontend/src/routes/(app)/vm/create/_components/DiskCard.svelte
frontend/src/routes/(app)/vm/create/_components/NetworkCard.svelte
```

### Fichiers modifiés

```
frontend/tsconfig.json                          ← strict: true
frontend/src/routes/+layout.svelte              ← import pv-components.css
frontend/src/routes/(app)/+layout.svelte        ← transition:fade sur navigation
frontend/src/routes/(app)/home/+page.svelte     ← $effect, transitions
frontend/src/routes/(app)/search/+page.svelte   ← $effect, debounce, transitions
frontend/src/routes/(app)/vm/[id]/+page.svelte  ← orchestrateur (< 150 lignes)
frontend/src/routes/(app)/vm/create/+page.svelte ← orchestrateur (< 200 lignes)
frontend/src/routes/docs/[type]/+page.svelte    ← DOMPurify, TOC actif
```

---

## Ordre d'exécution recommandé

```
Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6
```

Chaque phase est indépendante et déployable séparément.
Phase 1 est le prérequis de toutes les autres.
Phases 3, 4, 5, 6 peuvent se paralléliser une fois Phase 1+2 terminées.

---

## Ce plan ne couvre PAS (hors périmètre)

- Zone admin (pages admin/\*)
- Tests (couverture actuelle : 0%, sujet séparé)
- WebSocket temps réel pour les métriques (évolution future)
- Service Worker / offline support
- Accessibilité (ARIA, focus management) — partielle via bits-ui

# Plan : Modernisation Frontend Svelte 5 — Zone Utilisateur

> Date : 2026-04-10 — **mis à jour 2026-06-09 après audit du code**
> Périmètre : pages utilisateur (home, create VM, VM details, search, docs)
> Stack confirmée : Svelte 5 + SvelteKit 2 + TypeScript + Tailwind 4 + shadcn/bits-ui

---

## Statut (vérifié contre le code le 2026-06-09)

| Phase | Contenu | Statut |
| --- | --- | --- |
| 1 | Actions Svelte (`lib/actions/index.ts`), styles partagés, `$effect`, `strict: true` | ✅ DONE |
| 2 | Transitions & animations (fade/slide/flip) | ✅ DONE |
| 3 | Search : debounce, `$derived`, URL sync | ✅ DONE |
| 4 | VM Details découpé : `_tabs/` (5 fichiers) + `_components/` | ✅ DONE |
| 5 | VM Create découpage wizard | ❌ **TODO — priorité 1** |
| 6 | Docs : TOC actif + copy-code ✅ — DOMPurify | ⚠️ **DOMPurify manquant** |

---

## Phase 5 — VM Create (`/vm/create`) — TODO

### État actuel (pire qu'au moment du plan initial)

- `vm/create/+page.svelte` = **1 701 lignes** (1 093 au moment du plan — la dette a grossi de +55 %)
- ~25+ `$state` variables pour le formulaire
- Validation par step incomplète
- Pas de persistance draft

### Plan de découpage

```bash
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
    DiskCard.svelte             ← carte disque individuelle
    NetworkCard.svelte          ← carte réseau individuelle
```

Suivre le pattern déjà appliqué avec succès sur `vm/[id]/` (`_tabs/` + `_components/`).

### State management : store dédié

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

### Validation par step

| Step      | Règles validées                                                        |
| --------- | ---------------------------------------------------------------------- |
| Base      | Nom requis, alphanumeric+tirets, node sélectionné, storage sélectionné |
| Hardware  | CPU ≥ 1 ≤ quota, RAM ≥ 512 MB ≤ quota                                  |
| Disk      | Au moins 1 disque, taille > 0                                          |
| Network   | Au moins 1 réseau, bridge sélectionné                                  |
| CloudInit | Si activé : user requis                                                |

### Transitions entre steps

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

### Draft localStorage

```typescript
$effect(() => {
  localStorage.setItem("vm-create-draft", JSON.stringify($state.snapshot(form)));
});
// Au chargement : restore depuis localStorage si présent
```

---

## Phase 6 (reste) — DOMPurify sur `/docs/[type]` — TODO

TOC actif (IntersectionObserver) et boutons copy-code sont déjà en place.
Reste : le rendu `{@html html}` n'est **pas** sanitisé (risque XSS si backend compromis).

```typescript
import DOMPurify from "dompurify";
const safeHtml = $derived(DOMPurify.sanitize(html));
// template : {@html safeHtml}
```

Ajouter la dépendance via `npm add dompurify` (et `@types/dompurify` si besoin).

---

## Vérification

```bash
make frontend-build        # build clean
npx svelte-check           # 0 erreur
make test-offline          # backend serve toujours la SPA
```

Test manuel : créer une VM complète via le wizard (chaque step), recharger la page en cours de saisie (draft restauré), vérifier les pages docs EN/FR.

---

## Ce plan ne couvre PAS (hors périmètre)

- Zone admin (pages admin/\*)
- Tests frontend (couverture actuelle : 0 %, sujet séparé)
- WebSocket temps réel pour les métriques (évolution future)
- Service Worker / offline support
- Flags TS `exactOptionalPropertyTypes` / `noUncheckedIndexedAccess` → voir `plans/2026-05-21-tsconfig-strict-flags.md`

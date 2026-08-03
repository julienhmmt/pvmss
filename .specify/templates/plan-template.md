# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]

**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: [e.g., Python 3.11, Swift 5.9, Rust 1.75 or NEEDS CLARIFICATION]

**Primary Dependencies**: [e.g., FastAPI, UIKit, LLVM or NEEDS CLARIFICATION]

**Storage**: [if applicable, e.g., PostgreSQL, CoreData, files or N/A]

**Testing**: [e.g., pytest, XCTest, cargo test or NEEDS CLARIFICATION]

**Target Platform**: [e.g., Linux server, iOS 15+, WASM or NEEDS CLARIFICATION]

**Project Type**: [e.g., library/cli/web-service/mobile-app/compiler/desktop-app or NEEDS CLARIFICATION]

**Performance Goals**: [domain-specific, e.g., 1000 req/s, 10k lines/sec, 60 fps or NEEDS CLARIFICATION]

**Constraints**: [domain-specific, e.g., <200ms p95, <100MB memory, offline-capable or NEEDS CLARIFICATION]

**Scale/Scope**: [domain-specific, e.g., 10k users, 1M LOC, 50 screens or NEEDS CLARIFICATION]

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*
*Source: `.specify/memory/constitution.md` v1.1.0. Any ✗ must be justified in Complexity Tracking or the slice is not merged.*

| # | Gate | ✓/✗ | Note |
|---|------|-----|------|
| I | **Vocabulaire** — *gabarit* / *quota* / *capacité* employés correctement ; le mot « limite » seul n'apparaît nulle part | | |
| II | **API-first** — aucun endpoint réservé à la SPA ; tout ce que fait l'UI est faisable par API (cookie ou jeton Bearer) | | |
| III | **Resolve()** *(NON NÉGOCIABLE)* — tout accès en écriture à une VM passe par `Resolve(actor, cluster, vmid)` ; aucun `node` fourni par le client | | |
| IV | **Lecture / écriture** — la lecture sert la projection en mémoire, jamais Proxmox ; l'écriture passe par Proxmox puis invalide | | |
| V | **Multi-cluster** — identité `<cluster>:<vmid>`, décodée en un seul endroit ; un compte de service par cluster | | |
| VI | **Frontière serveur** *(NON NÉGOCIABLE)* — aucune règle métier uniquement côté client ; bornes chargées depuis le serveur | | |
| VII | **Runes** — pas de `svelte/store` ni `$app/stores` ; `$effect` en dernier recours ; `$state.raw` pour les réponses d'API | | |
| VIII | **Simplicité** — aucune abstraction sans deuxième appelant ; stdlib avant dépendance ; aucun copier-coller depuis la v0.3 | | |
| IX | **Tranche verticale + démo cliquable** — traverse base → API → UI ; se termine par une démonstration au navigateur décrite dans `quickstart.md` (ni `curl` seul, ni « les tests passent ») | | |
| X | **Design figé** — jetons OKLCH inchangés ; composants vendorés ; Iconify CSS ; Paraglide | | |
| XI | **Aucun Proxmox requis** *(NON NÉGOCIABLE)* — tout fonctionne sous le client factice ; le choix réel/factice se fait uniquement au câblage ; zéro `if offline` ailleurs | | |
| XII | **Accessibilité** *(NON NÉGOCIABLE)* — clavier, focus visible, `lang` suivant la locale, régions live, aucun widget accessible réécrit | | |
| XIII | **Erreurs** — aucune erreur avalée ; modèle d'erreur unique ; un seul point d'entrée réseau | | |
| — | **Plafonds** — `+page.svelte` < 150 l. ; fichier < 400 l. ; fonction < 50 l. ; imbrication < 4 | | |
| — | **Tests** — logique métier testable sans HTTP ni Proxmox ; Go en tables avec `-race` ; `.svelte.ts` sans DOM ; `svelte-check` à zéro | | |
| — | **E2E** — la tranche ajoute le scénario Playwright de sa propre démonstration, exécuté contre le binaire réel + client factice | | |

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

<!--
  ACTION REQUIRED: keep only the directories this slice actually touches, with real
  paths. The layout itself is FIXED by the constitution — do not propose alternatives.
-->

```text
server/                          # Go backend (v0.4)
  cmd/pvmss/main.go
  internal/
    config/                      # loading + startup validation
    store/                       # SQLite (WAL), migrations, queries
    auth/                        # pve | oidc | local ; sessions ; API tokens
    cluster/                     # one Proxmox client per cluster
    inventory/                   # read projection (worker + /cluster/resources)
    vm/                          # Resolve() ; reads ; writes
    catalog/                     # approved nodes, storages, bridges, ISOs, tags
    policy/                      # gabarit | quota | capacité
    audit/
    httpapi/                     # routes, middleware, DTOs
  migrations/

web/                             # SvelteKit SPA (v0.4)
  src/
    routes/                      # (public) prerendered | (app) | (admin)
    lib/features/<domain>/       # vms | vm-create | vm-detail | admin | auth
    lib/shared/                  # api | ui | i18n | utils

backend/  frontend/              # v0.3 legacy — READ-ONLY reference, never edited
```

Rules that are not negotiable in this layout:

- Go splits by **domain**, never by technical layer. Frontend splits by **feature**, never
  by file type.
- No `utils.go`, no `helpers.ts`, no catch-all file. A file is named after what it does.
- A v0.4 task never edits `backend/` or `frontend/`. Those stay deployable until cutover
  and serve only as a source of ideas.

**Structure Decision**: [List the exact directories this slice creates or touches.]

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |

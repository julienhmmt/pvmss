<!--
SYNC IMPACT REPORT
==================
Version change: (template, unversioned) → 1.0.0
Bump rationale: MAJOR — initial ratification. First concrete constitution replacing
the unfilled template. All principles newly defined.

Principles defined (12):
  I.    Vocabulaire imposé
  II.   API-first
  III.  Resolve() est l'unique accès en écriture (NON NÉGOCIABLE)
  IV.   Chemins de lecture et d'écriture séparés
  V.    Multi-cluster par construction
  VI.   Le serveur est la seule frontière (NON NÉGOCIABLE)
  VII.  Svelte 5 runes uniquement
  VIII. Le plus simple qui marche
  IX.   Tranches verticales
  X.    Design figé
  XI.   Accessibilité (NON NÉGOCIABLE)
  XII.  Erreurs explicites

Sections added:
  - Contraintes techniques  (absorbe les plafonds de taille : règle source n°8)
  - Qualité et workflow     (absorbe la discipline de test : règle source n°12)

Note: les 14 règles fournies en entrée sont toutes présentes. Les règles 8 (plafonds
de taille) et 12 (tests) ont été placées en sections dédiées plutôt qu'en principes,
car le « Constitution Check » de plan-template.md consomme ces sections comme portes
de validation — elles y sont donc plus contraignantes, pas moins.

Templates requiring updates:
  ✅ .specify/templates/plan-template.md  — Constitution Check rempli (14 portes, dont 3
                                            NON NÉGOCIABLES) ; arborescence remplacée par
                                            server/ + web/ ; les 3 options génériques
                                            (single project / web app / mobile) supprimées
  ✅ .specify/templates/tasks-template.md — 2 conflits RÉELS corrigés :
                                            (a) l.25 « Web app: backend/src/, frontend/src/ »
                                                contredisait la structure server/ + web/ ;
                                            (b) « Tests … (OPTIONAL - only if tests requested) »
                                                × 3 contredisait la discipline de test
                                                obligatoire → passé en (REQUIRED)
  ✅ .specify/templates/spec-template.md  — vérifié section par section, aucun conflit
  ✅ .claude/skills/speckit-*/SKILL.md    — aucune référence périmée

Deferred TODOs: aucun.

Correction: une première rédaction de ce rapport annonçait tasks-template.md « aucun
conflit détecté » sans l'avoir vérifié. Contrôle effectué ensuite : deux conflits réels,
corrigés ci-dessus.

--------------------------------------------------------------------------------
AMENDEMENT 1.0.0 → 1.1.0 (2026-08-01)
Bump rationale: MINOR — un principe ajouté, un principe renforcé, une section étendue.

Ajouté:
  XI. Aucun Proxmox requis (NON NÉGOCIABLE) — interface cluster.Client à deux
      implémentations de production (réelle + factice), choix fait une seule fois au
      câblage. Motivé par un constat vérifié : le legacy v0.3 compte 110 occurrences de
      « Offline » dispersées dans son code (state/manager_cache.go, manager_proxmox.go…),
      anti-motif explicitement proscrit.
  Anciens XI et XII renumérotés en XII (Accessibilité) et XIII (Erreurs explicites).

Renforcé:
  IX. Tranches verticales → « démontrables à la main ». Toute tranche se termine par une
      démonstration cliquable dans un navigateur, décrite dans son quickstart.md. Une
      démonstration réduite à curl ou à une suite verte ne compte pas.

Étendu:
  « Qualité et workflow » → sous-section Tests de bout en bout : Playwright contre le
  binaire réel et le client factice, mis en place à la première tranche cliquable, chaque
  tranche ajoutant ensuite le scénario E2E de sa propre démonstration.

Templates requiring updates:
  ⚠ .specify/templates/plan-template.md — Constitution Check : ajouter la porte XI,
                                          renuméroter XI→XII et XII→XIII, référence de
                                          version v1.0.0 → v1.1.0
  ⚠ .claude/v0.4/plan/PL00-plan-de-reecriture.md — découpage en tranches à revoir
                                          (client factice et E2E dès le socle, point de
                                          démonstration par tranche)
  ✅ .specify/templates/tasks-template.md — renvois génériques à « Qualité et workflow »,
                                          restent valides
-->

# Constitution PVMSS

PVMSS (Proxmox VM Self-Service) est un portail permettant à un utilisateur de gérer ses
machines virtuelles Proxmox sans accès à l'interface Proxmox. Cette constitution encadre
la réécriture intégrale **v0.4** (backend Go, frontend SvelteKit / Svelte 5 runes /
TypeScript).

Elle encode des décisions **déjà arbitrées** et documentées dans la cartographie
`.claude/v0.4/` (93 fiches : 58 fonctions utilisateur, 17 domaines backend, 9 fiches
techniques frontend, décisions produit et architecture). Aucune tranche de la réécriture
ne rediscute ces points.

## Core Principles

### I. Vocabulaire imposé

Trois concepts distincts, jamais confondus :

- **gabarit** — taille maximale d'une VM (vCPU, mémoire, disque) ;
- **quota** — nombre de VMs autorisé par utilisateur ;
- **capacité** — agrégat de ressources par nœud.

Le mot « limite » employé seul est **BANNI** du code, des identifiants, de l'interface,
des specs et des messages d'erreur.

*Rationale : ces trois notions ont été confondues dans le code v0.3, produisant des règles
appliquées au mauvais endroit. Un vocabulaire distinct rend l'erreur visible à la lecture.*

### II. API-first

Aucun endpoint n'est réservé à l'interface web. La SPA est **un client de l'API publique
parmi d'autres**. Le navigateur s'authentifie par cookie de session, l'automatisation par
jeton Bearer — les deux attaquent la **même** surface d'API, avec les mêmes droits.

Toute fonctionnalité accessible dans l'interface DOIT être accessible par API.

*Rationale : les utilisateurs experts doivent pouvoir automatiser leur usage avec leur
propre compte et leurs propres droits.*

### III. Resolve() est l'unique accès en écriture (NON NÉGOCIABLE)

Une seule fonction, `Resolve(actor, cluster, vmid)`, donne accès à une VM en écriture.
Elle vérifie le tag `pvmss` **ET** l'appartenance au pool de l'appelant.

- Aucun handler ne résout une VM autrement.
- Aucun handler n'accepte un nœud (`node`) fourni par le client — il est toujours résolu
  côté serveur.
- Tout nouveau chemin d'écriture passe par `Resolve()` ou n'est pas fusionné.

*Rationale : correctif structurel de la faille S01. En v0.3, cinq routes vérifiaient
correctement la propriété et une l'avait oubliée, permettant à tout utilisateur
authentifié de piloter n'importe quelle VM du cluster. Un point de passage unique rend
cet oubli impossible plutôt qu'improbable.*

### IV. Chemins de lecture et d'écriture séparés

- **Lecture** : ne touche jamais Proxmox. Elle sert une projection en mémoire, alimentée
  par un worker depuis `GET /cluster/resources`.
- **Écriture** : passe toujours par Proxmox, puis invalide la projection.

*Rationale : la v0.3 interrogeait Proxmox nœud par nœud à chaque requête de liste. La
projection rend une lecture quasi gratuite et supprime la dépendance du temps de réponse
à la taille du cluster.*

### V. Multi-cluster par construction

L'identité d'une VM est l'identifiant composite `<cluster>:<vmid>`. Il est décodé en
**un seul endroit**, jamais concaténé ni analysé à la main ailleurs. Chaque cluster
Proxmox a son **propre** compte de service ; aucun compte n'est partagé entre clusters.

Le multi-cluster n'est pas une option ajoutée après coup : le modèle de données et les
signatures le portent dès la première tranche, même si une seule instance est configurée.

*Rationale : rétrofitter une identité composite dans un code qui suppose un `vmid` global
impose de toucher chaque route. Le coût est nul au départ, élevé ensuite.*

### VI. Le serveur est la seule frontière (NON NÉGOCIABLE)

La validation côté client est un **confort** : elle évite un aller-retour, jamais elle ne
garantit quoi que ce soit.

- Aucune règle métier n'existe uniquement côté client.
- Les bornes (gabarits, quotas, capacités) sont **chargées depuis le serveur**, jamais
  codées en dur dans le frontend.
- Toute entrée est validée à la frontière du système, côté serveur, avant traitement.

### VII. Svelte 5 runes uniquement

- Interdits : `svelte/store`, `$app/stores`, mode legacy, `export let`, `<slot>`,
  `on:click`, directive `class:`, `use:action`.
- `$effect` est une **trappe de secours** : préférer `load` (données de route),
  `$derived` (calculs), `<svelte:window>` / `<svelte:document>` (événements globaux),
  `{@attach}` (intégration DOM).
- L'état partagé s'écrit en **classes à champs `$state`** exposées via `createContext`,
  pas en singletons de module.
- `$state.raw` est **obligatoire** pour toute réponse d'API : ces données sont réassignées,
  jamais mutées en place.

### VIII. Le plus simple qui marche

- Aucune abstraction sans **deuxième appelant** réel.
- Aucune option de configuration que personne ne règle.
- Bibliothèque standard avant toute dépendance ; fonctionnalité native de la plateforme
  avant du code.
- Le code v0.3 fournit des **idées** (algorithmes, pièges résolus, cas de test), jamais du
  copier-coller.

*Rationale : la v0.3 contient 798 lignes d'abstraction testée sans aucun appelant, et
2 762 lignes réparties sur quatre pages pour une seule fonction. Les deux symptômes ont la
même cause.*

### IX. Tranches verticales, démontrables à la main

Chaque tranche traverse **base de données → API → interface** et se termine par quelque
chose d'utilisable. Jamais « backend d'abord, frontend ensuite ».

**Toute tranche se termine par une démonstration cliquable**, décrite dans son
`quickstart.md` : une suite de gestes que le responsable du projet peut exécuter lui-même
dans un navigateur, sans Proxmox, sans identifiants, sans données préparées à la main. Une
tranche dont la « démonstration » se réduit à `curl` ou à une suite de tests verte est
**incomplète** — elle n'a pas prouvé qu'un humain peut s'en servir.

Une tranche qui ne produit rien d'essayable est mal découpée.

### X. Design figé

- Le jeu de jetons de couleur **OKLCH** de la v0.3 est conservé tel quel, thème clair et
  sombre. Aucune couleur n'est ajoutée ni modifiée sans décision explicite.
- Les composants accessibles vendorés (shadcn / bits-ui) sont utilisés tels quels ; aucune
  feuille de style globale maison.
- Icônes via **Iconify en CSS** — jamais une bibliothèque à un fichier par icône.
- Internationalisation via **Paraglide** : clés typées, absence détectée à la compilation.

### XI. Aucun Proxmox requis (NON NÉGOCIABLE)

L'application DOIT démarrer, se naviguer et se démontrer **intégralement sans aucun
serveur Proxmox joignable**. C'est le mode par défaut en intégration continue, en
développement, et pour toute démonstration.

- L'accès à Proxmox passe par une **interface** (`cluster.Client`) qui a **deux
  implémentations de production** : le client Proxmox réel et un client **factice** servant
  un jeu de données cohérent (nœuds, VMs, stockages, pools, tâches).
- Le choix entre les deux se fait **une seule fois, au câblage**, dans `main.go`. Aucun
  autre fichier ne demande jamais « suis-je hors-ligne ? ».
- **Interdit** : `if offline { … } else { … }` dans un handler, un service ou un composant.
  Le legacy v0.3 compte **110 occurrences** de `Offline` dispersées dans son code — c'est
  précisément l'anti-motif que cette règle proscrit.
- Le client factice n'est pas un bouchon de test : c'est du code de production, tenu au
  même niveau d'exigence, versionné avec le reste, et il fait foi pour les tests E2E.
- Tout endpoint ajouté par une tranche DOIT fonctionner sous le client factice avant
  d'être considéré comme terminé.

*Rationale : sans cela, ni la CI ni une démonstration ne peuvent tourner, et chaque
tranche resterait bloquée derrière la disponibilité d'un cluster. Deux implémentations
réelles justifient pleinement l'interface au regard du principe VIII.*

### XII. Accessibilité (NON NÉGOCIABLE)

- Navigation clavier complète sur tout parcours.
- Focus toujours visible.
- L'attribut `lang` du document **suit la locale active**.
- Les changements d'état asynchrones (fin de tâche, perte de service, résultat d'action
  groupée) sont annoncés par une région live.
- `prefers-reduced-motion` respecté.
- Un `select`, un `dialog` ou un menu déroulant n'est **jamais** réécrit à la main.

### XIII. Erreurs explicites

- Aucune erreur avalée en silence, nulle part.
- Côté Go : erreurs enveloppées, sentinelles pour les cas testables.
- Côté client : un **modèle d'erreur unique**, un seul point d'entrée réseau. Aucun appel
  ne contourne le client d'API.
- Message clair pour l'utilisateur, contexte détaillé dans les journaux serveur. Un message
  d'erreur ne divulgue jamais d'information sensible.

## Contraintes techniques

**Plafonds de taille** — dépassement = revue refusée :

| Élément | Plafond |
| --- | --- |
| `+page.svelte` | 150 lignes (câblage seulement : `load` → composant de feature → rendu) |
| Tout fichier | 400 lignes |
| Toute fonction | 50 lignes |
| Imbrication | 4 niveaux |

**Structure** — le code v0.4 vit dans `server/` (Go) et `web/` (SvelteKit). Le legacy
v0.3 (`backend/`, `frontend/`) a été supprimé à la bascule T16 (commit `a7a26f7a`) ;
il ne subsiste que comme référence historique dans les *rationale* ci-dessous.

- Go : découpage par domaine (`inventory/`, `policy/`, `catalog/`, `vm/`, `auth/`,
  `cluster/`, `store/`, `httpapi/`), pas par couche technique.
- Frontend : découpage par fonctionnalité (`lib/features/<domaine>/`), pas par type de
  fichier.
- **Interdits** : `utils.go`, `helpers.ts`, et tout fichier fourre-tout. Un fichier porte le
  nom de ce qu'il fait.

**Immutabilité** — les structures de données ne sont pas mutées en place ; une modification
produit une nouvelle valeur.

**Persistance** — SQLite en mode WAL. Aucun état mutable global dans le processus.

## Qualité et workflow

**Discipline de test** :

- La logique métier DOIT être testable **sans HTTP et sans Proxmox**. Si un test exige de
  monter un serveur ou d'appeler Proxmox, la logique est au mauvais endroit.
- Go : tests en tables, exécutés avec `-race`. Les tests hors-ligne ne requièrent aucun
  Proxmox joignable.
- Frontend : les modules `.svelte.ts` (état de liste, assistants, filtres) se testent
  **sans DOM**. C'est la raison d'être de leur séparation d'avec les composants.
- `svelte-check` : **zéro erreur, zéro avertissement** en intégration continue.
- Tout correctif de sécurité arrive accompagné d'un test qui échoue avant lui.

**Tests de bout en bout (E2E)** :

- **Playwright**, exécuté contre le binaire réel servant la SPA réelle, avec le client
  Proxmox **factice** (principe XI). Aucun Proxmox, aucun identifiant réel, aucune donnée
  préparée à la main.
- La suite E2E est mise en place à la **première tranche produisant un parcours cliquable**
  et grandit ensuite d'une tranche à l'autre : **chaque tranche ajoute le scénario E2E de
  sa propre démonstration**. Une tranche qui n'ajoute aucun scénario E2E n'est pas terminée.
- Les E2E tournent en intégration continue à chaque changement. Ils sont la preuve
  exécutable de ce que la démonstration manuelle du principe IX montre à l'œil.
- Portée : les parcours réels d'un utilisateur, pas la couverture exhaustive. Les cas
  limites appartiennent aux tests unitaires, qui sont moins chers et plus rapides.

**Portes de validation** — une tranche n'est fusionnée que si :

1. les principes ci-dessus sont respectés, ou la dérogation est justifiée dans le
   « Complexity Tracking » du plan ;
2. les plafonds de taille sont tenus ;
3. la couverture de test de la logique métier est effective ;
4. les fiches de la cartographie couvertes par la tranche passent.

## Governance

Cette constitution **prévaut** sur toute autre pratique, habitude ou préférence exprimée
dans une spec, un plan ou une revue.

**Amendement** — toute modification exige : (1) la décision explicite du responsable du
projet, (2) la mise à jour de ce fichier avec incrément de version, (3) la propagation aux
templates dépendants (`plan-template.md`, `spec-template.md`, `tasks-template.md`).

**Versionnement sémantique** :

- **MAJOR** — suppression ou redéfinition incompatible d'un principe ;
- **MINOR** — ajout d'un principe ou d'une section, ou extension matérielle d'une règle ;
- **PATCH** — clarification, reformulation, correction sans effet sémantique.

**Conformité** — chaque plan de tranche DOIT franchir le « Constitution Check » avant la
phase de recherche, puis à nouveau après la conception. Toute violation assumée est
consignée dans le tableau « Complexity Tracking » du plan, avec la raison et l'alternative
plus simple écartée. Une violation non consignée bloque la fusion.

**Référence** — la cartographie `.claude/v0.4/` fait foi sur *ce que fait* l'application et
*ce qu'il ne faut pas refaire*. Elle n'est pas publiée tant que la fiche `securite/S01`
contient une preuve de concept exploitable.

**Version**: 1.1.0 | **Ratified**: 2026-08-01 | **Last Amended**: 2026-08-01

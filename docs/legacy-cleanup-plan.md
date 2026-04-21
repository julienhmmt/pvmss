# Legacy Code Cleanup Plan

**Objectif :** supprimer le sous-système session/CSRF/formulaires hérité de l'ère pré-SPA,
ne conserver que la stack JWT `api/v1/`, et corriger les règles rate-limiter qui ciblent
des routes mortes.

**Aucune dépendance `telmate`** n'existe dans le code — uniquement un commentaire orphelin
à supprimer. La dette réelle est ~2 000 lignes de handlers session-based jamais atteints
en production quand le SPA est disponible.

---

## Contexte architectural

Le mux `handlers/handlers.go` dispatche ainsi :

```
/static, /_app/*, /favicon.ico  →  publicHandler   (sans session)
/api/v1/*                        →  apiHandler      (JWT, sans session)
/api/* (hors /api/v1/)           →  appHandler      (session + CSRF ← inutile)
tout le reste (SPA dispo)        →  serveSPA        (index.html, bypass router)
tout le reste (SPA absent)       →  appHandler      (fallback legacy)
```

**Conséquence :** quand le SPA est compilé (toujours vrai en production), les routes
`/login`, `/admin/login`, `/tags`, etc. enregistrées sur le router ne sont **jamais**
atteintes — `serveSPA` les intercèpte avant. `buildAppMiddleware` (session + CSRF + HSTS
templates) ne sert en pratique que `/api/health` et `/api/health/proxmox`.

---

## Phase 1 — Quick wins, risque zéro

> Fichiers sans aucun appelant vivant. Suppression immédiate.

### Fichiers à supprimer entièrement

| Fichier | Lignes | Raison |
|---|---|---|
| `backend/security/config.go` | ~53 | `GetConfig()` jamais appelé ; `CSRFTokenTTL` défini à 3 endroits |
| `backend/handlers/sanitize.go` | ~104 | `InputSanitizer` uniquement référencé dans son propre test |
| `backend/handlers/security_test.go` | ~200 | Teste uniquement `InputSanitizer` (supprimé ci-dessus) |
| `backend/handlers/validation.go` | ~135 | `MakeValidationHelper` jamais instancié en production |
| `backend/handlers/base_handlers.go` | ~211 | `BaseFormHandler`/`BaseAPIHandler` jamais instanciés ; `api/v1/errors.go` couvre le même besoin |
| `backend/handlers/safe_errors.go` | ~25 | `RespondWithSafeError` sans appelant |

### Modifications ciblées

| Fichier | Action |
|---|---|
| `backend/proxmox/cloudinit.go:274` | Supprimer le commentaire `// Create multipart form body following telmate/proxmox-api-go approach` |
| `backend/handlers/formatting.go` | Supprimer `FormatBytes`, `FormatMemoryGB`, `BytesToGB`, `FormatUptime` (sans appelant) ; **conserver** `MBToGB` (utilisé par `limits_helpers.go`) |
| `backend/constants/security.go` | Supprimer les constantes `SessionKey*` (jamais adoptées en production) ; conserver `RateLimit*` et `CSRF*` jusqu'à la phase 3 |

**Total estimé phase 1 :** ~860 lignes supprimées, 0 comportement modifié.

---

## Phase 2 — Nettoyage routes legacy et fix sécurité rate-limiter

> Handlers session-based enregistrés sur le router mais jamais atteints quand le SPA est présent.
> Contient un **fix de sécurité** (règles rate-limiter sur des routes mortes).

### 2a — Fix rate-limiter (sécurité, priorité haute)

Les règles actuelles dans `handlers/handlers.go:31-42` protègent des routes inexistantes :

```go
// AVANT — protège des routes mortes
rateLimiter.AddRule("POST", "/login",              Rule{...})
rateLimiter.AddRule("POST", "/admin/login",        Rule{...})
rateLimiter.AddRule("POST", "/admin/proxmox-login",Rule{...})

// APRÈS — protège les vraies endpoints JWT
rateLimiter.AddRule("POST", "/api/v1/auth/login",              Rule{...})
rateLimiter.AddRule("POST", "/api/v1/auth/proxmox-admin-login",Rule{...})
```

### 2b — `handlers/auth.go` (786 lignes → ~80 lignes)

Fonctions à **supprimer** :

| Fonction | Raison |
|---|---|
| `ShowLoginForm`, `ShowAdminLoginForm` | Redirigent vers `/login` (SPA) — stub circulaire |
| `renderLoginForm`, `renderAdminLoginForm` | Idem |
| `handleLogin`, `handleAdminLogin`, `handleProxmoxAdminLogin` | Session auth remplacée par `api/v1/auth/login` |
| `LogoutGet`, `LogoutHandler` | Remplacés par `POST /api/v1/auth/logout` |
| `establishSession`, `establishSessionWithTicket` | Appelés uniquement par les handlers ci-dessus |
| `GetProxmoxTicketFromSession`, `IsProxmoxTicketValid` | Aucun appelant hors `auth.go` |
| `getRedirectURL`, `ensureLocalPath` | Support du flow de redirect mort |
| `validateCSRF` | CSRF session remplacé par middleware JWT |

Fonctions à **conserver** (nécessaires jusqu'en phase 3) :

- `MakeAuthHandler`, `RegisterRoutes` (stub minimal — enregistre seulement les GET → SPA)
- `IsAuthenticated`, `IsAdmin` (encore référencés par `auth_guard.go` jusqu'à phase 3)
- `RedirectIfAuthenticated`

### 2c — `handlers/tags.go`

Supprimer `CreateTagHandler`, `DeleteTagHandler` et leurs enregistrements `POST /tags`, `POST /tags/delete`.
**Conserver** `EnsureDefaultTag` (appelé depuis `InitHandlers`).

### 2d — Fichiers entiers à supprimer

| Fichier | Lignes | Raison |
|---|---|---|
| `backend/handlers/form_middleware.go` | ~246 | `SecureFormHandler` uniquement appelé par les tags morts (2c) |
| `backend/handlers/route_helpers.go` | ~90 | `MakeRouteHelpers`/`MakeAdminPageRoutes` sans appelant après 2c |
| `backend/handlers/handler_context.go` | ~265 | Appelé uniquement depuis les handlers morts de `auth.go` |

### 2e — `handlers/auth_guard.go`

Supprimer `RequireAuth`, `RequireAdminAuth`, `RequireAuthHandleWS`, `RequireAuthHandle`
(wrappers session-based — sans appelant après 2c/2d).
**Conserver** `IsAuthenticated`, `IsAdmin` jusqu'à phase 3.

### 2f — `handlers/common.go`

Supprimer `AdminAuditMiddleware` (jamais enregistré sur aucune route) et
`HandlerFuncToHTTPrHandle` (plus d'appelant après 2c/2d).

### 2g — `handlers/helpers.go`

Supprimer `PostOnlyHandler`, `ParseFormMiddleware`, `PostFormHandler`, `RedirectWithSuccess`,
`RedirectWithError` (sans appelant).
**Conserver** `CreateHandlerLogger` et `RenderErrorPage` (utilisés par le router 404/405).

**Total estimé phase 2 :** ~1 100 lignes supprimées.

---

## Phase 3 — Démantèlement infrastructure session

> Supprimer le store CSRF en mémoire (jamais alimenté), alléger `buildAppMiddleware`,
> et potentiellement retirer la dépendance SCS.

### 3a — `state/manager_security.go` (entier, ~58 lignes)

- Supprimer `AddCSRFToken`, `ValidateAndRemoveCSRFToken`, `CleanExpiredCSRFTokens`
  de l'interface `StateManager` (`state/interface.go:48-50`)
- Supprimer `csrfTokens map[string]time.Time` et `securityMu` de `appState`
- Supprimer la goroutine `cleanupSecurityData` dans `state/manager.go:104-111`
- Supprimer les stubs correspondants dans tous les mocks (`StateManager` test implementations)
- Supprimer `constants.CSRFCleanupInterval` de `constants/security.go`

### 3b — Rerouter `isLegacyAPIPath` → `apiHandler`

Dans `handlers/handlers.go`, remplacer :

```go
case isLegacyAPIPath(r.URL.Path):
    appHandler.ServeHTTP(w, r)
```

par :

```go
case isLegacyAPIPath(r.URL.Path):
    apiHandler.ServeHTTP(w, r)
```

`/api/health` n'a pas besoin de session ni de CSRF. Cela vide complètement `appHandler`
de tout trafic réel.

### 3c — Supprimer `buildAppMiddleware` et `sessionDebugMiddleware`

Une fois aucune route ne passant par `appHandler` :
- Supprimer `buildAppMiddleware` de `middleware_utils.go`
- Supprimer `sessionDebugMiddleware` de `middleware_utils.go`
- Supprimer l'appel `buildAppMiddleware` dans `handlers.go`
- Supprimer le bloc `appHandler` du mux switch

### 3d — Supprimer l'infrastructure session (si plus aucun appelant)

- Supprimer `NewSessionManager` et son appel dans `main.go:initializeApp`
- Supprimer `SetSessionManager`/`GetSessionManager` de `StateManager` + implémentation
- Supprimer `security/init.go` (uniquement `NewSessionManager`)
- Supprimer `security/middleware/session.go` (`SessionMiddleware`)
- Supprimer `security/csrfgen.go` et `security/session.go`
- Supprimer le package `security/middleware/` s'il devient vide
- Retirer `github.com/alexedwards/scs/v2` de `go.mod` et `go.sum`
- Retirer `securityMiddleware "pvmss/security/middleware"` des imports de `main.go`

### 3e — Nettoyer `security/csrfgen.go:isSafeMethod` (avant suppression)

Les préfixes `/css/`, `/js/`, `/webfonts/`, `/components/` dans `isSafeMethod` correspondent
à l'ancienne arborescence de templates HTML. Le SPA met tout sous `/_app/` (déjà dans
`isStaticPath`). Simplifier ou supprimer si le fichier entier est supprimé en 3d.

### 3f — Constantes orphelines dans `constants/security.go`

Après phase 3, supprimer `CSRFTokenLength`, `CSRFTokenTTL` (dupliqués avec les constantes
du package `security/` qui sera supprimé), et le fichier entier si vide.

**Total estimé phase 3 :** ~200 lignes + retrait dépendance `alexedwards/scs/v2`.

---

## Phase 4 — Décision `limits_helpers.go`

`ValidateVMResourcesAgainstNodeLimits` et `CalculateNodeResourceUsage` existent dans
`handlers/limits_helpers.go` mais **ne sont pas appelées** depuis `api/v1/vm_create.go`.
Elles étaient branchées sur le vieux flow de création VM par formulaire.

Deux options, à trancher selon la roadmap produit :

### Option A — Brancher dans l'API (enforcement voulu)

1. Déplacer `ValidateVMResourcesAgainstNodeLimits` et `CalculateNodeResourceUsage`
   dans un package partagé (ex. `internal/limits/`) ou directement dans `api/v1/vm_create.go`
2. Appeler depuis `api/v1/vm_create.go:CreateVM` avant la création
3. Supprimer `returnLocalizedError` (remplacer par `fmt.Errorf`)
4. Encapsuler `nodeUsageCache`, `nodeUsageCacheMu`, etc. dans une struct (package-level
   mutable state — rend les tests non-déterministes)

### Option B — Supprimer (enforcement non requis / côté frontend)

1. Supprimer `handlers/limits_helpers.go` entier
2. Déplacer `MBToGB` dans `utils/` (utilisé par `api/v1/admin_mutations.go`)
3. Les limites par utilisateur restent gérées côté DB/SPA

---

## Phase 5 — Résidus finaux

| Action | Fichier | Détail |
|---|---|---|
| Supprimer `GET /api/health`, `GET /api/health/proxmox` | `handlers/health.go` | Doublon de `/api/v1/health` ; mettre à jour `tests/integration_test.go` |
| Supprimer `NotFoundHandler`, `MethodNotAllowedHandler` | `handlers/health.go` | Définis mais jamais assignés (le router utilise des lambdas inline dans `handlers.go:70-84`) |
| Supprimer `sendJSONResponse` | `handlers/health.go` | Sans appelant après nettoyage |
| Supprimer `handlers/settings.go` | `handlers/settings.go` | `GetSettingsHandler`, `GetAllVMBRsHandler`, `GetAllSettingsHandler` jamais enregistrés sur le router |
| Supprimer `handlers/resty_helper.go` | `handlers/resty_helper.go` | Wrapper mince autour de `proxmox.MakeRestyClientFromEnv` — appeler directement |
| Supprimer `isLegacyAPIPath` + sa branche dans le mux | `handlers/middleware_utils.go`, `handlers/handlers.go` | Inutile après suppression des routes `/api/health` |
| Supprimer `RenderErrorPageWithI18n` | `handlers/middleware_utils.go` | Wrapper autour de `RenderErrorPage` avec paramètre i18n ignoré ; inliner l'appel dans `recoverMiddleware` |
| Supprimer `handlers/errors.go:LocalizeError`, `LocalizeErrorWithFallback` | `handlers/errors.go` | Stubs i18n qui retournent la clé telle quelle ; remplacer les deux sites d'appel par `fmt.Errorf` |

---

## Améliorations Go best practices à faire en passant

| Problème | Fichier | Correction |
|---|---|---|
| Package-level mutable state avec mutex non encapsulé | `handlers/limits_helpers.go:28-33` (`nodeUsageCache`, `nodeUsageCacheMu`) | Encapsuler dans une struct avec méthodes, ou injecter dans `StateManager` |
| Mauvais nom de constante pour un ticker non-CSRF | `state/manager.go` (`cleanupGuestAgentCache` utilise `constants.CSRFCleanupInterval`) | Créer `GuestAgentCacheCleanupInterval` ou utiliser une constante locale |
| Log de session token en DEBUG sur chaque requête | `handlers/auth_guard.go:IsAuthenticated` | Supprimer ou gater derrière `os.Getenv("PVMSS_DEBUG_AUTH")` |
| Trois définitions parallèles de `CSRFTokenLength`/`CSRFTokenTTL` | `security/csrfgen.go`, `constants/security.go`, `security/config.go` | Une seule source après phases 1-3 |
| `RenderErrorPageWithI18n` dans `recoverMiddleware` retourne HTML sur routes JSON `/api/v1/` | `handlers/middleware_utils.go:recoverMiddleware` | Détecter `isAPIPath` dans `recoverMiddleware` et retourner JSON ; sinon HTML |

---

## Résumé des suppressions

| Phase | Fichiers supprimés | Fichiers modifiés | Lignes ~supprimées |
|---|---|---|---|
| 1 | 6 | 3 | ~860 |
| 2 | 3 | 5 | ~1 100 |
| 3 | 4-7 | 4 | ~200 + 1 dép. go.mod |
| 4 | 0-1 | 1-2 | ~200 |
| 5 | 3 | 3 | ~150 |
| **Total** | **~17** | **~15** | **~2 500** |

---

## Ordre recommandé

```
Phase 1  →  Phase 2a (rate-limiter, fix sécurité)  →  Phase 2b-g
         →  Phase 3a-c (rerouting + buildAppMiddleware)
         →  Phase 3d-f (session infra, go.mod)
         →  Phase 4 (décision produit)
         →  Phase 5 (résidus)
```

Chaque phase compile et passe `make test-offline` avant de passer à la suivante.

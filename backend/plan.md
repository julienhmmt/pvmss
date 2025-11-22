# 0. Objectif du plan

But : que le backend Proxmox ne dépende plus de `github.com/Telmate/proxmox-api-go`, en s’appuyant uniquement sur :

- [RestyClient](/pvmss/backend/proxmox/client.go:355:0-359:1) + fonctions `*Resty` déjà en place  
- éventuellement un petit client HTTP pour les flux basés sur cookies (tickets, password, VNC)

Le tout **sans casser** :

- profils utilisateurs / pools / ACL  
- console noVNC  
- changement de mot de passe PVE  
- recherche avancée (tags, pool)  
- limites / monitoring / offline mode

---

## 1. Vision globale de la migration

Je te propose une migration en **4 phases** :

1. **Nettoyer l’usage des fonctions `*WithContext`** (VMs, nodes, configs) au profit des `*Resty`.
2. **Réécrire les flux “admin/security”** (ticket, users, pools, ACL, password) en Resty/HTTP.
3. **Supprimer la dépendance structurelle au [ClientInterface](/pvmss/backend/proxmox/interfaces.go:9:0-24:1) Telmate** dans `state` + handlers.
4. **Retirer complètement Telmate** ([Client](/pvmss/backend/proxmox/client.go:21:0-33:1), import, entrée [go.mod](/pvmss/backend/go.mod:0:0-0:0)) + simplifier l’API Proxmox.

Chaque phase peut faire 1–N PRs, mais l’ordre doit rester globalement le même.

---

## 2. Phase 1 – Supprimer les usages “data” de Telmate

### 2.1. Cible

Remplacer partout :

- [GetVMConfigWithContext](/pvmss/backend/proxmox/vms.go:23:0-37:1), [GetVMCurrentWithContext](/pvmss/backend/proxmox/vms.go:272:0-281:1)
- [GetNodeDetailsWithContext](/pvmss/backend/proxmox/nodes.go:55:0-113:1), [GetNodeNamesWithContext](/pvmss/backend/proxmox/nodes.go:131:0-150:1)
- [UpdateVMConfigWithContext](/pvmss/backend/proxmox/vms.go:39:0-61:1)
- [VMActionWithContext](/pvmss/backend/proxmox/vms.go:309:0-339:1), [DeleteVMWithContext](/pvmss/backend/proxmox/vms.go:341:0-362:1)
- Tout appel direct à [client.GetJSON(...)](/pvmss/backend/proxmox/client.go:140:0-150:1) sur des endpoints où on a déjà un équivalent `*Resty`.

### 2.2. Fichiers concernés

- `backend/handlers` :
  - [vm_actions.go](/pvmss/backend/handlers/vm_actions.go:0:0-0:0) (déjà mixte : desc/tags en Telmate, reste en Resty)
  - [vm_details.go](/pvmss/backend/handlers/vm_details.go:0:0-0:0) (guest-agent, refresh cache nodes)
  - [vm_delete.go](/pvmss/backend/handlers/vm_delete.go:0:0-0:0) (encore Telmate pour cache invalidation, pas pour les actions)
  - [search.go](/pvmss/backend/handlers/search.go:0:0-0:0) (beaucoup de [GetVMConfigWithContext](/pvmss/backend/proxmox/vms.go:23:0-37:1))
  - [profile.go](/pvmss/backend/handlers/profile.go:0:0-0:0) (pool, nodes, mais partie VMs déjà Resty)
  - [storage.go](/pvmss/backend/proxmox/storage.go:0:0-0:0) (fallback live quand snapshot absent → [GetNodeNames](/pvmss/backend/proxmox/nodes.go:115:0-121:1))
  - [settings_limits.go](/pvmss/backend/handlers/settings_limits.go:0:0-0:0) (GetNodeNamesWithContext dans Limits)
- [backend/proxmox](/pvmss/backend/Users/jh/git/pvmss/backend/proxmox:0:0-0:0) :
  - [vms.go](/pvmss/backend/proxmox/vms.go:0:0-0:0), [nodes.go](/pvmss/backend/proxmox/nodes.go:0:0-0:0) (parties `*WithContext` encore utilisées)

### 2.3. Plan concret

1. **VM configs & status**  
   - Dans les handlers qui utilisent [GetVMConfigWithContext](/pvmss/backend/proxmox/vms.go:23:0-37:1) / [GetVMCurrentWithContext](/pvmss/backend/proxmox/vms.go:272:0-281:1), remplacer par :
     - [GetVMConfigResty(ctx, restyClient, node, vmid)](/pvmss/backend/proxmox/vms.go:410:0-427:1)  
     - [GetVMCurrentResty(ctx, restyClient, node, vmid)](/pvmss/backend/proxmox/vms.go:429:0-441:1)
   - Si un handler n’a pas encore de `restyClient`, utiliser `getDefaultRestyClient()`.

2. **Node lists & details**  
   - Remplacer [GetNodeNamesWithContext(ctx, client)](/pvmss/backend/proxmox/nodes.go:131:0-150:1) par :
     - soit [GetNodeNamesResty(ctx, restyClient)](/pvmss/backend/proxmox/nodes.go:175:0-196:1),
     - soit, si le code n’a besoin que de “test de connexion”, une fonction dédiée type `CheckConnectionResty(...)`.
   - Pour les usages dans [settings_limits.go](/pvmss/backend/handlers/settings_limits.go:0:0-0:0), [storage.go](/pvmss/backend/proxmox/storage.go:0:0-0:0), [vm_details.go](/pvmss/backend/handlers/vm_details.go:0:0-0:0) → créer/partager un helper qui renvoie une liste de nodes via Resty.

3. **Update config / actions / delete**  
   - Assurer que tous les write-paths VM (actions, mise à jour config, delete) utilisent uniquement :
     - [UpdateVMConfigResty](/pvmss/backend/proxmox/vms.go:443:0-465:1)
     - [VMActionResty](/pvmss/backend/proxmox/vms.go:467:0-501:1)
     - [DeleteVMResty](/pvmss/backend/proxmox/vms.go:503:0-517:1)
   - Supprimer ensuite les fonctions [VMActionWithContext](/pvmss/backend/proxmox/vms.go:309:0-339:1) / [DeleteVMWithContext](/pvmss/backend/proxmox/vms.go:341:0-362:1) une fois plus utilisées.

4. **Cache invalidation**  
   - Là où tu appelles aujourd’hui [client.InvalidateCache("/nodes/...")](/pvmss/backend/proxmox/client.go:245:0-251:1), décider :
     - soit de supprimer ces invalidations (les `*Resty` ne s’appuient pas sur le cache Telmate),
     - soit de les remplacer par une future couche de cache Resty (facultatif).  
   - Dans un premier temps, **tu peux simplement garder l’invalidation mais la rendre no-op** (voir phase 3).

### 2.4. Commentaires à ajouter dans le code

Les commentaires suivants doivent être ajoutés en anglais, juste au-dessus des blocs ciblés.

- **backend/proxmox/vms.go**
  - `GetVMConfigWithContext`, `GetVMCurrentWithContext`, `UpdateVMConfigWithContext` : ajouter  
    `// TODO Telmate migration: replace this Telmate-based VM helper by the corresponding Resty helper (GetVMConfigResty / GetVMCurrentResty / UpdateVMConfigResty) and drop the ClientInterface dependency.`
  - `VMActionWithContext`, `DeleteVMWithContext` : ajouter  
    `// TODO Telmate migration: replace this Telmate-based VM action by VMActionResty / DeleteVMResty and remove the Telmate client usage.`
- **backend/proxmox/nodes.go**
  - `GetNodeDetailsWithContext`, `GetNodeNamesWithContext` : ajouter  
    `// TODO Telmate migration: migrate this Telmate-based node helper to the Resty node helpers (GetNodeDetailsResty / GetNodeNamesResty) and remove the ClientInterface dependency.`
- **backend/proxmox/client.go**
  - helper `GetJSON` utilisé pour les appels Telmate directs : ajouter  
    `// TODO Telmate migration: this generic Telmate-based JSON helper should be replaced by Resty-based helpers; callers must stop depending on the Telmate client.`
  - `InvalidateCache` / `CleanExpiredCache` : ajouter  
    `// TODO Telmate migration: this cache layer is tied to the Telmate client; once all API calls use Resty, remove this cache or reimplement it on top of RestyClient.`
- **backend/handlers/vm_actions.go**
  - sur le handler qui met à jour description/tags avec Telmate : ajouter  
    `// TODO Telmate migration: this handler still uses Telmate-based VM config helpers (for description/tags). Replace them with UpdateVMConfigResty and remove the Telmate client usage.`
- **backend/handlers/vm_details.go**
  - sur le handler principal de détails VM : ajouter  
    `// TODO Telmate migration: this handler still relies on Telmate-based helpers (guest agent data, cache invalidation). Replace them with Resty-based helpers and drop the Telmate cache.`
- **backend/handlers/vm_delete.go**
  - sur le handler de suppression de VM : ajouter  
    `// TODO Telmate migration: this handler still uses the Telmate client for cache invalidation; remove this dependency or replace it with a Resty-based cache strategy.`
- **backend/handlers/search.go**
  - sur le handler de recherche optimisée : ajouter  
    `// TODO Telmate migration: this handler still calls GetVMConfigWithContext to evaluate filters; switch to GetVMConfigResty and remove the Telmate ClientInterface.`
- **backend/handlers/profile.go**
  - sur le handler de profil utilisateur : ajouter  
    `// TODO Telmate migration: this handler still uses the Telmate client for pool and node access; migrate these paths to the Resty helpers and remove the Telmate dependency.`
- **backend/proxmox/storage.go**
  - sur le fallback live quand le snapshot manque : ajouter  
    `// TODO Telmate migration: this fallback still uses Telmate-based node listing; replace it with a Resty-based node listing helper.`
- **backend/handlers/settings_limits.go**
  - sur le handler de mise à jour des limites : ajouter  
    `// TODO Telmate migration: this handler uses Telmate-based node helpers to validate limits; migrate it to the Resty node helpers and drop the ClientInterface usage.`

---

## 3. Phase 2 – Réécrire les flux “auth / admin” en Resty/HTTP

Ce sont les usages les plus “profonds” de Telmate, mais bien localisés.

### 3.1. Ticket / login / password

- Fichiers :
  - [proxmox/access.go](/pvmss/backend/proxmox/access.go:0:0-0:0) : [CreateTicket](/pvmss/backend/proxmox/access.go:34:0-108:1), [UpdateUserPassword](/pvmss/backend/proxmox/access.go:156:0-199:1)
  - [handlers/profile.go](/pvmss/backend/handlers/profile.go:0:0-0:0) : [UpdatePassword](/pvmss/backend/handlers/profile.go:346:0-452:1)
  - `handlers/auth.go` (si login Proxmox y passe encore)

#### Plan

1. Créer un **client HTTP orienté cookies** :

   - Option A : étendre [RestyClient](/pvmss/backend/proxmox/client.go:355:0-359:1) :
     - ajouter des méthodes `SetCookieAuth(PVEAuthCookie, CSRF)` et `PostFormWithCookies(...)`.
   - Option B : ajouter un petit `SessionClient` basé sur `net/http` pour les endpoints `/access/...`, `/nodes/...` qui nécessitent cookies.

2. Réécrire dans [access.go](/pvmss/backend/proxmox/access.go:0:0-0:0) :

   - [CreateTicket(ctx, client, ...)](/pvmss/backend/proxmox/access.go:34:0-108:1) → utiliser Resty/HTTP directement :
     - `POST /access/ticket` avec `username/password/realm` dans `url.Values`.
     - parser la réponse JSON en [TicketResponse](/pvmss/backend/proxmox/access.go:14:0-20:1) (déjà défini).
   - [UpdateUserPassword](/pvmss/backend/proxmox/access.go:156:0-199:1) → `PUT /access/password` en Resty :
     - `SetFormDataFromValues` + headers `CSRFPreventionToken` & cookie `PVEAuthCookie`.

3. Adapter `profile.go / UpdatePassword` :

   - Remplacer [NewClientCookieAuth](/pvmss/backend/proxmox/client.go:55:0-61:1) + [CreateTicket](/pvmss/backend/proxmox/access.go:34:0-108:1) (Telmate) par une version basée sur ton nouveau client Resty/HTTP.
   - Stocker ticket + CSRF dans la session comme aujourd’hui.

### 3.2. Users / pools / roles / ACL

- Fichier : [proxmox/access.go](/pvmss/backend/proxmox/access.go:0:0-0:0) ([EnsureUser](/pvmss/backend/proxmox/access.go:110:0-154:1), [EnsurePool](/pvmss/backend/proxmox/access.go:201:0-234:1), [EnsureRole](/pvmss/backend/proxmox/access.go:261:0-296:1), [EnsurePoolACL](/pvmss/backend/proxmox/access.go:236:0-259:1)).
- Fichiers handlers : [user_pool.go](/pvmss/backend/handlers/user_pool.go:0:0-0:0).

#### Plan users / pools / roles / ACL

1. Réécrire [EnsureUser](/pvmss/backend/proxmox/access.go:110:0-154:1), [EnsurePool](/pvmss/backend/proxmox/access.go:201:0-234:1), [EnsureRole](/pvmss/backend/proxmox/access.go:261:0-296:1), [EnsurePoolACL](/pvmss/backend/proxmox/access.go:236:0-259:1) pour qu’ils :
   - prennent soit un `*RestyClient`, soit une nouvelle `AdminAPI` abstraction,
   - envoient `POST/PUT` avec `url.Values` via [RestyClient.Post(...)](/pvmss/backend/proxmox/client.go:445:0-463:1).
2. Ajuster [user_pool.go](/pvmss/backend/handlers/user_pool.go:0:0-0:0) :
   - remplacer les `client` de type [ClientInterface](/pvmss/backend/proxmox/interfaces.go:9:0-24:1) par `restyClient` obtenu via `getDefaultRestyClient()` ou un wrapper `getAdminRestyClient()`.

### 3.3. VNC / console

- Fichiers :
  - [proxmox/vnc.go](/pvmss/backend/proxmox/vnc.go:0:0-0:0) (`GetVNCProxy`)
  - `handlers/vm_console_helpers.go` (crée client cookie et appelle `GetVNCProxy`)

#### Plan VNC

1. Réécrire `GetVNCProxy` pour qu’il fonctionne soit directement sur [RestyClient](/pvmss/backend/proxmox/client.go:355:0-359:1) supportant les cookies, soit sur un mini HTTP client.
2. Adapter `GetVNCProxyTicket` pour **ne plus créer de [NewClientCookieAuth](/pvmss/backend/proxmox/client.go:55:0-61:1) Telmate** :
   - utiliser directement ton client cookie Resty/HTTP avec le ticket stocké en session.

### 3.4. Commentaires à ajouter dans le code

Les commentaires suivants doivent être ajoutés en anglais, juste au-dessus des blocs ciblés.

- **backend/proxmox/access.go**
  - `CreateTicket` : ajouter  
    `// TODO Telmate migration: implement this ticket creation using the Resty client and cookie-based authentication instead of the Telmate ClientInterface.`
  - `UpdateUserPassword` : ajouter  
    `// TODO Telmate migration: implement this password update using Resty (PUT /access/password) with CSRFPreventionToken and PVEAuthCookie instead of the Telmate client.`
  - `EnsureUser`, `EnsurePool`, `EnsureRole`, `EnsurePoolACL` : ajouter  
    `// TODO Telmate migration: replace this Telmate-based admin helper by a Resty-based implementation calling the corresponding /access and /pools endpoints.`
- **backend/handlers/profile.go**
  - sur `UpdatePassword` : ajouter  
    `// TODO Telmate migration: this password change flow still depends on Telmate ticket creation; migrate it to the Resty-based access helpers.`
- **backend/handlers/user_pool.go**
  - sur les handlers qui créent ou mettent à jour les user pools : ajouter  
    `// TODO Telmate migration: this handler still relies on Telmate-based EnsureUser/EnsurePool/EnsurePoolACL; switch to the Resty-based admin helpers and remove the ClientInterface.`
- **backend/proxmox/vnc.go**
  - `GetVNCProxy` : ajouter  
    `// TODO Telmate migration: build the VNC proxy ticket using the Resty client with cookie authentication instead of the Telmate ClientInterface.`
- **backend/handlers/vm_console_helpers.go**
  - sur `GetVNCProxyTicket` : ajouter  
    `// TODO Telmate migration: stop building a Telmate cookie client here; use the Resty-based VNC proxy helper instead.`

---

## 4. Phase 3 – Enlever [ClientInterface](/pvmss/backend/proxmox/interfaces.go:9:0-24:1) Telmate du `StateManager`

Une fois que :

- plus aucun handler n’utilise [GetVMConfigWithContext](/pvmss/backend/proxmox/vms.go:23:0-37:1), [GetNodeNamesWithContext](/pvmss/backend/proxmox/nodes.go:131:0-150:1), etc.,
- les fonctions d’[access.go](/pvmss/backend/proxmox/access.go:0:0-0:0) et [vnc.go](/pvmss/backend/proxmox/vnc.go:0:0-0:0) ont été réécrites sur Resty,

tu peux commencer à retirer vraiment Telmate du state.

### 4.1. Simplifier l’interface `StateManager`

- Dans [backend/state/interface.go](/pvmss/backend/state/interface.go:0:0-0:0) :
  - Remplacer :

    ```go
    GetProxmoxClient() proxmox.ClientInterface
    SetProxmoxClient(pc proxmox.ClientInterface) error
    ```

    par quelque chose de plus conceptuel, par exemple :

    ```go
    IsOfflineMode() bool
    GetProxmoxStatus() (bool, string)
    ```

    (que tu as déjà) et, si besoin, un accès à un **provider** Resty :

    ```go
    GetProxmoxAPI() proxmox.API // nouvelle interface plus haut niveau, optionnel
    ```

  - L’idée est que **les handlers n’ont plus besoin d’un client bas niveau** mais d’**opérations métier** (GetVMs, GetNodeNames, etc.), aujourd’hui exposées sous forme de fonctions utilitaires dans [proxmox](/pvmss/backend/Users/jh/git/pvmss/backend/proxmox:0:0-0:0).

### 4.2. Adapter [appState](/pvmss/backend/state/manager.go:19:0-64:1) et [initializeApp](/pvmss/backend/main.go:107:0-161:1)

- Dans [state/manager.go](/pvmss/backend/state/manager.go:0:0-0:0) :
  - Retirer le champ [proxmoxClient proxmox.ClientInterface](/pvmss/backend/proxmox/interfaces.go:9:0-24:1).
  - Remplacer [SetProxmoxClient](/pvmss/backend/state/manager.go:331:0-349:1) / [GetProxmoxClient](/pvmss/backend/state/manager.go:324:0-329:1) par :
    - soit rien (si chaque handler se crée son [RestyClient](/pvmss/backend/proxmox/client.go:355:0-359:1) depuis les envs),
    - soit un stockage d’une simple config `ProxmoxConfig` (URL, token, verifySSL) utilisée par [NewRestyClientFromEnv](/pvmss/backend/proxmox/helpers.go:8:0-21:1).

- Dans [main.go](/pvmss/backend/main.go:0:0-0:0) :
  - [initProxmoxClient()](/pvmss/backend/main.go:163:0-206:1) :
    - soit disparaît, soit ne fait plus que **valider** la config Proxmox (ping via Resty, par exemple).
  - [initializeApp](/pvmss/backend/main.go:107:0-161:1) :
    - n’a plus besoin de passer un client concret au `StateManager`, seulement de mettre à jour le statut Proxmox ([CheckProxmoxConnection](/pvmss/backend/state/manager.go:382:0-425:1) pourra utiliser Resty et les envs).

### 4.3. Remplacer les usages restants de [GetProxmoxClient](/pvmss/backend/state/manager.go:324:0-329:1)

- Fichiers à passer en revue (grep déjà fait) :
  - [profile.go](/pvmss/backend/handlers/profile.go:0:0-0:0), [user_pool.go](/pvmss/backend/handlers/user_pool.go:0:0-0:0), [vm_actions.go](/pvmss/backend/handlers/vm_actions.go:0:0-0:0), [vm_details.go](/pvmss/backend/handlers/vm_details.go:0:0-0:0), [vm_create.go](/pvmss/backend/handlers/vm_create.go:0:0-0:0), [vm_delete.go](/pvmss/backend/handlers/vm_delete.go:0:0-0:0), [settings_limits.go](/pvmss/backend/handlers/settings_limits.go:0:0-0:0), [storage.go](/pvmss/backend/proxmox/storage.go:0:0-0:0), [admin.go](/pvmss/backend/handlers/admin.go:0:0-0:0), [admin_vms.go](/pvmss/backend/handlers/admin_vms.go:0:0-0:0), [search.go](/pvmss/backend/handlers/search.go:0:0-0:0), [vmbr.go](/pvmss/backend/proxmox/vmbr.go:0:0-0:0), [network_helpers.go](/pvmss/backend/handlers/network_helpers.go:0:0-0:0), [validation.go](/pvmss/backend/handlers/validation.go:0:0-0:0)…

Pour chacun :

- Si [GetProxmoxClient()](/pvmss/backend/state/manager.go:324:0-329:1) n’était utilisé que pour savoir s’il y a une connexion → utiliser [GetProxmoxStatus()](/pvmss/backend/state/manager.go:375:0-380:1) + [IsOfflineMode()](/pvmss/backend/state/manager.go:368:0-373:1).
- Si c’était pour faire des appels `/pools`, `/access`, `/nodes` → remplacer par :
  - `restyClient, err := getDefaultRestyClient()`
  - - appels `*Resty` ou nouvelles fonctions `access_rest y.go`.

### 4.4. Commentaires à ajouter dans le code

Les commentaires suivants doivent être ajoutés en anglais, juste au-dessus des blocs ciblés.

- **backend/state/interface.go**
  - sur `GetProxmoxClient` et `SetProxmoxClient` : ajouter  
    `// TODO Telmate migration: this interface exposes the Telmate ClientInterface; replace it by higher-level Proxmox status or API helpers once the migration to Resty is done.`
- **backend/state/manager.go**
  - sur le champ `proxmoxClient proxmox.ClientInterface` dans `appState` : ajouter  
    `// TODO Telmate migration: this field stores the Telmate ClientInterface; remove it once all handlers use Resty-based helpers.`
  - sur `GetProxmoxClient` et `SetProxmoxClient` : ajouter  
    `// TODO Telmate migration: this getter/setter is only needed for the Telmate ClientInterface; delete them after the migration and rely on Resty-based helpers instead.`
  - sur `CheckProxmoxConnection` : ajouter  
    `// TODO Telmate migration: reimplement this Proxmox health check using only Resty-based helpers instead of the Telmate client.`
- **backend/main.go**
  - sur `initProxmoxClient` : ajouter  
    `// TODO Telmate migration: this bootstrap builds a Telmate-based client; replace it by a simple Resty-based health check and configuration validation.`
  - sur l’appel à `initProxmoxClient` dans `initializeApp` : ajouter  
    `// TODO Telmate migration: stop passing a Telmate ClientInterface into the state manager; use Resty-based Proxmox status instead.`

---

## 5. Phase 4 – Retirer Telmate et nettoyer

Quand :

- plus aucune référence à [proxmox.Client](/pvmss/backend/proxmox/client.go:21:0-33:1) / [ClientInterface](/pvmss/backend/proxmox/interfaces.go:9:0-24:1) / [NewClient](/pvmss/backend/proxmox/client.go:38:0-53:1) / [NewClientCookieAuth](/pvmss/backend/proxmox/client.go:55:0-61:1) n’existe,
- tous les [GetVMConfigWithContext](/pvmss/backend/proxmox/vms.go:23:0-37:1), [GetNodeNamesWithContext](/pvmss/backend/proxmox/nodes.go:131:0-150:1) & co sont supprimés / non utilisés,

alors :

1. **Supprimer le code Telmate-wrapper** :

   - Dans [proxmox/client.go](/pvmss/backend/proxmox/client.go:0:0-0:0) :
     - retirer l’import `px "github.com/Telmate/proxmox-api-go/proxmox"`,
     - supprimer le type [Client](/pvmss/backend/proxmox/client.go:21:0-33:1) et les fonctions associées ([NewClient](/pvmss/backend/proxmox/client.go:38:0-53:1), [NewClientCookieAuth](/pvmss/backend/proxmox/client.go:55:0-61:1), [GetRawWithContext](/pvmss/backend/proxmox/client.go:152:0-177:1), etc.),
     - ne garder que [RestyClient](/pvmss/backend/proxmox/client.go:355:0-359:1) et les helpers HTTP.

   - Dans [proxmox/interfaces.go](/pvmss/backend/proxmox/interfaces.go:0:0-0:0) :
     - soit supprimer [ClientInterface](/pvmss/backend/proxmox/interfaces.go:9:0-24:1),
     - soit le re-définir pour être implémenté par une version Resty si tu veux conserver une abstraction.

2. **Nettoyer les fichiers proxmox** :

   - Supprimer les fonctions `*WithContext` devenues mortes.
   - Renommer éventuellement les fichiers :
     - par ex. [vms.go](/pvmss/backend/proxmox/vms.go:0:0-0:0) ne contenant que les versions `*Resty`, ou scinder si nécessaire.

3. **Mise à jour [go.mod](/pvmss/backend/go.mod:0:0-0:0) / [go.sum](/pvmss/backend/go.sum:0:0-0:0)** :

   - Supprimer la ligne :

     ```go
     github.com/Telmate/proxmox-api-go ...
     ```

   - Lancer `go mod tidy` (en local / via Makefile) pour nettoyer [go.sum](/pvmss/backend/go.sum:0:0-0:0).

4. **Re-run QA** :

   - `go build ./backend/...`
   - tests unitaires / intégration existants (`make test-all` si dispo),
   - `gosec`, `golangci-lint` si tu les utilises encore.

---

## 6. Stratégie de validation

Pour chaque phase, prévoir des tests manuels/automatisés sur les features qui bougent :

- **Phase 1** :
  - VM details (description, tags, ressources),
  - search (tag / nom / VMID),
  - admin storage / limits.

- **Phase 2** :
  - login utilisateur (si couplé à Proxmox),
  - changement de mot de passe dans `/profile`,
  - création / suppression de user pools,
  - accès console noVNC.

- **Phase 3–4** :
  - démarrage complet de l’app avec et sans Proxmox,
  - offline mode (`PVMSS_OFFLINE=true`),
  - pages admin (nodes, appinfo) et leurs indicateurs de statut Proxmox.

---

### 7. Résumé

- La migration est faisable **sans gros redesign** en 4 grandes phases :
  1. Basculer tout ce qui est “data” VM/nodes vers `*Resty`.
  2. Réécrire les flux auth/admin (`/access`, `/pools`, `/roles`, VNC) sur Resty/HTTP.
  3. Retirer le [ClientInterface](/pvmss/backend/proxmox/interfaces.go:9:0-24:1) Telmate du `StateManager` + handlers.
  4. Supprimer la dépendance Telmate (code + go.mod) et ne garder qu’un client Resty + helpers HTTP.

Si tu veux, on peut attaquer ensuite **phase 1** ensemble, fichier par fichier, en commençant par les handlers les plus simples (ex. [vm_actions.go](/pvmss/backend/handlers/vm_actions.go:0:0-0:0) / [search.go](/pvmss/backend/handlers/search.go:0:0-0:0)) pour limiter le risque.

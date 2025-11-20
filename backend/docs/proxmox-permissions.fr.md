# Proxmox – Permissions & API utilisées par PVMSS

## 1. Objectifs du document

- **But principal**
  - Décrire précisément quelles API Proxmox PVMSS utilise.
  - Lier ces API aux **privilèges Proxmox** nécessaires.
  - Définir des **rôles recommandés** (`PVMSS_Service`, `PVMSS_Admin`, `PVMSS_User`, etc.).
- **Public cible**
  - Administrateurs Proxmox qui doivent configurer les droits pour PVMSS.
  - LLM/agents qui doivent répondre à des questions du type « Est-ce que tel rôle peut faire X ? ».
- **Sources de vérité**
  - Code PVMSS, notamment :
    - `backend/proxmox/*.go` (accès API Proxmox)
    - `backend/handlers/*.go` (handlers métier : création VM, actions, console, pools, utilisateurs, etc.)
  - Documentation Proxmox VE :
    - User Management : [https://pve.proxmox.com/pve-docs/chapter-pveum.html](https://pve.proxmox.com/pve-docs/chapter-pveum.html)
    - Privileges & Roles : [https://pve.proxmox.com/wiki/User_Management](https://pve.proxmox.com/wiki/User_Management)
    - Proxmox API Viewer : [https://pve.proxmox.com/pve-docs/api-viewer/](https://pve.proxmox.com/pve-docs/api-viewer/)

> Les noms d’API et de privilèges sont alignés sur l’API Viewer Proxmox, mais la liste exacte de privilèges minimaux doit toujours être validée dans la doc Proxmox, car certaines combinaisons peuvent dépendre de la version de PVE. On utilise Proxmox 9.1.

---

## 2. Acteurs et rôles côté Proxmox

### 2.1 Compte de service PVMSS (`PVMSS_Service`)

- Utilisé par le backend PVMSS via **API Token** (variables `PROXMOX_API_TOKEN_NAME` / `PROXMOX_API_TOKEN_VALUE`).
- Sert à :
  - Lister et monitorer **cluster, nœuds, VMs, stockages**.
  - Créer / modifier / supprimer des **VMs**.
  - Lire les **bridges réseau** (vmbr et sdn) et les **ISOs**.
  - Gérer **utilisateurs et les pools** côté Proxmox.

Ce compte est sensible, qui a des droits élevés sur l'ensemble du cluster. Il est recommandé de limiter son utilisation à des opérations automatisées uniquement et de ne pas l'utiliser pour des tâches manuelles.

```bash
pveum roleadd PVMSS_Service -privs "Sys.Audit VM.Audit VM.Allocate VM.PowerMgmt VM.Console VM.Config.CPU VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.Options VM.Config.Cloudinit Datastore.Audit Datastore.AllocateSpace Pool.Allocate User.Modify Permissions.Modify Realm.AllocateUser"

pveum useradd pvmss-svc@pve \
  -comment "PVMSS service account" \
  -enable 1

pveum user token add pvmss-svc@pve pvmss-service-token --privsep 0
┌──────────────┬──────────────────────────────────────┐
│ key          │ value                                │
╞══════════════╪══════════════════════════════════════╡
│ full-tokenid │ pvmss-svc@pve!pvmss-service-token    │
├──────────────┼──────────────────────────────────────┤
│ info         │ {"privsep":"0"}                      │
├──────────────┼──────────────────────────────────────┤
│ value        │ secret_value_to_store_in_env         │
└──────────────┴──────────────────────────────────────┘
```

### 2.2 Administrateurs PVMSS (`PVMSS_Admin`)

- Utilisateurs humains administrateurs de PVMSS.
- Accès typique :
  - Gestion des VMs de tous les utilisateurs de PVMSS.
  - Consultation des ressources cluster/nœuds.
  - Gestion des utilisateurs Proxmox créés par PVMSS.
  - Gestion des pools Proxmox associés aux utilisateurs créés par PVMSS.
  - Ne pas gérer les comptes utilisateurs existants hors du périmètre de PVMSS.
  - Ne pas accéder aux paramètres système globaux non liés aux VMs.

Dans Proxmox, ce compte est créé avec les privilèges suivants :

```bash
pveum roleadd PVMSS_Admin -privs "Sys.Audit VM.Audit VM.PowerMgmt VM.Console VM.Config.CPU VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.Options VM.Config.Cloudinit Datastore.Audit Datastore.AllocateSpace Pool.Allocate User.Modify Permissions.Modify Realm.AllocateUser"

pveum useradd pvmss-admin1@pve \
  -comment "PVMSS administrator <name>" \
  -enable 1

pveum aclmod / -user pvmss-admin1@pve -role PVMSS_Admin -propagate 1
```

### 2.3 Utilisateurs finaux PVMSS (`PVEVMUser`)

- Utilisateurs finaux qui consomment des VMs via PVMSS.
- Accès typique :
  - Voir **uniquement** les VMs de leurs pools.
  - Démarrer / arrêter / redémarrer leurs propres VMs.
  - Accéder à la **console noVNC**.
  - Modifier des paramètres limités de leur VM (ex. description, ISO, options cloud-init).
  - Ne pas accéder aux paramètres système globaux non liés aux VMs.
  - Ne pas accéder aux configurations de cluster ou de nœuds.

Le rôle `PVEVMUser` est déjà créé par Proxmox. Il n'est pas nécessaire de le créer manuellement.

---

## 3. Familles d’API Proxmox utilisées par PVMSS

Cette section donne une vue synthétique des domaines d’API utilisés pour les opérations de PVMSS.

> **Périmètre fonctionnel** : à date, PVMSS **ne gère pas** les fonctionnalités suivantes de Proxmox : backup/snapshot, LXC/containers, SDN via `/cluster/sdn`, firewall Proxmox, haute disponibilité (HA), gestion de storage replication ou de ceph/rbd spécifiques. Ces domaines doivent être documentés et administrés séparément côté Proxmox.

- **Authentification & gestion d’utilisateurs**
  - `/access/ticket`, `/access/users`, `/access/password`, `/access/roles`, `/access/acl`, `/pools`.
- **Cluster & nœuds**
  - `/cluster/status`, `/nodes`, `/nodes/{node}/status`.
- **Gestion des VMs (QEMU)**
  - `/nodes/{node}/qemu`, `/nodes/{node}/qemu/{vmid}/config`, `/status/current`, `/status/{action}`, suppression VM.
  - Guest agent : `/nodes/{node}/qemu/{vmid}/agent/network-get-interfaces`.
- **Stockage**
  - `/storage`, `/nodes/{node}/storage`, `/nodes/{node}/storage/{storage}/content`.
- **Réseau (bridges vmbr)**
  - `/nodes/{node}/network` (les bridges créés par SDN apparaissent ici comme interfaces `vmbrX`).
- **Console / VNC**
  - `/nodes/{node}/qemu/{vmid}/vncproxy`, `/nodes/{node}/qemu/{vmid}/vncwebsocket`.

Les sections suivantes détaillent ces usages par **fonctionnalité PVMSS**, avec une structure stable exploitable par un LLM.

---

## 4. Authentification & gestion d’utilisateurs

### 4.1 Authentification utilisateur PVMSS

- **Feature ID** : `auth.user_ticket`
- **Description** : Authentifier un utilisateur Proxmox depuis PVMSS (login PVMSS) et récupérer un ticket + CSRF token.
- **Fichiers PVMSS** :
  - `backend/proxmox/access.go` → `CreateTicket`
  - `backend/handlers/auth.go`
- **Endpoints Proxmox** :
  - `POST /access/ticket`
- **Paramètres principaux** (cf. API Viewer) :
  - `username` (string, ex. `user@pve` ou `user@pam`)
  - `password` (string)
  - `otp` (string, optionnel)
  - `realm` (string, optionnel)
  - `path` / `privs` (optionnels pour vérification fine des droits)
- **Privilèges nécessaires** :
  - Pas de privilège explicite côté user pour s’authentifier, mais l’utilisateur doit exister et être **enable=1**.
- **Rôles concernés** :
  - `PVMSS_User`, `PVMSS_Admin` (comptes utilisateurs humains).

### 4.2 Création / mise à jour d’utilisateurs Proxmox

- **Feature ID** : `auth.manage_users`
- **Description** : Créer un utilisateur Proxmox correspondant à un utilisateur PVMSS, et mettre à jour son mot de passe.
- **Fichiers PVMSS** :
  - `backend/proxmox/access.go` → `EnsureUser`, `UpdateUserPassword`
  - `backend/handlers/user_pool.go`, `backend/handlers/admin.go` (selon intégration exacte)
- **Endpoints Proxmox** :
  - `GET /access/users/{userid}` (vérification existence)
  - `POST /access/users` (création)
  - `PUT /access/password` (changement de mot de passe)
- **Privilèges Proxmox probables** (à confirmer dans doc PVE) :
  - `User.Modify` (gestion des comptes utilisateurs)
  - `Sys.Audit` (pour lister/voir les users)
- **Rôles concernés** :
  - `PVMSS_Service` (si création automatisée d’utilisateurs par le backend).
  - `PVMSS_Admin` (si les admins PVMSS gèrent les users directement côté Proxmox).

### 4.3 Gestion des pools & ACL

- **Feature ID** : `auth.manage_pools_acl`
- **Description** : Créer des pools Proxmox et affecter des utilisateurs à ces pools avec un rôle PVMSS dédié.
- **Fichiers PVMSS** :
  - `backend/proxmox/access.go` → `EnsurePool`, `EnsurePoolACL`, `EnsureRole`
  - `backend/handlers/user_pool.go`
- **Endpoints Proxmox** :
  - `GET /pools/{poolid}`
  - `POST /pools`
  - `GET /access/roles/{roleid}`
  - `POST /access/roles`
  - `PUT /access/acl`
- **Privilèges Proxmox probables** :
  - `Pool.Allocate` (création et gestion des pools)
  - `Permissions.Modify` (gestion ACL)
  - `Sys.Audit` (lecture globale des rôles & permissions)
- **Rôles concernés** :
  - `PVMSS_Service` (backend qui prépare pools + ACL).
  - `PVMSS_Admin` (administration manuelle complémentaire).

---

## 5. Cluster & nœuds

### 5.1 Détection cluster / standalone

- **Feature ID** : `cluster.status`
- **Description** : Déterminer si Proxmox est en mode cluster, nom du cluster, nombre de nœuds.
- **Fichiers PVMSS** :
  - `backend/proxmox/cluster.go` → `GetClusterStatus`, `GetClusterStatusResty`
  - `backend/handlers/admin_appinfo.go`
- **Endpoints Proxmox** :
  - `GET /cluster/status`
- **Privilèges Proxmox probables** :
  - `Sys.Audit` (lecture du cluster et des nœuds).
- **Rôles concernés** :
  - `PVMSS_Service`, `PVMSS_Admin`.

### 5.2 Liste et détails des nœuds

- **Feature ID** : `nodes.list_and_details`
- **Description** : Lister les nœuds, récupérer leur statut et capacité (CPU/RAM/disk) pour les écrans d’administration et la création de VM.
- **Fichiers PVMSS** :
  - `backend/proxmox/nodes.go` → `GetNodeNames{,Resty}`, `GetOnlineNodeNamesResty`, `GetNodeDetails{,Resty}`
  - `backend/handlers/admin.go`, `backend/handlers/vm_create.go`, `backend/handlers/admin_vms.go`
- **Endpoints Proxmox** :
  - `GET /nodes`
  - `GET /nodes/{node}/status`
- **Privilèges Proxmox probables** :
  - `Sys.Audit`
- **Rôles concernés** :
  - `PVMSS_Service`, `PVMSS_Admin`.

---

## 6. Gestion des VMs (QEMU)

### 6.1 Listing et lecture de configuration VM

- **Feature ID** : `vm.list_and_read_config`
- **Description** :
  - Lister toutes les VMs (profil utilisateur, administration).
  - Lire la configuration détaillée d’une VM (CPU, RAM, disques, réseau, tags, EFI, TPM, etc.).
- **Fichiers PVMSS** :
  - `backend/proxmox/vms.go` → `GetVMsResty`, `GetVMsForNodeResty`, `GetVMConfigResty`, `GetVMCurrentResty`, `GetGuestAgentNetworkInterfaces`
  - `backend/handlers/profile.go`, `backend/handlers/admin_vms.go`, `backend/handlers/vm_details.go`, `backend/state/manager.go`
- **Endpoints Proxmox** :
  - `GET /nodes` (pour itérer sur les nœuds)
  - `GET /nodes/{node}/qemu` (liste VMs d’un nœud)
  - `GET /nodes/{node}/qemu/{vmid}/config`
  - `GET /nodes/{node}/qemu/{vmid}/status/current`
  - `GET /nodes/{node}/qemu/{vmid}/agent/network-get-interfaces`
- **Privilèges Proxmox probables** :
  - `VM.Audit` (lecture des VMs)
  - `Sys.Audit` (lecture des nœuds)
  - Guest agent : aucun privilège additionnel spécifique, mais la VM doit être configurée avec QEMU guest agent.
- **Rôles concernés** :
  - `PVMSS_Service` (lecture globale, cache en RAM, agrégation).
  - `PVMSS_Admin` (vue d’admin complète).
  - `PVMSS_User` (lecture limitée aux VMs de ses pools via ACL).

### 6.2 Création de VM

- **Feature ID** : `vm.create`
- **Description** : Créer une nouvelle VM avec CPU, RAM, disques, réseau, ISO, EFI, TPM, etc.
- **Fichiers PVMSS** :
  - `backend/handlers/vm_create.go`
  - `backend/proxmox/vms.go` → `GetNextVMIDResty` (pour déterminer le prochain VMID)
- **Endpoints Proxmox typiques** (basés sur l’API Viewer `chapter-qm.html`) :
  - `GET /nodes/{node}/qemu` (calcul prochain VMID, via liste VMs)
  - `POST /nodes/{node}/qemu` (création VM, paramétrage initial : cores, memory, disks, net0, bios=ovmf, tpmstate0, etc.)
  - Ensuite, éventuellement des appels complémentaires `POST /nodes/{node}/qemu/{vmid}/config` pour ajuster certains paramètres.
- **Privilèges Proxmox probables** :
  - `VM.Allocate` (création de VMs)
  - `VM.Config.CPU`, `VM.Config.Memory`, `VM.Config.Disk`, `VM.Config.Network`, `VM.Config.Options`, `VM.Config.Cloudinit` (selon les options utilisées)
  - `Datastore.AllocateSpace` (création des disques sur les storages sélectionnés)
  - `Sys.Audit` (lecture des nœuds)
- **Rôles concernés** :
  - `PVMSS_Service` (création au nom de la plateforme).
  - `PVMSS_Admin` (si création manuelle d’admin).

### 6.3 Actions sur VM (power / cycle de vie)

- **Feature ID** : `vm.power_actions`
- **Description** : Démarrer, arrêter, redémarrer, reset, shutdown les VMs.
- **Fichiers PVMSS** :
  - `backend/proxmox/vms.go` → `VMActionResty`
  - `backend/handlers/vm_actions.go`, `backend/handlers/vm_delete.go`
- **Endpoints Proxmox** :
  - `POST /nodes/{node}/qemu/{vmid}/status/start`
  - `POST /nodes/{node}/qemu/{vmid}/status/stop`
  - `POST /nodes/{node}/qemu/{vmid}/status/shutdown`
  - `POST /nodes/{node}/qemu/{vmid}/status/reboot`
  - `POST /nodes/{node}/qemu/{vmid}/status/reset`
- **Privilèges Proxmox probables** :
  - `VM.PowerMgmt`
  - `VM.Audit` (lecture de la VM)
- **Rôles concernés** :
  - `PVMSS_Service`, `PVMSS_Admin`, `PVMSS_User` (limité à ses propres VMs via ACL).

### 6.4 Modification de la configuration VM (resources, tags, réseau, EFI/TPM)

- **Feature ID** : `vm.update_config`
- **Description** : Mettre à jour description, tags, CPU, RAM, disques, réseau (net0..netN), EFI, TPM, etc.
- **Fichiers PVMSS** :
  - `backend/proxmox/vms.go` → `UpdateVMConfigResty`
  - `backend/handlers/vm_details.go`, `backend/handlers/vm_actions.go`
- **Endpoint Proxmox** :
  - `POST /nodes/{node}/qemu/{vmid}/config`
- **Privilèges Proxmox probables** :
  - `VM.Config.CPU`
  - `VM.Config.Memory`
  - `VM.Config.Disk`
  - `VM.Config.Network`
  - `VM.Config.Options`
  - `VM.Config.Cloudinit` (si utilisé)
- **Rôles concernés** :
  - `PVMSS_Service`, `PVMSS_Admin`.
  - Éventuellement `PVMSS_User` si l’on veut lui autoriser quelques modifications ciblées (par exemple seulement `VM.Config.Options`).

### 6.5 Suppression de VM

- **Feature ID** : `vm.delete`
- **Description** : Arrêter proprement une VM puis la supprimer.
- **Fichiers PVMSS** :
  - `backend/handlers/vm_delete.go`
  - `backend/proxmox/vms.go` → `VMActionResty`, `DeleteVMResty`
- **Endpoints Proxmox** :
  - `POST /nodes/{node}/qemu/{vmid}/status/stop` (ou `shutdown`)
  - `DELETE /nodes/{node}/qemu/{vmid}`
- **Privilèges Proxmox probables** :
  - `VM.Allocate` (pour supprimer)
  - `VM.PowerMgmt`
  - `Datastore.AllocateSpace` (libération espace disque, selon doc Proxmox)
- **Rôles concernés** :
  - `PVMSS_Service`, `PVMSS_Admin`.

---

## 7. Stockage & ISOs

### 7.1 Listing des stockages et de leur état

- **Feature ID** : `storage.list`
- **Description** : Lister les storages disponibles, leur type, usage, et ceux visibles par un nœud donné.
- **Fichiers PVMSS** :
  - `backend/proxmox/storage.go` → `GetStoragesResty`, `GetNodeStoragesResty`
  - `backend/handlers/storage.go`, `backend/handlers/vm_create.go`
- **Endpoints Proxmox** :
  - `GET /storage`
  - `GET /nodes/{node}/storage`
- **Privilèges Proxmox probables** :
  - `Datastore.Audit`
  - `Sys.Audit` (lecture des nœuds)
- **Rôles concernés** :
  - `PVMSS_Service`, `PVMSS_Admin`.

### 7.2 Listing des ISOs et contenus de stockage

- **Feature ID** : `storage.list_content`
- **Description** : Lister les ISOs et autres contenus (images, templates) d’un storage.
- **Fichiers PVMSS** :
  - `backend/proxmox/iso.go` → `GetISOListResty`, `GetAllStorageContentResty`
  - `backend/handlers/settings_iso.go`, `backend/handlers/vm_create.go`
- **Endpoint Proxmox** :
  - `GET /nodes/{node}/storage/{storage}/content`
- **Privilèges Proxmox probables** :
  - `Datastore.Audit`
- **Rôles concernés** :
  - `PVMSS_Service`, `PVMSS_Admin`.

### 7.3 Création / suppression de volumes

- **Feature ID** : `storage.create_delete_volume`
- **Description** : Créer ou supprimer des volumes (disques) pour les VMs gérées par PVMSS.
- **Endpoints Proxmox typiques** :
  - `POST /nodes/{node}/storage/{storage}/content` (création upload/volume)
  - `DELETE /nodes/{node}/storage/{storage}/content/{volid}` (suppression)
- **Privilèges Proxmox probables** :
  - `Datastore.AllocateSpace`
  - Éventuellement `Datastore.Allocate` (selon le cas d’usage exact)
- **Rôles concernés** :
  - `PVMSS_Service`, `PVMSS_Admin`.

---

## 8. Réseau (bridges vmbr)

### 8.1 Lecture des interfaces réseau et bridges

- **Feature ID** : `network.list_bridges`
- **Description** : Lister les interfaces réseau d’un nœud et filtrer les bridges (`vmbrX`) utilisables lors de la création/modification de VM.
- **Fichiers PVMSS** :
  - `backend/proxmox/vmbr.go` → `GetVMBRsResty`, `GetAllNetworkInterfacesResty`
  - `backend/handlers/vmbr.go`, `backend/handlers/vm_create.go`, `backend/handlers/vm_details.go`
- **Endpoint Proxmox** :
  - `GET /nodes/{node}/network`
- **Privilèges Proxmox probables** :
  - `Sys.Audit` (lecture de la configuration réseau du nœud)
- **Rôles concernés** :
  - `PVMSS_Service`, `PVMSS_Admin`.

### 8.2 Gestion avancée réseau (optionnelle)

Si PVMSS devait créer / modifier des bridges :

- **Endpoints** :
  - `POST /nodes/{node}/network`
  - `DELETE /nodes/{node}/network/{iface}`
- **Privilèges Proxmox probables** :
  - `Sys.Modify` ou privilèges réseau spécifiques (à valider dans la doc Proxmox).
- **Rôles concernés** :
  - Probablement **réservé** à un rôle très privilégié (équivalent `PVEAdmin`).

---

## 9. Console noVNC / VNC proxy

### 9.1 Obtention d’un ticket VNC et d’un port WebSocket

- **Feature ID** : `console.vncproxy`
- **Description** : Générer un ticket VNC et un port WebSocket pour ouvrir la console noVNC d’une VM.
- **Fichiers PVMSS** :
  - `backend/proxmox/vnc.go` → `GetVNCProxy`
  - `backend/handlers/vm_console_api.go`, `vm_console_websocket.go`, `vm_console_helpers.go`
- **Endpoint Proxmox** :
  - `POST /nodes/{node}/qemu/{vmid}/vncproxy`
- **Privilèges Proxmox probables** :
  - `VM.Console` (accès console)
  - `VM.Audit` (lecture VM)
- **Rôles concernés** :
  - `PVMSS_Service` (si la plateforme gère un proxy HTTP/WS côté backend).
  - `PVMSS_Admin`, `PVMSS_User` (accès console à leurs VMs via ticket).

### 9.2 Connexion WebSocket noVNC

- **Feature ID** : `console.websocket`
- **Description** : Établir la connexion WebSocket entre le navigateur et Proxmox (directement ou via proxy HTTP PVMSS).
- **Endpoint Proxmox** :
  - `GET /nodes/{node}/qemu/{vmid}/vncwebsocket?port={port}&vncticket={ticket}` (appelé par noVNC)
- **Privilèges Proxmox** :
  - Identiques à la création du ticket VNC (`VM.Console`).
- **Rôles concernés** :
  - `PVMSS_Admin`, `PVMSS_User` (via leur propre authentification et tickets temporaires).

---

## 10. Rôles recommandés et mapping de privilèges

> **IMPORTANT** : Les listes ci-dessous sont des recommandations à affiner avec la doc officielle Proxmox (User Management + pveum). L’idée est de couvrir les fonctionnalités PVMSS listées ci-dessus avec le minimum de privilèges raisonnable.

### 10.1 Rôle `PVMSS_Service` (compte technique API)

- **But** : compte non interactif utilisé par le backend.
- **Scope recommandé** : chemin `/` avec propagation (plus simple), ou chemin restreint selon architecture multi-tenant.
- **Privilèges typiques** :
  - Cluster / nœuds
    - `Sys.Audit`
  - VMs
    - `VM.Audit`
    - `VM.Allocate`
    - `VM.PowerMgmt`
    - `VM.Console` (utile pour debug console via proxy)
    - `VM.Config.CPU`, `VM.Config.Memory`, `VM.Config.Disk`, `VM.Config.Network`, `VM.Config.Options`, `VM.Config.Cloudinit`
  - Stockage
    - `Datastore.Audit`
    - `Datastore.AllocateSpace`
  - Pools / ACL / utilisateurs (si gérés par la plateforme)
    - `Pool.Allocate`
    - `User.Modify`
    - `Permissions.Modify`

### 10.2 Rôle `PVMSS_Admin`

- **But** : administrateurs humains de PVMSS.
- **Scope recommandé** : `/` ou sous-arbre dédié aux ressources PVMSS.
- **Privilèges typiques** :
  - Tous ceux de `PVMSS_Service` côté VM + stockage + lecture cluster.
  - `Pool.Allocate`, `User.Modify`, `Permissions.Modify` si les admins gèrent pools/users.
  - Éviter si possible les privilèges systèmes très globaux (ex. `Sys.Modify`, `Realm.Allocate`) sauf nécessité.

### 10.3 Rôle `PVMSS_User`

- **But** : utilisateurs finaux PVMSS, limités à leurs VMs.
- **Scope recommandé** : appliquer le rôle sur `/pool/{poolid}` avec `propagate=1` pour chaque pool dédié.
- **Privilèges typiques** :
  - `VM.Audit` (voir leurs VMs)
  - `VM.Console` (console)
  - `VM.PowerMgmt` (démarrer/arrêter/redémarrer leurs VMs)
  - (Optionnel) certains `VM.Config.*` si on veut autoriser des modifications ciblées (par exemple `VM.Config.Options`).

---

## 11. Résumé machine-friendly (pour LLM)

Cette section fournit un format plus structuré que les agents peuvent parser facilement.

### 11.1 Dictionnaire `features → endpoints → privilèges`

- **Feature** `auth.user_ticket`
  - `endpoints` : `POST /access/ticket`
  - `min_privileges` : `[]` (simple login)
  - `roles` : `[PVMSS_User, PVMSS_Admin]`

- **Feature** `auth.manage_users`
  - `endpoints` : `GET /access/users/{userid}`, `POST /access/users`, `PUT /access/password`
  - `min_privileges` : `[User.Modify, Sys.Audit]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `auth.manage_pools_acl`
  - `endpoints` : `GET /pools/{poolid}`, `POST /pools`, `GET /access/roles/{roleid}`, `POST /access/roles`, `PUT /access/acl`
  - `min_privileges` : `[Pool.Allocate, Permissions.Modify, Sys.Audit]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `cluster.status`
  - `endpoints` : `GET /cluster/status`
  - `min_privileges` : `[Sys.Audit]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `nodes.list_and_details`
  - `endpoints` : `GET /nodes`, `GET /nodes/{node}/status`
  - `min_privileges` : `[Sys.Audit]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `vm.list_and_read_config`
  - `endpoints` : `GET /nodes/{node}/qemu`, `GET /nodes/{node}/qemu/{vmid}/config`, `GET /nodes/{node}/qemu/{vmid}/status/current`, `GET /nodes/{node}/qemu/{vmid}/agent/network-get-interfaces`
  - `min_privileges` : `[VM.Audit, Sys.Audit]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin, PVMSS_User]`

- **Feature** `vm.create`
  - `endpoints` : `GET /nodes/{node}/qemu`, `POST /nodes/{node}/qemu`
  - `min_privileges` : `[VM.Allocate, VM.Config.*, Datastore.AllocateSpace, Sys.Audit]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `vm.power_actions`
  - `endpoints` : `POST /nodes/{node}/qemu/{vmid}/status/{action}`
  - `min_privileges` : `[VM.PowerMgmt, VM.Audit]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin, PVMSS_User]`

- **Feature** `vm.update_config`
  - `endpoints` : `POST /nodes/{node}/qemu/{vmid}/config`
  - `min_privileges` : `[VM.Config.*]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `vm.delete`
  - `endpoints` : `POST /nodes/{node}/qemu/{vmid}/status/stop`, `DELETE /nodes/{node}/qemu/{vmid}`
  - `min_privileges` : `[VM.Allocate, VM.PowerMgmt, Datastore.AllocateSpace]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `storage.list`
  - `endpoints` : `GET /storage`, `GET /nodes/{node}/storage`
  - `min_privileges` : `[Datastore.Audit, Sys.Audit]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `storage.list_content`
  - `endpoints` : `GET /nodes/{node}/storage/{storage}/content`
  - `min_privileges` : `[Datastore.Audit]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `storage.create_delete_volume`
  - `endpoints` : `POST /nodes/{node}/storage/{storage}/content`, `DELETE /nodes/{node}/storage/{storage}/content/{volid}`
  - `min_privileges` : `[Datastore.AllocateSpace]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `network.list_bridges`
  - `endpoints` : `GET /nodes/{node}/network`
  - `min_privileges` : `[Sys.Audit]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `console.vncproxy`
  - `endpoints` : `POST /nodes/{node}/qemu/{vmid}/vncproxy`
  - `min_privileges` : `[VM.Console, VM.Audit]`
  - `roles` : `[PVMSS_Service, PVMSS_Admin, PVMSS_User]`

- **Feature** `console.websocket`
  - `endpoints` : `GET /nodes/{node}/qemu/{vmid}/vncwebsocket`
  - `min_privileges` : `[VM.Console]`
  - `roles` : `[PVMSS_Admin, PVMSS_User]`

> Pour tout ajustement fin des privilèges (par exemple séparer `VM.Config.Disk` de `VM.Config.CPU`), se référer impérativement aux sections « Privileges » et « Predefined Roles » de la doc Proxmox, et adapter ce dictionnaire en conséquence.

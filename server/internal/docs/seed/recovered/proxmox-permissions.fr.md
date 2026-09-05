# Permissions Proxmox pour PVMSS

Cette page décrit les rôles et le jeton API Proxmox dont PVMSS a besoin, ainsi
que les privilèges requis par chaque rôle. Les commandes `pveum` exactes sont
tenues à jour avec le README du projet. Ce sont les rôles recommandés ;
validez toujours la liste des privilèges contre la documentation Proxmox de
votre version PVE.

## Aperçu des rôles

PVMSS s'appuie sur des rôles et ACL Proxmox dédiés pour trois acteurs :

- **PVMSS_Service** — le compte de service du backend, utilisé via un jeton API
  (`PROXMOX_API_TOKEN_NAME` / `PROXMOX_API_TOKEN_VALUE`). Il effectue les
  opérations cluster, nœud, VM, stockage, réseau et utilisateur/pool pour le
  compte de l'application.
- **PVMSS_Admin** — les administrateurs humains de PVMSS. Ils gèrent les VMs,
  utilisateurs et pools créés par PVMSS, et consultent les ressources cluster et
  nœuds.
- **PVMSSUser** — le rôle par pool attribué automatiquement à chaque
  utilisateur de libre-service. PVMSS provisionne ce rôle, l'utilisateur, le
  pool et l'ACL lorsque vous créez un pool depuis `/admin/pools`. Vous ne le
  créez pas manuellement.

Le rôle par défaut `PVEVMUser` n'est pas utilisé par PVMSS ; les
utilisateurs de libre-service reçoivent le rôle ciblé `PVMSSUser` sur leur
propre pool à la place.

## Compte de service (PVMSS_Service)

Exécutez en tant que `root` sur le nœud Proxmox. Créez le rôle, puis
l'utilisateur et son jeton API. Le secret du jeton doit être stocké dans
`PROXMOX_API_TOKEN_VALUE`.

```bash
pveum roleadd PVMSS_Service -privs "Sys.Audit VM.Audit VM.Allocate VM.PowerMgmt VM.Console VM.Config.CDROM VM.Config.CPU VM.Config.HWType VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.Options VM.Config.Cloudinit VM.Snapshot VM.Snapshot.Rollback Datastore.Audit Datastore.AllocateSpace Datastore.AllocateTemplate Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Allocate SDN.Audit SDN.Use"

pveum useradd pvmss-svc@pve \
  -comment "PVMSS service account" \
  -enable 1

pveum aclmod / -user pvmss-svc@pve -role PVMSS_Service -propagate 1

pveum user token add pvmss-svc@pve pvmss-service-token --privsep 0
```

## Administrateurs (PVMSS_Admin)

```bash
pveum roleadd PVMSS_Admin -privs "Sys.Audit VM.Audit VM.PowerMgmt VM.Console VM.Config.CPU VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.HWType VM.GuestAgent.Audit VM.Migrate VM.Config.CDROM VM.Config.Options VM.Config.Cloudinit Datastore.Audit Datastore.AllocateSpace Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Audit Group.Allocate"

pveum useradd pvmss-admin1@pve \
  -comment "PVMSS administrator" -password "strong_password" \
  -enable 1

pveum aclmod / -user pvmss-admin1@pve -role PVMSS_Admin -propagate 1
```

## Pools par utilisateur (PVMSSUser)

PVMSS crée le rôle `PVMSSUser`, l'utilisateur, le pool dédié et l'ACL
automatiquement lorsqu'un administrateur provisionne un pool depuis
`/admin/pools`. Aucune étape `pveum` manuelle n'est requise pour les
utilisateurs finaux. Chaque utilisateur ne voit que les VMs de son propre pool.

## Recommandations de sécurité

- Utilisez des comptes Proxmox dédiés pour PVMSS ; ne partagez jamais le mot de
  passe administrateur intégré.
- Gardez le secret du jeton de service ; il porte des privilèges élevés sur tout
  le cluster.
- Restreignez les comptes de service et d'administration à leurs opérations
  respectives ; ne les réutilisez pas pour d'autres tâches.
- Préférez un jeton de service dédié aux jetons `root@pam` en production.

## Périmètre fonctionnel

PVMSS ne gère pas les fonctionnalités Proxmox suivantes via sa propre
interface : sauvegardes et restauration, conteneurs LXC, orchestration SDN
au-delà de la lecture des ponts, pare-feu Proxmox, haute disponibilité,
réplication de stockage, et gestion spécifique Ceph/RBD. Celles-ci restent
administrées directement dans Proxmox.

## Familles d'API utilisées par PVMSS

PVMSS appelle les familles d'API Proxmox standard :

- **Authentification & utilisateurs** : `/access/ticket`, `/access/users`,
  `/access/password`, `/access/roles`, `/access/acl`, `/pools`.
- **Cluster & nœuds** : `/cluster/status`, `/nodes`, `/nodes/{node}/status`.
- **Gestion des VMs (QEMU)** : `/nodes/{node}/qemu`, ses sous-chemins `config`
  et `status`, et la suppression de VM.
- **Stockage** : `/storage`, `/nodes/{node}/storage`,
  `/nodes/{node}/storage/{storage}/content`.
- **Réseau** : `/nodes/{node}/network`.
- **Console / VNC** : `/nodes/{node}/qemu/{vmid}/vncproxy` et `vncwebsocket`.

Les privilèges listés par rôle ci-dessus sont les minimaux nécessaires à ces
appels.

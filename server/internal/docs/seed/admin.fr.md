# Guide admin

Cette page n'est visible que des administrateurs. Elle résume la surface
d'administration ; la procédure complète est dans le
[Guide administrateur](/docs/admin-guide).

## Infrastructure

**Clusters** connecte un ou plusieurs environnements Proxmox (URL, token
d'API, test de connexion) et définit la cible d'écriture cloud-init par
cluster. **Nœuds** approuve les hôtes sur lesquels les utilisateurs peuvent
créer des VM. **Pools** crée les utilisateurs self-service (utilisateur
Proxmox + pool + ACL en une étape).

### Cible d'écriture cloud-init (stockage snippets)

PVMSS écrit les vendor-data cloud-init de chaque VM comme un fichier snippet
sur le cluster, puis l'attache via `cicustom`. L'API REST de Proxmox ne peut
pas écrire de snippets, donc PVMSS écrit directement dans un répertoire
bind-mounté dans son conteneur. Pour activer les documents cloud-init et la
baseline générée (qemu-guest-agent) sur un cluster :

> **Pas de SSH, pas d'upload API.** PVMSS ne se connecte pas en SSH à Proxmox
> et l'API REST de Proxmox ne peut pas écrire de fichiers snippets. PVMSS
> écrit le fichier dans un répertoire de son propre processus, et ce
> répertoire doit être le *même répertoire physique* que celui dont Proxmox
> lit les snippets - rendu visible par un bind mount, NFS, ou en exécutant
> PVMSS sur l'hôte Proxmox. Si le fichier apparaît dans votre répertoire de
> snippets configuré mais que la VM ne peut pas démarrer, le répertoire n'est
> pas celui dont Proxmox sert les snippets. Vérifiez sur l'hôte Proxmox avec
> `pvesm path <stockage>` - le chemin snippets est ce chemin plus `/snippets`.

1. Choisissez un stockage Proxmox ayant le contenu **snippets** activé. Pour
   l'activer : **Datacenter → Stockage → <stockage> → Contenu**, cochez
   `snippets`. Le stockage doit être visible par chaque nœud qui hébergera
   des VM issues d'images - un stockage partagé (NFS, CephFS, `dir` sur un
   montage partagé) est le choix usuel.
2. Dans **Admin → Clusters → Modifier**, le champ **Stockage snippets**
   liste chaque stockage compatible snippets que PVMSS voit sur le cluster.
   Sélectionnez-en un. Si la liste est vide, aucun stockage du cluster
   n'annonce le contenu snippets - activez-le dans Proxmox puis rouvrez le
   formulaire.
3. Montez le répertoire `snippets/` de ce stockage dans le conteneur PVMSS à
   un chemin connu et indiquez ce chemin dans **Répertoire des snippets**.
   Le chemin doit être le même répertoire que celui que Proxmox lit pour ce
   stockage. Pour un stockage `dir` avec `path /var/lib/vz`, le répertoire
   snippets est `/var/lib/vz/snippets` ; pour NFS ou CephFS, montez le
   partage et utilisez son sous-répertoire `snippets/`.
4. Enregistrez et lancez **Test**. La ligne du cluster doit afficher
   `cloud-init : on`.

Laissez les deux champs vides pour désactiver les documents cloud-init sur
le cluster. Les VM créées depuis des images cloud signaleront alors
`Baseline cloud-init non livrée` et la baseline qemu-guest-agent ne sera pas
attachée.

### Alternative : livraison des snippets par SSH

Quand PVMSS ne peut pas partager un système de fichiers avec Proxmox (hôtes
distincts, pas de NFS, pas de bind mount), il peut livrer les snippets par
SSH. PVMSS résout l'IP de chaque nœud Proxmox via `/cluster/status` et se
connecte en SSH au nœud spécifique où la VM est créée, donc aucune
configuration SSH par cluster n'est nécessaire - une seule clé globale
suffit.

Définissez ces variables d'environnement sur le serveur PVMSS :

| Variable             | Notes                                                       |
| -------------------- | ----------------------------------------------------------- |
| `PVMSS_SSH_USER`     | Utilisateur SSH sur les hôtes Proxmox (ex. `root`)          |
| `PVMSS_SSH_KEY_FILE` | Chemin de la clé privée (requis quand l'utilisateur est défini) |
| `PVMSS_SSH_PORT`     | Port SSH (22 par défaut)                                    |

La clé doit être autorisée sur chaque nœud Proxmox qui hébergera des VM
cloud-init (ajoutez-la à `~/.ssh/authorized_keys` pour l'utilisateur SSH). Le
**répertoire de snippets** configuré par cluster est alors le chemin sur
l'hôte Proxmox, pas un montage local. La vérification de la clé hôte n'est
pas appliquée ; à utiliser sur un réseau de gestion de confiance.

**Clusters multi-nœuds :** SSH écrit le snippet sur le nœud spécifique où la
VM est créée. Pour un cluster, vous devriez toujours utiliser un stockage de
snippets partagé (NFS, CephFS) afin que le snippet soit visible par tous les
nœuds - sinon une VM migrée pointerait vers un fichier que le nouveau nœud
ne peut pas voir. La livraison par SSH est principalement destinée au
Proxmox à nœud unique ou aux clusters qui ont déjà un stockage partagé mais
pas de montage de système de fichiers partagé dans le conteneur PVMSS.

Quand `PVMSS_SSH_USER` est vide (valeur par défaut), la livraison par SSH est
désactivée et l'approche par système de fichiers partagé ci-dessus est
utilisée.

## Catalogue

La section **Catalogue** permet d'approuver ou masquer les stockages, ISO,
images cloud, templates de VM et bridges que la création de VM peut
référencer, et de gérer les profils matériels, tags et templates cloud-init.
Les ressources découvertes apparaissent automatiquement ; l'interrupteur
« activé » contrôle ce que voient les utilisateurs. Quand une ressource
disparaît de Proxmox, son approbation obsolète est signalée pour que vous
puissiez la retirer.

## Politique

**Limites** définit le gabarit par cluster (sockets, cœurs, mémoire, disque
par VM, cartes réseau, snapshots, YAML cloud-init libre, VLAN d'isolation) et
le quota par utilisateur (VM max). **Capacité des nœuds** plafonne ce que
PVMSS peut allouer sur chaque nœud. Les deux sont appliqués côté serveur avant
tout appel à Proxmox.

## Documentation

Cette page est elle-même gérée sous **Documentation** dans le menu admin. Les
administrateurs peuvent créer, modifier, activer/désactiver et supprimer des
pages Markdown en anglais et en français. Les pages intégrées (comme
celle-ci) sont marquées **système** et ne peuvent pas être supprimées, mais
leur contenu peut être modifié. Chaque page a une audience `user` (publique)
ou `admin` (administrateurs uniquement).

## Système

La page **Infos application** affiche la version, l'environnement et l'état
des clusters. **Paramètres** regroupe le journal d'audit (avec rétention et
aperçu de purge) et l'export / import en deux temps de la base de données.

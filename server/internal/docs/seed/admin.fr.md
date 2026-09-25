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

### Publication cloud-init (SSH)

Les documents cloud-init sont écrits uniquement par les administrateurs
(**Admin > Modèles cloud-init**). PVMSS publie chacun d'eux, fusionné avec
la base qemu-guest-agent, sous forme d'un fichier immuable
`pvmss-tpl-<id>-<hash>.yml` dans le stockage de snippets de chaque nœud, par
SSH (l'API REST de Proxmox ne sait pas écrire de snippets). La création
d'une VM n'écrit jamais de fichier : la VM pointe vers le fichier publié.

1. Dans Proxmox, activez le type de contenu **Snippets** sur un stockage
   disponible sur chaque nœud (`local` convient).
2. Générez une paire de clés et fournissez la clé privée à PVMSS :

   | Variable             | Notes                                                        |
   | -------------------- | ------------------------------------------------------------ |
   | `PVMSS_SSH_KEY_FILE` | Chemin de la clé privée (montage en lecture seule ou Secret) |

3. Sur chaque nœud, en root, lancez `tools/pvmss-node-setup.sh --storage
   <stockage> --key '<clé publique PVMSS>'`. Il installe l'utilitaire
   `pvmss-snippet`, un utilisateur dédié `pvmss` limité au répertoire
   `snippets/` du stockage, et la clé avec une commande forcée (pas de shell).
4. Dans **Admin > Clusters > Modifier**, renseignez le stockage de snippets,
   l'utilisateur SSH, le port et les clés d'hôte épinglées : collez les
   lignes de `ssh-keyscan -t ed25519 <ip du nœud>`, ou enregistrez d'abord
   sans utilisateur SSH, rouvrez et cliquez sur **Scanner les clés d'hôte**.
   Vérifiez les empreintes et enregistrez. Les clés d'hôte sont toujours
   vérifiées. La ligne du cluster affiche `cloud-init : activé`, ou
   l'élément manquant.

Laissez l'utilisateur SSH vide pour désactiver les documents cloud-init sur
le cluster. Voir le [guide de configuration cloud-init](/docs/cloud-init-setup)
pour le détail et le dépannage.

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
par VM, cartes réseau, snapshots, VLAN d'isolation) et
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

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

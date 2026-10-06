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

### Modèles cloud-init

Seuls les administrateurs écrivent les documents cloud-init (**Admin >
Cloud-init**). L'API REST de Proxmox ne peut pas écrire de snippets, donc
PVMSS n'écrit jamais sur les nœuds : pour chaque modèle, il affiche une
commande que l'administrateur colle, en root, sur les nœuds qui doivent le
proposer. PVMSS lit via l'API quels nœuds ont le fichier et ne propose le
modèle que sur ces nœuds.

1. Dans Proxmox, activez le type de contenu **Snippets** sur un stockage
   (`local` convient).
2. Dans **Infrastructure > Clusters > Modifier**, choisissez ce stockage de
   snippets.
3. Écrivez le modèle, copiez sa commande, collez-la sur les nœuds choisis,
   cliquez sur **Vérifier**.

Laissez le stockage vide pour désactiver les modèles cloud-init sur le
cluster. Voir le [guide de configuration cloud-init](/docs/cloud-init-setup)
pour le détail et le dépannage.

## Catalogue

La section **Catalogue** permet d'approuver ou masquer les stockages, ISO,
images cloud, templates de VM et bridges que la création de VM peut
référencer, et de gérer les profils matériels, tags et templates cloud-init.
Les ressources découvertes apparaissent automatiquement ; l'interrupteur
« activé » contrôle ce que voient les utilisateurs. Quand une ressource
disparaît de Proxmox, son approbation obsolète est signalée pour que vous
puissiez la retirer.

## Migrer une VM

Dans **Infrastructure > Nœuds**, ouvrez un nœud : chaque VM gérée par PVMSS a
un bouton **Migrer...**. La fenêtre vérifie où la VM peut aller, vous choisissez
un nœud cible, relisez le résumé puis confirmez. La migration s'exécute comme
une tâche Proxmox, suivie dans le tiroir des tâches (vous pouvez fermer la
fenêtre entre-temps), et la VM quitte la liste du nœud source une fois
terminée. Une VM en marche migre à chaud, une VM arrêtée à froid.

Limites :

- Réservé aux administrateurs, une VM à la fois, au sein d'un seul cluster.
- Uniquement les VM gérées par PVMSS (étiquette `pvmss`).
- Les cibles sont les nœuds approuvés, en ligne et acceptés par Proxmox pour
  cette VM.
- Il n'y a pas de choix du stockage cible : les disques locaux sont emportés
  par Proxmox (`with-local-disks`) quand la VM en a.
- Les plafonds de capacité par nœud sont des avertissements, pas des blocages :
  une cible au-dessus de son plafond affiche un badge et reste sélectionnable.
- Une VM verrouillée, ou en marche avec des ressources locales (passthrough
  PCI ou USB), est refusée ; arrêtez la VM ou attendez la fin du verrou.
- Les VM gérées par HA et la migration entre clusters sont laissées à Proxmox.

La migration est consignée dans le journal d'audit (`vm.migrate`, avec le nœud
source et le nœud cible). Le jeton du compte de service Proxmox doit disposer de `VM.Migrate`
(voir les [permissions Proxmox](/docs/proxmox-permissions)).

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

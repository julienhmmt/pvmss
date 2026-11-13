# Guide admin

Seuls les administrateurs voient cette page. La procédure complète est dans le
[Guide administrateur](/docs/admin-guide).

## Infrastructure

**Clusters** connecte un ou plusieurs environnements Proxmox (URL, token
d'API, test de connexion). Chaque cluster a un identifiant (choisi à la création ; `default` pour le premier) et un libellé modifiable ; le formulaire d'édition permet aussi de renommer l'identifiant, qui est réécrit partout où PVMSS le stocke - les anciens liens cessent de fonctionner. **Nœuds** approuve les hôtes sur lesquels les utilisateurs peuvent
créer des VM. **Utilisateurs PVMSS** crée les utilisateurs self-service
(utilisateur Proxmox + pool + ACL en une étape).

### Les modèles de VM portent les accès

PVMSS n'ajoute ni clé SSH ni mot de passe aux VM qu'il crée (l'écriture
cloud-init a été supprimée). Un clone garde exactement ce que porte son
modèle Proxmox : préparez donc chaque modèle approuvé avec les accès dont les
utilisateurs auront besoin :

- dans l'onglet **Cloud-Init** du modèle dans Proxmox, renseignez
  l'utilisateur et une clé SSH publique ou un mot de passe, ou intégrez un
  compte directement dans l'image ;
- installez et activez `qemu-guest-agent`, pour que la VM signale son adresse
  et que l'onglet **Vue d'ensemble** affiche la commande SSH.

Toutes les VM clonées d'un même modèle partagent ces accès. Transmettez-les
à vos utilisateurs, ou gardez un modèle par équipe. Les installations depuis
une ISO ne sont pas concernées : l'utilisateur crée son propre compte pendant
l'installation, depuis la console du navigateur.

## Ressources et provisionnement

Le groupe **Ressources** permet d'approuver ou masquer les stockages, ISO,
modèles de VM et bridges que la création de VM peut
référencer. Le groupe **Provisionnement** gère les profils VM et les tags. Les ressources découvertes apparaissent automatiquement ; l'interrupteur
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
le quota par pool (VM max). Un pool peut avoir sa propre limite : définissez
ou retirez une surcharge depuis la page de détail du pool, et relisez-les
toutes sur la page des limites. Un utilisateur arrivé à son quota peut
déposer une demande d'augmentation depuis la page de création ; les demandes
en attente mènent les alertes du tableau de bord et le compteur de la barre
latérale. En approuver une écrit une surcharge pour le pool du demandeur
(éventuellement un autre nombre) ; refuser ne change rien. **Capacité des
nœuds** plafonne ce que PVMSS peut allouer sur chaque nœud. Tout cela est
appliqué côté serveur avant tout appel à Proxmox.

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
aperçu de purge), l'onglet **Comptes** réservé au propriétaire pour les
administrateurs locaux, et l'export / import en deux temps de la base de
données. Réinitialiser, désactiver ou supprimer un compte révoque ses
sessions ; si le seul propriétaire perd l'accès, définissez
`ADMIN_PASSWORD_HASH` et redémarrez pour redevenir propriétaire - détails
dans le [Guide administrateur](/docs/admin-guide).

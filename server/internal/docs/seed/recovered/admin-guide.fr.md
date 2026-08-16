# Guide de l'administrateur

Bienvenue dans le guide de l'administrateur PVMSS. PVMSS (Proxmox Virtual
Machine Self-Service) est un portail en libre-service permettant à vos
utilisateurs de créer, exploiter et dépanner des machines virtuelles Proxmox
VE sans exposer l'interface Proxmox.

L'administrateur dispose d'un accès complet à toutes les fonctionnalités de
l'application. Il n'y a pas de rôle auditeur ou observateur distinct : se
connecter avec un compte administrateur déverrouille à la fois l'interface
utilisateur standard et tout ce qui se trouve sous `/admin`.

## Premiers pas

1. Ouvrez le panneau d'administration sur `/admin` (un compte administrateur est requis).
2. Consultez **Informations sur l'application** (`/admin/appinfo`) pour confirmer que l'instance est connectée au bon environnement Proxmox.
3. Approuvez les ressources que les utilisateurs peuvent utiliser : **Nœuds**, **Stockages**, **ISO**, **Ponts**, et **Modèles cloud-init**.
4. Facultatif mais recommandé : créez des **Profils de VM** pour que les utilisateurs puissent choisir des configurations matérielles pré-approuvées, et définissez des **Tags** pour organiser les VMs.
5. Définissez la **Politique** (quotas par utilisateur) et les plafonds de **Capacité des nœuds**.
6. Créez autant de **Pools utilisateurs** que nécessaire depuis `/admin/pools`.
7. Indiquez à vos utilisateurs que le portail est disponible pour qu'ils commencent à créer des VMs.
8. Surveillez le **Journal d'audit** (`/admin/settings`) pour retracer chaque écriture de VM jusqu'à l'utilisateur responsable.

## Informations sur l'application (App Info)

`/admin/appinfo` offre une vue d'ensemble en lecture seule de l'instance en cours :

- **Informations de build** : version de l'application, version de Go, système d'exploitation et architecture.
- **Environnement** : indique si PVMSS fonctionne contre un vrai cluster Proxmox (`PVMSS_CLUSTER_SOURCE=proxmox`) ou le cluster d'essai intégré (`PVMSS_CLUSTER_SOURCE=fake`).
- **Statut du cluster Proxmox** : nom du cluster et nombre de nœuds en mode cluster, ou mode autonome pour un nœud unique.
- **Variables d'environnement (sous-ensemble sûr)** : configuration non sensible telle que `PROXMOX_URL`, `PVMSS_PORT`, `PVMSS_DB_PATH`.

Utilisez cette page pour confirmer la connectivité après un déploiement ou une modification de configuration. Si l'instance ne peut pas joindre Proxmox, vérifiez les journaux du serveur et les variables d'environnement ci-dessous.

## Configuration (variables d'environnement)

PVMSS se configure entièrement via des variables d'environnement, validées au démarrage. Le serveur refuse de démarrer si une valeur requise est manquante ou mal formée.

Requises :

- `PVMSS_PORT` — port TCP sur lequel le serveur écoute (l'image officielle utilise `50000`).
- `PVMSS_DB_PATH` — chemin vers le fichier de base de données SQLite.
- `SESSION_SECRET` — 32+ octets utilisés pour chiffrer les sessions utilisateur.
- `LOG_LEVEL` — `debug`, `info`, `warn` ou `error` (minuscules uniquement).
- `LOG_FORMAT` — `json` ou `console`.
- `LOG_OUTPUT` — `stdout`, `stderr` ou un chemin de fichier.
- `PVMSS_CLUSTER_SOURCE` — `proxmox` ou `fake`. Volontairement sans défaut : `fake` embarque des identifiants de démonstration et ne doit jamais être sélectionné par accident.

Requises quand `PVMSS_CLUSTER_SOURCE=proxmox` :

- `PROXMOX_URL` — par exemple `https://hote:8006/api2/json`.
- `PROXMOX_API_TOKEN_NAME` — l'identifiant du jeton API Proxmox (`user@pve!token`).
- `PROXMOX_API_TOKEN_VALUE` — le secret associé au jeton.

Optionnelles :

- `PVMSS_HOST` — adresse de liaison (l'image définit `0.0.0.0`).
- `PVMSS_WEB_DIR` — emplacement de la SPA compilée (par défaut un chemin relatif à l'exécutable).
- `ADMIN_PASSWORD_HASH` — si défini, doit être un hash bcrypt (`$2…`) ; permet d'épingler le mot de passe administrateur.
- `PVMSS_COOKIE_SECURE` — `true` par défaut ; passez à `false` uniquement en HTTP pour les essais locaux.
- `PVMSS_INVENTORY_REFRESH_INTERVAL` — intervalle d'actualisation de l'inventaire en arrière-plan (défaut `30s`).
- `PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL` — délai minimal entre deux actualisations manuelles (défaut `5s`).
- `PVMSS_INVENTORY_REFRESH_TIMEOUT` — délai par actualisation (défaut `15s`).
- `PVMSS_MAX_LIST_PAGE_SIZE` — taille maximale des pages de liste (défaut `100`).

Pour les instructions de déploiement complètes (Docker, Kubernetes, Helm), consultez le README du projet.

## Nœuds

`/admin/nodes` liste chaque hôte Proxmox VE avec la consommation CPU et mémoire en direct, ainsi que l'état en ligne/hors ligne. Un worker en arrière-plan actualise les métriques des nœuds selon l'intervalle configuré par `PVMSS_INVENTORY_REFRESH_INTERVAL` ; les pages d'administration lisent ce cache local, la navigation reste donc instantanée même sur de gros clusters. Chaque carte de nœud expose aussi un bouton d'actualisation dédié et l'horodatage de la dernière mise à jour. Lorsque Proxmox signale un nœud hors ligne, PVMSS continue d'afficher les dernières valeurs connues tout en marquant clairement le nœud comme hors ligne.

## Clusters (multi-cluster)

PVMSS prend en charge la connexion simultanée à plusieurs environnements Proxmox. Chaque connexion est un **cluster** avec sa propre URL, son propre jeton API et éventuellement un fournisseur OIDC.

- Ouvrez **Admin > Clusters** (`/admin/clusters`) pour ajouter, éditer, tester et supprimer des connexions de cluster.
- Un cluster est identifié par un nom ; les VMs sont toujours adressées par leur `cluster` et leur `VMID`, donc deux clusters peuvent réutiliser les mêmes VMID sans conflit.
- Utilisez l'action **Tester** pour vérifier la connectivité et les identifiants avant d'exposer le cluster aux utilisateurs.
- Lorsque le Proxmox d'un cluster prend en charge OIDC, vous pouvez **activer OIDC** sur ce cluster afin que ses utilisateurs puissent se connecter via le fournisseur d'identité du cluster depuis l'écran de connexion.
- Les nœuds, stockages, ISO, ponts et tags approuvés sont gérés par cluster ; les surfaces du catalogue vous permettent de limiter ce que les utilisateurs voient sur chaque cluster.

La page **Informations sur l'application** indique le(s) cluster(s) connecté(s), le nom et le nombre de nœuds, ce qui permet de confirmer la topologie attendue.

## Catalogue : ressources exposées aux utilisateurs

La zone **Catalogue** de la navigation administrateur contrôle ce que la création de VM peut référencer. Les ressources découvertes apparaissent automatiquement ici ; basculez l'interrupteur pour contrôler ce que les utilisateurs voient.

- **Stockages** (`/admin/storages`) — approuvez les backends de stockage pouvant héberger les disques de VM, regroupés par nœud en mode cluster.
- **ISO** (`/admin/isos`) — approuvez les images ISO que les utilisateurs peuvent démarrer.
- **Ponts** (`/admin/bridges`) — approuvez les ponts réseau (VMBR) disponibles pour les cartes réseau des VMs. Les ponts Open vSwitch ne sont pas listés.
- **Modèles cloud-init** (`/admin/cloudinit-templates`) — créez, activez, désactivez et éditez des modèles `#cloud-config` gérés par les administrateurs, que les utilisateurs peuvent choisir à la création.
- **Profils** (`/admin/profiles`) — définissez des profils matériels pré-approuvés (CPU, mémoire, disque, bus) pour que les utilisateurs choisissent une configuration connue au lieu de saisir des valeurs libres.
- **Tags** (`/admin/tags`) — gérez les libellés que les utilisateurs peuvent attacher aux VMs pour le filtrage et la recherche. Un tag est immuable une fois créé ; le tag `pvmss` est réservé et ne peut pas être supprimé.

## Considérations réseau

Les utilisateurs finaux peuvent spécifier par carte réseau, dans les pages Créer une VM et Modifier les ressources :

- **Vitesse réseau** (Proxmox `rate`, en Mo/s) : bornée entre 1 et 10240 Mo/s. Laisser vide donne une vitesse illimitée.
- **Tag VLAN** (1-4096) : ajouté à l'interface sous la forme `,tag=X`. Assurez-vous que vos commutateurs physiques et vos ponts Proxmox sont configurés pour les ID VLAN autorisés.
- **MTU** (576-9000 octets) : laisser vide utilise la valeur par défaut Proxmox de 1500. N'utilisez des MTU personnalisés que sur des réseaux soigneusement contrôlés.

En tant qu'administrateur, documentez les ID VLAN que vos utilisateurs doivent utiliser et surveillez les journaux de création pour les problèmes liés au VLAN.

## Pools utilisateurs

`/admin/pools` est l'endroit où vous créez les utilisateurs en libre-service. Chaque pool provisionne :

- un utilisateur Proxmox dédié,
- un pool Proxmox dédié nommé d'après l'utilisateur,
- et une ACL liant l'utilisateur au rôle partagé `PVMSSUser` sur ce pool.

PVMSS impose un motif de nom de pool (1-32 caractères alphanumériques minuscules avec tirets internes) et une longueur minimale de mot de passe de 8 caractères. Les utilisateurs ne voient que les VMs de leur propre pool.

## Politique (limites)

`/admin/policy` définit les quotas par utilisateur : nombre maximal de VMs, CPU, mémoire et disque. `/admin/policy/nodes` plafonne la part des ressources d'un nœud qu'une seule VM peut consommer. Les deux sont appliqués côté serveur avant tout appel Proxmox, donc les requêtes au-dessus du quota ou de la capacité du nœud sont rejetées tôt. Les limites de snapshots (nombre maximal par VM) sont aussi appliquées via la politique.

## Documentation (ce CMS)

Cette page est l'une des plusieurs gérées sous **Documentation** (`/admin/docs`). Les administrateurs peuvent créer, éditer, activer, désactiver et supprimer des pages Markdown ici. Les pages intégrées sont marquées **système** et ne peuvent pas être supprimées, mais leur contenu peut être édité. Chaque page a un public `user` (public) ou `admin` (réservé aux administrateurs) ; les pages réservées aux administrateurs sont masquées aux non-administrateurs dans la liste publique des docs.

## Paramètres, audit et maintenance

`/admin/settings` expose des contrôles opérationnels :

- **Journal d'audit** — chaque écriture de VM (création, démarrage, arrêt, édition, suppression, changement cloud-init) est enregistrée avec l'utilisateur responsable, ce qui permet de retracer l'activité.
- **Export / import de base de données** — sauvegardez ou restaurez la base SQLite utilisée par PVMSS pour toute sa configuration et son historique d'audit.

## Recommandations de sécurité

- Exposez PVMSS uniquement en HTTPS (généralement derrière un reverse proxy) et restreignez l'accès aux réseaux de confiance.
- Utilisez des comptes Proxmox dédiés pour PVMSS ; évitez de partager le mot de passe administrateur intégré.
- Gardez les permissions Proxmox simples : un jeton API de service avec le rôle `PVMSS_Service` pour le backend, des comptes administrateurs humains avec le rôle `PVMSS_Admin`, et des utilisateurs finaux confinés à leur pool en rôle `PVMSSUser`. Voir `/docs/proxmox-permissions` pour les commandes `pveum` exactes.
- N'attribuez pas de privilèges Proxmox étendus aux utilisateurs de libre-service habituels.
- Passez régulièrement en revue les pools utilisateurs et désactivez ou supprimez les comptes inutilisés.

## Limites connues

- PVMSS cible les clusters (et nœuds autonomes) Proxmox VE 8.x/9.x.
- Il n'y a pas d'intégration d'authentification externe (OIDC/SAML) câblée dans les comptes PVMSS au-delà de ce que l'écran de connexion peut proposer ; les comptes utilisateurs sont provisionnés via `/admin/pools` côté Proxmox.
- PVMSS prend en charge les serveurs autonomes et les clusters, mais les opérations cluster avancées (migration à chaud, HA, orchestration des sauvegardes) se font directement dans Proxmox.
- Les administrateurs ne peuvent pas créer de VMs depuis l'interface d'administration ; la création de VMs se fait via l'interface utilisateur en libre-service ou directement dans Proxmox.
- Les sauvegardes et les conteneurs LXC sont gérés dans Proxmox, pas dans PVMSS.

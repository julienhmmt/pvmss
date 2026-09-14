# Guide administrateur

Bienvenue dans le guide administrateur de PVMSS. PVMSS (Proxmox Virtual
Machine Self-Service) est un portail en libre-service qui permet à vos
utilisateurs de créer, exploiter et dépanner des machines virtuelles Proxmox
VE sans exposer l'interface Proxmox.

L'administrateur a accès à toutes les fonctionnalités. Il n'existe pas de rôle
auditeur ou observateur distinct : se connecter avec un compte administrateur
débloque à la fois la surface utilisateur standard et tout ce qui se trouve
sous `/admin`.

## Premiers pas

1. Ouvrez le panneau d'administration sur `/admin` (compte administrateur requis).
2. Vérifiez **Infos application** (`/admin/appinfo`) pour confirmer que l'instance est connectée au bon environnement Proxmox.
3. Ajoutez ou vérifiez vos **Clusters** (`/admin/clusters`) et lancez le **Test** de connexion.
4. Approuvez les ressources utilisables : **Nœuds**, **Stockages**, **ISO**, **Templates de VM**, **Images cloud** et **Bridges**.
5. Optionnel mais recommandé : créez des **profils de VM** pour proposer des gabarits matériels pré-approuvés, définissez des **Tags** et rédigez des **templates cloud-init**.
6. Réglez la **Politique** (gabarit et quota par cluster) et la **Capacité des nœuds**.
7. Activez éventuellement les **documents cloud-init** (voir plus bas).
8. Créez autant de **pools utilisateurs** que nécessaire depuis `/admin/pools`.
9. Informez vos utilisateurs que le portail est disponible.
10. Surveillez le **journal d'audit** (`/admin/settings`) pour retracer chaque écriture jusqu'à l'utilisateur qui l'a faite.

## Tableau de bord

`/admin` résume le cluster : état et charge des nœuds, nombre de VM par état,
version en cours et heure du dernier rafraîchissement d'inventaire.

## Informations de l'application (App Info)

`/admin/appinfo` est une vue en lecture seule de l'instance :

- **Informations de build** : version de l'application, version de Go, système d'exploitation et architecture.
- **Environnement** : PVMSS tourne-t-il sur un vrai cluster Proxmox (`PVMSS_CLUSTER_SOURCE=proxmox`) ou sur le cluster de démonstration intégré (`PVMSS_CLUSTER_SOURCE=fake`).
- **État du cluster Proxmox** : nom du cluster et nombre de nœuds, ou mode autonome pour un nœud seul.
- **Variables d'environnement (sous-ensemble sûr)** : configuration non sensible comme `PROXMOX_URL`, `PVMSS_PORT`, `PVMSS_DB_PATH`.

Utilisez cette page pour confirmer la connectivité après un déploiement ou un changement de configuration. Si l'instance ne joint pas Proxmox, consultez les logs du serveur et les variables ci-dessous.

## Configuration (variables d'environnement)

PVMSS se configure entièrement par variables d'environnement, validées au démarrage. Le serveur refuse de démarrer si une valeur requise manque ou est mal formée.

Requises :

- `PVMSS_PORT` — port TCP d'écoute (l'image officielle utilise `50000`).
- `PVMSS_DB_PATH` — chemin du fichier SQLite.
- `SESSION_SECRET` — 32 octets minimum, chiffre les sessions.
- `LOG_LEVEL` — `debug`, `info`, `warn` ou `error` (minuscules uniquement).
- `LOG_FORMAT` — `json` ou `console`.
- `LOG_OUTPUT` — `stdout`, `stderr` ou un chemin de fichier.
- `PVMSS_CLUSTER_SOURCE` — `proxmox` ou `fake`. Pas de valeur par défaut, volontairement : `fake` embarque des identifiants de démonstration et ne doit jamais être choisi par accident.

Requises quand `PVMSS_CLUSTER_SOURCE=proxmox` :

- `PROXMOX_URL` — par exemple `https://hote:8006/api2/json`.
- `PROXMOX_API_TOKEN_NAME` — identifiant du token d'API Proxmox (`user@pve!token`).
- `PROXMOX_API_TOKEN_VALUE` — le secret correspondant.

Ces trois variables décrivent le premier cluster ; les clusters supplémentaires s'ajoutent depuis `/admin/clusters`.

Optionnelles :

- `PVMSS_HOST` — adresse d'écoute (l'image met `0.0.0.0`).
- `PVMSS_WEB_DIR` — emplacement de la SPA compilée (par défaut relatif à l'exécutable).
- `ADMIN_PASSWORD_HASH` — si défini, doit être un hash bcrypt (`$2…`) ; active la connexion administrateur locale.
- `PVMSS_COOKIE_SECURE` — `true` par défaut ; `false` uniquement derrière du HTTP en clair pour un essai local.
- `PVMSS_TRUSTED_PROXY_HOPS` — nombre de reverse proxies devant PVMSS, pour déduire l'IP client (limitation de débit, audit) — défaut `1`.
- `PVMSS_INVENTORY_REFRESH_INTERVAL` — période de rafraîchissement d'inventaire (défaut `30s`).
- `PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL` — délai minimal entre deux rafraîchissements manuels (défaut `5s`).
- `PVMSS_INVENTORY_REFRESH_TIMEOUT` — délai maximal d'un rafraîchissement (défaut `15s`).
- `PVMSS_MAX_LIST_PAGE_SIZE` — taille de page maximale des listes (défaut `100`).

Pour les instructions de déploiement complètes (Docker, Kubernetes, Helm), voir le README du projet.

## Clusters (multi-cluster)

PVMSS peut se connecter à plusieurs environnements Proxmox simultanément. Chaque connexion est un **cluster** avec son URL, son token d'API et ses options.

- Ouvrez **Admin > Clusters** (`/admin/clusters`) pour ajouter, modifier, tester et retirer des connexions.
- Un cluster est identifié par un nom ; les VM sont toujours adressées par `cluster` + `VMID`, deux clusters peuvent donc réutiliser les mêmes VMID sans conflit.
- Le **Test** vérifie la connectivité et les identifiants avant d'exposer le cluster ; il rapporte la version de Proxmox et le nombre de nœuds et de VM.
- La **vérification TLS** peut être désactivée par cluster pour un labo auto-signé ; gardez-la active en production.
- L'interrupteur **OIDC** est réservé à une future intégration SSO : l'activer affiche un bouton sur l'écran de connexion mais la connexion n'est pas encore implémentée.
- **Répertoire de snippets** et **Stockage de snippets** activent les documents cloud-init pour ce cluster (voir la section dédiée). Le badge du cluster affiche « cloud-init : activé » une fois les deux renseignés.
- Nœuds, stockages, ISO, images, templates, bridges, templates cloud-init et politique sont gérés par cluster.

## Nœuds

`/admin/nodes` liste chaque hôte Proxmox VE par cluster avec la consommation CPU et mémoire en direct, le nombre de VM et l'état en ligne / hors ligne. Recherchez, filtrez par état ou activation, triez la table. Basculez un nœud pour l'approuver ou le masquer à la création de VM ; désactiver un nœud qui héberge des VM demande confirmation et ne touche jamais ces VM. Un travail de fond rafraîchit les métriques selon `PVMSS_INVENTORY_REFRESH_INTERVAL` ; les pages admin lisent ce cache, la navigation reste instantanée même sur de gros clusters.

## Catalogue : ressources exposées aux utilisateurs

La zone **Catalogue** contrôle ce que la création de VM peut référencer. Les ressources découvertes apparaissent automatiquement ; l'interrupteur « activé » contrôle ce que voient les utilisateurs. Chaque page du catalogue a un sélecteur de cluster, une recherche, des filtres et des colonnes triables.

- **Stockages** (`/admin/storages`) — approuver les stockages pouvant héberger des disques de VM, regroupés par nœud, avec barres d'usage.
- **ISO** (`/admin/isos`) — approuver les images ISO de démarrage.
- **Templates de VM** (`/admin/templates`) — approuver les templates Proxmox clonables, avec surcharges optionnelles par template. Un clone reste sur le nœud du template ; l'assistant prévient quand le stockage cible impose une copie complète.
- **Images cloud** (`/admin/images`) — approuver les images cloud découvertes dans le contenu `import/` d'un stockage. PVMSS ne télécharge jamais d'image depuis Internet : déposez-les vous-même sur le stockage.
- **Bridges** (`/admin/bridges`) — approuver les bridges réseau (VMBR) pour les cartes réseau. Les bridges Open vSwitch ne sont pas listés.
- **Templates cloud-init** (`/admin/cloudinit-templates`) — créer, activer, désactiver et modifier des documents `#cloud-config` que les utilisateurs peuvent choisir à la création.
- **Profils** (`/admin/profiles`) — définir des profils matériels pré-approuvés (sockets, cœurs, mémoire, disque, bus) avec surcharges nœud/stockage optionnelles, une icône et une couleur.
- **Tags** (`/admin/tags`) — gérer les étiquettes attachables aux VM, chacune avec une couleur. Un tag est immuable une fois créé (seule sa couleur change) ; le tag `pvmss` est réservé et ne peut pas être supprimé.

### Approbations obsolètes

Les approbations sont réconciliées avec la découverte en direct. Quand une ressource (nœud, stockage, ISO, image, template, bridge) disparaît de Proxmox, sa ligne est signalée comme manquante et propose une action **Retirer**, pour que le catalogue ne référence jamais quelque chose qui n'existe plus.

## Considérations réseau

À la création, les utilisateurs choisissent un bridge et un modèle de carte par NIC ; le pare-feu Proxmox par VM est toujours activé. Après création, ils peuvent modifier chaque NIC dans l'onglet Réseau de la VM :

- **Débit** (`rate` Proxmox, en Mbit/s) : vide = illimité.
- **Tag VLAN** (1-4094) : ajouté à l'interface en `,tag=X`. Assurez-vous que vos commutateurs physiques et bridges Proxmox acceptent les VLAN autorisés.

Le **tag VLAN d'isolation** de la politique applique un VLAN à chaque NIC créée via PVMSS sur ce cluster ; laissez 0 pour désactiver.

## Pools utilisateurs

`/admin/pools` crée les utilisateurs self-service. Chaque pool provisionne :

- un utilisateur Proxmox dédié,
- un pool Proxmox dédié à cet utilisateur,
- une ACL liant l'utilisateur au rôle partagé `PVMSSUser` sur ce pool.

Vous saisissez un nom court (1 à 32 caractères alphanumériques minuscules avec tirets internes) ; PVMSS le préfixe en `pvmss-`, donc le pool Proxmox est `pvmss-<nom>` et l'utilisateur `pvmss-<nom>@pve`. Le mot de passe de connexion est généré, affiché une seule fois dans la réponse de création, et jamais stocké — communiquez-le à l'utilisateur de façon sécurisée. Les utilisateurs ne voient que les VM de leur propre pool. Supprimer un pool supprime aussi l'utilisateur et l'ACL Proxmox ; les pools non créés par PVMSS sont refusés.

## Politique (limites)

`/admin/policy` est par cluster et comporte deux parties :

- **Gabarit** — le plafond d'une VM : sockets, cœurs, mémoire, disque par VM, cartes réseau, snapshots, autorisation du YAML cloud-init libre sur les VM, et tag VLAN d'isolation.
- **Quota** — nombre maximal de VM par utilisateur.

`/admin/policy/nodes` plafonne ce que PVMSS peut allouer au total sur un nœud (VM, vCPU, RAM, disque) et montre l'usage courant face à la capacité physique. Tout est appliqué côté serveur avant tout appel à Proxmox : une demande au-delà d'une limite est refusée tôt avec un message clair.

## Limites de la plateforme

Au-delà des réglages de la politique, l'application elle-même impose :

- **Limitation de débit** — 10 requêtes/minute par IP sur les endpoints d'authentification ; 30 écritures/minute par utilisateur sur les routes VM ; 120 lectures de statut/minute par utilisateur ; 60 écritures/minute par utilisateur sur les routes admin ; 10 tests de cluster/minute.
- **Actions groupées** — 100 VM maximum par requête.
- **Fichiers cloud-init** — 20 documents maximum par utilisateur.
- **Nom de VM** — un nom d'hôte en minuscules, 63 caractères maximum, unique dans le pool du propriétaire ; **description** — 512 caractères maximum.
- **Nom de snapshot** — une lettre puis lettres, chiffres, tirets ou underscores, 2 à 40 caractères ; `current` est réservé.
- **Nom de pool** — 1 à 32 caractères alphanumériques minuscules avec tirets internes (stocké en `pvmss-<nom>`).
- **Listes** — au plus `PVMSS_MAX_LIST_PAGE_SIZE` entrées par page (défaut 100).

## Activer les documents cloud-init

L'API REST de Proxmox ne sait pas écrire de fichiers `snippets` ; PVMSS les écrit donc lui-même dans un répertoire que vous montez dans son conteneur. Une fois activé, les utilisateurs peuvent attacher un template admin ou l'un de leurs fichiers (page `/cloud-init`, 20 par utilisateur) à une nouvelle VM ; PVMSS écrit une copie propre à la VM `pvmss-<vmid>.yml`, l'attache en vendor data et l'enregistre. Modifier la source ensuite ne touche jamais les VM existantes.

1. Dans Proxmox, choisissez un stockage **partagé par tous les nœuds** (NFS/CIFS). Datacenter › Storage › Edit → Content : ajoutez **Snippets**.
2. Montez `<chemin du stockage>/snippets` dans le conteneur PVMSS. Compose : `- /mnt/pve/shared/snippets:/snippets`. Helm : `persistence.snippets.enabled=true` avec `existingClaim` ou `nfs.server` + `nfs.path`. Kubernetes brut : le volume `snippets` commenté dans `pvmss-deployment.yaml`.
3. **Admin › Clusters › Modifier** : *Répertoire de snippets* = le chemin dans le conteneur (`/snippets`), *Stockage de snippets* = l'identifiant du stockage Proxmox. Le badge passe à « cloud-init : activé ».
4. Vérifiez : créez un template cloud-init, créez une VM avec, puis sur un nœud lancez `qm config <vmid> | grep cicustom` et contrôlez le fichier dans `snippets/`.
5. Un stockage de type répertoire local à un nœud ne fonctionne que si toutes les VM sont placées sur ce nœud — déconseillé.
6. Activez **Autoriser le YAML cloud-init libre** dans la politique si les utilisateurs peuvent modifier le document de leurs VM depuis l'onglet Cloud-init.

Sans cible d'écriture, l'assistant masque le sélecteur et une création portant un document est refusée avec `cloudinit_write_unavailable`, avant qu'un VMID ne soit consommé.

Les VM créées depuis une image cloud reçoivent en plus un snippet de base fixe, `pvmss-baseline.yml`, si vous en déposez un dans le même répertoire `snippets/` (par exemple pour installer `qemu-guest-agent`) ; son absence est silencieuse.

Supprimer une VM via PVMSS retire aussi son `pvmss-<vmid>.yml` (au mieux — un échec de nettoyage est journalisé et ne bloque jamais la suppression). Les VM supprimées directement dans Proxmox laissent leur fichier ; listez les orphelins sur un nœud avec `ls /mnt/pve/<stockage>/snippets/pvmss-*.yml` à comparer avec `qm list`.

## Documentation (ce CMS)

Cette page fait partie de celles gérées sous **Documentation** (`/admin/docs`). Les administrateurs peuvent créer, modifier, activer/désactiver et supprimer des pages Markdown en anglais et en français. Les pages intégrées sont marquées **système** et ne peuvent pas être supprimées, mais leur contenu est modifiable. Chaque page a une audience `user` (publique) ou `admin` (administrateurs) ; les pages admin sont masquées aux non-administrateurs et refusées en accès direct.

Les pages intégrées sont insérées une seule fois, si elles manquent, et jamais écrasées au redémarrage : vos modifications survivent aux mises à jour. Pour récupérer un texte intégré plus récent, supprimez la page et redémarrez, ou collez le nouveau contenu depuis la release.

## Paramètres, audit et maintenance

`/admin/settings` expose les commandes d'exploitation :

- **Journal d'audit** — chaque écriture (création de VM, action d'alimentation, modification, suppression, changement cloud-init, changements de catalogue et de politique, connexions) est enregistrée avec l'utilisateur, l'IP, la sévérité et la cible. Filtrez et paginez ; chaque VM affiche aussi ses propres entrées dans son onglet Activité.
- **Rétention** — nombre de jours de conservation des lignes d'audit, avec aperçu du nombre de lignes qu'une purge supprimerait avant de l'appliquer.
- **Export / import de la base** — sauvegardez la base SQLite ou restaurez-en une. L'import est en deux temps : l'envoi renvoie un aperçu table par table, rien n'est écrit avant confirmation.

## Recommandations de sécurité

- Servez PVMSS uniquement en HTTPS (derrière un reverse proxy) et restreignez l'accès aux réseaux de confiance. Réglez `PVMSS_TRUSTED_PROXY_HOPS` au nombre de proxies pour que la limitation de débit et l'audit voient la vraie IP client.
- Utilisez des comptes Proxmox dédiés à PVMSS ; ne partagez pas le mot de passe administrateur intégré.
- Gardez des permissions Proxmox simples : un token de service avec le rôle `PVMSS_Service` pour le backend, des administrateurs humains avec le rôle `PVMSS_Admin`, et des utilisateurs confinés à leur pool `PVMSSUser`. Voir `/docs/proxmox-permissions` pour les commandes `pveum` exactes.
- N'accordez pas de privilèges Proxmox étendus aux utilisateurs self-service.
- Passez régulièrement en revue les pools et désactivez ou supprimez les comptes inutilisés.
- Les documents cloud-init sont stockés en clair ; rappelez aux utilisateurs de n'y mettre aucun secret.

## Limitations connues

- PVMSS cible les clusters Proxmox VE 8.x/9.x (et les nœuds autonomes).
- La connexion OIDC/SSO n'est pas encore implémentée ; les comptes sont provisionnés via `/admin/pools` côté Proxmox.
- Les opérations avancées de cluster (migration à chaud, HA, orchestration des sauvegardes) se font directement dans Proxmox.
- Les administrateurs créent des VM via la même interface self-service que les utilisateurs, ou directement dans Proxmox.
- Sauvegardes et conteneurs LXC sont gérés dans Proxmox, pas dans PVMSS.
- Le changement de mot de passe n'est disponible que par API pour l'instant (`POST /api/v1/auth/password`).
- Les tokens API personnels sont désactivés dans cette version : leurs routes ne sont pas enregistrées, la page des tokens ne peut ni créer ni lister de jetons.

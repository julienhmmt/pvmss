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
4. Approuvez les ressources utilisables : **Nœuds**, **Stockages**, **ISO**, **Templates de VM** et **Bridges**. Préparez chaque template avec les accès dont les utilisateurs auront besoin (voir **Les templates de VM portent les accès** plus bas).
5. Optionnel mais recommandé : créez des **profils de VM** pour proposer des gabarits matériels pré-approuvés et définissez des **Tags**.
6. Réglez la **Politique** (gabarit et quota par cluster) et la **Capacité des nœuds**.
7. Créez autant d'**utilisateurs PVMSS** que nécessaire depuis `/admin/pools`.
8. Informez vos utilisateurs que le portail est disponible.
9. Surveillez le **journal d'audit** (`/admin/settings`) pour retracer chaque écriture jusqu'à l'utilisateur qui l'a faite.

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

- `PVMSS_PORT` - port TCP d'écoute (l'image officielle utilise `50000`).
- `PVMSS_DB_PATH` - chemin du fichier SQLite.
- `SESSION_SECRET` - 32 octets minimum, chiffre les sessions.
- `LOG_LEVEL` - `debug`, `info`, `warn` ou `error` (minuscules uniquement).
- `LOG_FORMAT` - `json` ou `console`.
- `LOG_OUTPUT` - `stdout`, `stderr` ou un chemin de fichier.
- `PVMSS_CLUSTER_SOURCE` - `proxmox` ou `fake`. Pas de valeur par défaut, volontairement : `fake` embarque des identifiants de démonstration et ne doit jamais être choisi par accident.

Requises quand `PVMSS_CLUSTER_SOURCE=proxmox` :

- `PROXMOX_URL` - par exemple `https://hote:8006/api2/json`.
- `PROXMOX_API_TOKEN_NAME` - identifiant du token d'API Proxmox (`user@pve!token`).
- `PROXMOX_API_TOKEN_VALUE` - le secret correspondant.

Ces trois variables décrivent le premier cluster ; les clusters supplémentaires s'ajoutent depuis `/admin/clusters`.

Optionnelles :

- `PVMSS_HOST` - adresse d'écoute (l'image met `0.0.0.0`).
- `PVMSS_WEB_DIR` - emplacement de la SPA compilée (par défaut relatif à l'exécutable).
- `ADMIN_PASSWORD_HASH` - si défini, doit être un hash bcrypt (`$2…`) ; initialise l'administrateur bootstrap de PVMSS, le seul compte autorisé à gérer les autres administrateurs locaux. Son mot de passe se change en modifiant cette valeur puis en redémarrant.
- `PVMSS_COOKIE_SECURE` - `true` par défaut ; `false` uniquement derrière du HTTP en clair pour un essai local.
- `PVMSS_TRUSTED_PROXY_HOPS` - nombre de reverse proxies devant PVMSS, pour déduire l'IP client (limitation de débit, audit) - défaut `1`.
- `PVMSS_INVENTORY_REFRESH_INTERVAL` - période de rafraîchissement d'inventaire (défaut `30s`).
- `PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL` - délai minimal entre deux rafraîchissements manuels (défaut `5s`).
- `PVMSS_INVENTORY_REFRESH_TIMEOUT` - délai maximal d'un rafraîchissement (défaut `15s`).
- `PVMSS_MAX_LIST_PAGE_SIZE` - taille de page maximale des listes (défaut `100`).
- `PVMSS_MAX_LOCAL_ADMINS` - nombre maximal de comptes administrateurs locaux en base, de 1 à 50 (défaut `5`). L'administrateur bootstrap n'est pas compté.

Pour les instructions de déploiement complètes (Docker, Kubernetes, Helm), voir le README du projet.

## Administrateurs et comptes locaux

Trois voies donnent les droits d'administration, et toutes mènent à la même interface :

- **Administrateur Proxmox** - connectez-vous avec un compte Proxmox dont l'ACL accorde `Permissions.Modify` sur `/` (rôle `PVEAdmin`, `Administrator` ou un rôle personnalisé). Choisissez le realm `pve` ou `pam`, ou saisissez `user@realm` ; les realms OIDC ne sont pas encore pris en charge et répondent `realm_unsupported`. Le privilège est lu une fois à la connexion : accordez-le avant que l'utilisateur se connecte.
- **Administrateur local bootstrap** - le compte `admin` initialisé par `ADMIN_PASSWORD_HASH`. Il est immuable, n'est jamais verrouillé et n'entre pas dans `PVMSS_MAX_LOCAL_ADMINS`.
- **Administrateurs locaux en base** - comptes stockés dans PVMSS et créés par l'administrateur bootstrap dans **Paramètres > Comptes**. Ils fonctionnent sans aucune connexion à Proxmox.

Seul l'administrateur bootstrap peut gérer les comptes locaux ; tout autre administrateur reçoit `owner_only` de `/api/v1/admin/accounts`. Depuis l'onglet Comptes, vous pouvez créer un compte (3 à 32 caractères minuscules, chiffres, point, tiret bas ou tiret), réinitialiser son mot de passe, le désactiver ou l'activer, et le supprimer. Chacune de ces actions révoque les sessions du compte. Les mots de passe doivent contenir au moins 8 caractères et au plus 72 octets (limite bcrypt). Cinq échecs consécutifs verrouillent un compte pendant 15 minutes ; une connexion réussie, une réinitialisation ou une désactivation lève le verrou. Chaque modification est écrite dans le journal d'audit.

Les administrateurs locaux en base changent leur propre mot de passe sur `/profile`, comme les utilisateurs Proxmox. Le compte bootstrap ne le peut pas : changez `ADMIN_PASSWORD_HASH` puis redémarrez.

## Clusters (multi-cluster)

PVMSS peut se connecter à plusieurs environnements Proxmox simultanément. Chaque connexion est un **cluster** avec son URL, son token d'API et ses options.

- Ouvrez **Infrastructure > Clusters** (`/admin/clusters`) pour ajouter, modifier, tester et retirer des connexions.
- Un cluster est identifié par un nom ; les VM sont toujours adressées par `cluster` + `VMID`, deux clusters peuvent donc réutiliser les mêmes VMID sans conflit.
- Le **Test** vérifie la connectivité et les identifiants avant d'exposer le cluster ; il rapporte la version de Proxmox et le nombre de nœuds et de VM.
- La **vérification TLS** se règle par cluster. Gardez-la active en production : ajoutez le certificat CA du cluster (`/etc/pve/pve-root-ca.pem`) ou épinglez les empreintes SHA-256 des nœuds dans la section « Certificats TLS » du cluster, puis lancez **Test** pour voir l'état TLS. Ne la désactivez que pour un labo auto-signé. Un certificat client et sa clé (mTLS) peuvent aussi être enregistrés ; la clé est chiffrée et n'est plus jamais affichée.
- Nœuds, stockages, ISO, templates, bridges et politique sont gérés par cluster.

## Nœuds

`/admin/nodes` liste chaque hôte Proxmox VE par cluster avec la consommation CPU et mémoire en direct, le nombre de VM et l'état en ligne / hors ligne. Recherchez, filtrez par état ou activation, triez la table. Basculez un nœud pour l'approuver ou le masquer à la création de VM ; désactiver un nœud qui héberge des VM demande confirmation et ne touche jamais ces VM. Un travail de fond rafraîchit les métriques selon `PVMSS_INVENTORY_REFRESH_INTERVAL` ; les pages admin lisent ce cache, la navigation reste instantanée même sur de gros clusters.

## Ressources et provisionnement : ce que les utilisateurs peuvent choisir

Les groupes **Ressources** et **Provisionnement** du menu admin contrôlent ce que la création de VM peut référencer. Les ressources découvertes apparaissent automatiquement ; l'interrupteur « activé » contrôle ce que voient les utilisateurs. Chaque page du catalogue a un sélecteur de cluster, une recherche, des filtres et des colonnes triables.

- **Stockages** (`/admin/storages`) - approuver les stockages pouvant héberger des disques de VM, regroupés par nœud, avec barres d'usage. Un stockage partagé (Ceph, NFS, CephFS) est approuvé une seule fois par cluster.
- **ISO** (`/admin/isos`) - approuver les images ISO de démarrage.
- **Templates de VM** (`/admin/templates`) - approuver les templates Proxmox clonables, avec surcharges optionnelles par template. Un clone reste sur le nœud du template ; l'assistant prévient quand le stockage cible impose une copie complète. Un clone garde les accès du template : PVMSS n'en ajoute aucun.
- **Bridges** (`/admin/bridges`) - approuver les bridges réseau (VMBR) pour les cartes réseau. Les bridges Open vSwitch ne sont pas listés.
- **Profils** (`/admin/profiles`) - définir des profils matériels pré-approuvés (sockets, cœurs, mémoire, disque, bus).
- **Tags** (`/admin/tags`) - gérer les étiquettes attachables aux VM, chacune avec une couleur. Un tag est immuable une fois créé (seule sa couleur change) ; le tag `pvmss` est réservé et ne peut pas être supprimé.

### Approbations obsolètes

Les approbations sont réconciliées avec la découverte en direct. Quand une ressource (nœud, stockage, ISO, image, template, bridge) disparaît de Proxmox, sa ligne est signalée comme manquante et propose une action **Retirer**, pour que le catalogue ne référence jamais quelque chose qui n'existe plus.

## Considérations réseau

À la création, les utilisateurs choisissent un bridge et un modèle de carte par NIC ; le pare-feu Proxmox par VM est toujours activé. Après création, ils peuvent modifier chaque NIC dans l'onglet Réseau de la VM :

- **Débit** (`rate` Proxmox, en Mbit/s) : vide = illimité.
- **Tag VLAN** (1-4094) : ajouté à l'interface en `,tag=X`. Assurez-vous que vos commutateurs physiques et bridges Proxmox acceptent les VLAN autorisés.

Le **tag VLAN d'isolation** de la politique applique un VLAN à chaque NIC créée via PVMSS sur ce cluster ; laissez 0 pour désactiver.

## Utilisateurs PVMSS

**Utilisateurs PVMSS** (`/admin/pools`) crée les utilisateurs self-service. Chaque pool provisionne :

- un utilisateur Proxmox dédié,
- un pool Proxmox dédié à cet utilisateur,
- une ACL liant l'utilisateur au rôle partagé `PVMSSUser` sur ce pool.

Vous saisissez un nom court (1 à 32 caractères alphanumériques minuscules avec tirets internes) ; PVMSS le préfixe en `pvmss-`, donc le pool Proxmox est `pvmss-<nom>` et l'utilisateur `pvmss-<nom>@pve`. Le mot de passe de connexion est généré, affiché une seule fois dans la réponse de création, et jamais stocké - communiquez-le à l'utilisateur de façon sécurisée. Les utilisateurs ne voient que les VM de leur propre pool. Supprimer un pool supprime aussi l'utilisateur et l'ACL Proxmox ; les pools non créés par PVMSS sont refusés.

## Politique (limites)

`/admin/policy` est par cluster et comporte deux parties :

- **Gabarit** - le plafond d'une VM : sockets, cœurs, mémoire, disque par VM, cartes réseau, snapshots, et tag VLAN d'isolation.
- **Quota** - nombre maximal de VM par utilisateur.
- **Plage de VMID** - premier et dernier VMID optionnels pour les nouvelles VM de ce cluster (0 et 0 = prochain identifiant libre de Proxmox). Le plus petit identifiant libre de la plage est utilisé, en évitant ceux déjà pris sur le cluster.

`/admin/policy/nodes` plafonne ce que PVMSS peut allouer au total sur un nœud (VM, vCPU, RAM, disque) et montre l'usage courant face à la capacité physique. Tout est appliqué côté serveur avant tout appel à Proxmox : une demande au-delà d'une limite est refusée tôt avec un message clair.

## Limites de la plateforme

Au-delà des réglages de la politique, l'application elle-même impose :

- **Limitation de débit** - 10 requêtes/minute par IP sur les endpoints d'authentification ; 30 écritures/minute par utilisateur sur les routes VM ; 120 lectures de statut/minute par utilisateur ; 60 écritures/minute par utilisateur sur les routes admin ; 10 tests de cluster/minute.
- **Actions groupées** - 100 VM maximum par requête.
- **Nom de VM** - un nom d'hôte en minuscules, 63 caractères maximum, unique dans le pool du propriétaire ; **description** - 512 caractères maximum.
- **Nom de snapshot** - une lettre puis lettres, chiffres, tirets ou underscores, 2 à 40 caractères ; `current` est réservé.
- **Nom de pool** - 1 à 32 caractères alphanumériques minuscules avec tirets internes (stocké en `pvmss-<nom>`).
- **Listes** - au plus `PVMSS_MAX_LIST_PAGE_SIZE` entrées par page (défaut 100).

## Les templates de VM portent les accès

PVMSS n'ajoute ni clé SSH ni mot de passe aux VM qu'il crée : cloud-init
(clés par VM, templates cloud-init administrateur, images cloud) est en pause
tant qu'il n'est pas fiable. Un clone garde exactement ce que porte son
template Proxmox : préparez donc chaque template approuvé avec les accès dont
les utilisateurs auront besoin :

1. Construisez le template dans Proxmox (par exemple depuis une image cloud :
   importez le disque, ajoutez un lecteur Cloud-Init).
2. Dans l'onglet **Cloud-Init** du template, renseignez l'utilisateur et une
   clé SSH publique ou un mot de passe - ou intégrez un compte directement
   dans l'image.
3. Installez et activez `qemu-guest-agent`, pour que les VM signalent leur
   adresse et que l'onglet **Connexion** affiche la commande SSH.
4. Convertissez-le (`qm template <vmid>`) et approuvez-le dans **Templates de
   VM**.

Toutes les VM clonées d'un même template partagent ces accès : transmettez-les
aux utilisateurs autorisés à utiliser ce template, ou gardez un template par
équipe. Les installations ISO ne sont pas concernées - l'utilisateur crée son
propre compte pendant l'installation, depuis la console du navigateur.

## Documentation (ce CMS)

Cette page fait partie de celles gérées sous **Documentation** (`/admin/docs`). Les administrateurs peuvent créer, modifier, activer/désactiver et supprimer des pages Markdown en anglais et en français. Les pages intégrées sont marquées **système** et ne peuvent pas être supprimées, mais leur contenu est modifiable. Glissez la poignée d'une page pour réordonner la liste ; le réordonnancement exige une recherche, des filtres et un tri vides. Chaque page a une audience `user` (publique) ou `admin` (administrateurs) ; les pages admin sont masquées aux non-administrateurs et refusées en accès direct.

Les pages intégrées sont insérées une seule fois, si elles manquent, et jamais écrasées au redémarrage : vos modifications survivent aux mises à jour. Pour récupérer un texte intégré plus récent, collez le nouveau contenu de la release dans la page.

## Paramètres, audit et maintenance

`/admin/settings` expose les commandes d'exploitation :

- **Journal d'audit** - chaque écriture (création de VM, action d'alimentation, modification, suppression, changements de catalogue et de politique, connexions) est enregistrée avec l'utilisateur, l'IP, la sévérité et la cible. Filtrez et paginez ; chaque VM affiche aussi ses propres entrées dans son onglet Activité.
- **Comptes** - gestion réservée au propriétaire des administrateurs locaux : usage du plafond, statut et verrou, dernière connexion, création, réinitialisation du mot de passe, activation/désactivation, suppression.
- **Rétention** - nombre de jours de conservation des lignes d'audit, avec aperçu du nombre de lignes qu'une purge supprimerait avant de l'appliquer.
- **Export / import de la base** - sauvegardez la base SQLite ou restaurez-en une. L'import est en deux temps : l'envoi renvoie un aperçu table par table, rien n'est écrit avant confirmation.

## Recommandations de sécurité

- Servez PVMSS uniquement en HTTPS (derrière un reverse proxy) et restreignez l'accès aux réseaux de confiance. Réglez `PVMSS_TRUSTED_PROXY_HOPS` au nombre de proxies pour que la limitation de débit et l'audit voient la vraie IP client.
- Utilisez des comptes Proxmox dédiés à PVMSS ; ne partagez pas le mot de passe administrateur intégré.
- Gardez des permissions Proxmox simples : un token de service avec le rôle `PVMSS_Service` pour le backend, des administrateurs humains avec le rôle `PVMSS_Admin`, et des utilisateurs confinés à leur pool `PVMSSUser`. Voir `/docs/proxmox-permissions` pour les commandes `pveum` exactes.
- N'accordez pas de privilèges Proxmox étendus aux utilisateurs self-service.
- Passez régulièrement en revue les pools et désactivez ou supprimez les comptes inutilisés.
- Les accès d'un template sont partagés par toutes les VM qui en sont clonées ; préférez les clés SSH aux mots de passe et renouvelez-les quand un utilisateur part.

## Limitations connues

- PVMSS cible les clusters Proxmox VE 8.x/9.x (et les nœuds autonomes).
- Les comptes Proxmox ne peuvent se connecter que via les realms `pve` et `pam` ; la connexion aux realms OIDC n'est pas encore prise en charge. Utilisez en attendant un administrateur local ou un compte `pve`/`pam`.
- Les comptes sont provisionnés via `/admin/pools` côté Proxmox.
- Les administrateurs peuvent migrer une VM à la fois depuis **Infrastructure > Nœuds**. La HA, la migration entre clusters et l'orchestration des sauvegardes se font directement dans Proxmox.
- Les administrateurs créent des VM via la même interface self-service que les utilisateurs, ou directement dans Proxmox.
- Sauvegardes et conteneurs LXC sont gérés dans Proxmox, pas dans PVMSS.
- Les tokens API personnels sont désactivés dans cette version : leurs routes ne sont pas enregistrées, la page des tokens ne peut ni créer ni lister de jetons.

# Proxmox VM Self-Service (PVMSS)

[![Lint](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml) [![Go](https://github.com/julienhmmt/pvmss/actions/workflows/go.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/go.yml)

> Portail web léger pour Proxmox VE qui permet de créer, exploiter et dépanner des machines virtuelles sans exposer l'interface Proxmox.

Version anglaise : [README.md](README.md)

---

## Sommaire

1. [Vue d'ensemble](#vue-densemble)
2. [Fonctionnalités](#fonctionnalités)
3. [Architecture en un coup d'œil](#architecture-en-un-coup-dœil)
4. [Configuration](#configuration)
5. [Options de déploiement](#options-de-déploiement)
6. [Démarrage rapide avec Docker run](#démarrage-rapide-avec-docker-run)
7. [Démarrer avec Docker compose](#démarrer-avec-docker-compose)
8. [Démarrer avec Kubernetes](#démarrer-avec-kubernetes)
9. [Exploitation](#exploitation)
10. [Limites connues](#limites-connues)
11. [Licence](#licence)

---

## Vue d'ensemble

PVMSS est une application stateless (API REST Go + SPA SvelteKit) qui s'appuie exclusivement sur les API Proxmox pour toutes les actions. Ses objectifs :

- **Sécurité par défaut** : sessions par utilisateur.
- **Simplicité d'exploitation** : image conteneur prête à l'emploi, limites de ressources configurables, sélection de stockage compatible cluster.
- **Centrée utilisateur** : formulaires clairs et guidés, documentation intégrée.

> ⚠️ Le projet reste en développement actif. Consultez la section [Limites connues](#limites-connues) avant un déploiement en production.

## Fonctionnalités

L'inventaire complet, route par route, est dans [docs/FEATURES.md](docs/FEATURES.md) (en anglais).

### Utilisateurs finaux

- Connexion avec ses identifiants Proxmox sur le cluster de son choix.
- **Mes VM** : liste multi-cluster, recherche/filtre/tri reflétés dans l'URL, statut en direct, actions d'alimentation groupées avec résultat par VM, bouton console sur chaque ligne.
- **Assistant de création** (mode Simple / Détaillé) depuis trois sources : une **ISO** approuvée, un **template** Proxmox (clone lié ou complet) ou une **image cloud** (`import-from` + cloud-init). Profils matériels ou valeurs libres, placement automatique avec score de capacité, multi-NIC (bridge + modèle), UEFI (sans Secure Boot) / TPM 2.0, tags curés, démarrage sur CD-ROM.
- **Exploitation d'une VM** : 7 actions d'alimentation, renommage, description Markdown, suppression ; disques (ajout / agrandissement / détachement) ; cartes réseau (bridge, modèle, VLAN, débit) ; CPU/RAM/tags/CD-ROM ; snapshots (création / restauration / suppression, RAM optionnelle) ; historique de métriques (heure/jour/semaine) et flux temps réel ; journal d'activité par VM.
- **Consoles** : noVNC et série (xterm.js), toutes deux relayées par PVMSS avec ticket à usage unique ; actions d'alimentation depuis la console.
- **Cloud-init** : formulaire natif (utilisateur, mot de passe via agent invité, clés SSH, IP/DNS), injection « ajouter une clé maintenant », **modèles cloud-init** de l'administrateur choisis à la création ou changés ensuite sur la VM (les utilisateurs n'écrivent jamais de YAML).
- Page Nœuds avec capacité en direct, documentation intégrée, FR + EN, navigation clavier, cible WCAG 2.1 AA.

### Administrateurs

- **Clusters** : connexion à plusieurs environnements Proxmox, test de connectivité, publication cloud-init par cluster en SSH (stockage de snippets, utilisateur SSH, clés d'hôte épinglées, scan des clés d'hôte).
- **Catalogue** : approbation des nœuds, stockages, ISO, images cloud, templates de VM, bridges ; CRUD des profils matériels, tags, modèles cloud-init (publiés sur chaque nœud, état par nœud, « publier partout ») ; approbations obsolètes réconciliées avec la découverte en direct.
- **Pools** : création d'un utilisateur self-service = utilisateur Proxmox + pool + ACL en une étape ; suppression en cascade.
- **Politique** : gabarit par cluster (sockets, cœurs, mémoire, disque par VM, NIC, snapshots, VLAN d'isolation) et quota (VM par utilisateur) ; plafonds de capacité par nœud avec usage en direct.
- **Système** : tableau de bord, informations applicatives, journal d'audit avec rétention + aperçu de purge, export SQLite et import en deux temps, CMS de documentation intégrée (FR/EN, par audience).

## Architecture en un coup d'œil

- **Serveur** (`server/`) : Go 1.27, routage `net/http` de la stdlib, SQLite via `modernc.org/sqlite` (sans CGO). Sert `/api/v1/*` et le SPA.
- **Web** (`web/`) : SPA SvelteKit (Svelte 5 runes, TypeScript, Tailwind CSS v4, `adapter-static`).
- **Authentification** : token API Proxmox pour les actions cluster, sessions utilisateur pour l'UI.

## Configuration

### Roles et permissions (obligatoire)

PVMSS utilise des rôles et des ACLs Proxmox pour fonctionner correctement (compte service, comptes admin, pools utilisateurs).

Avant d'utiliser PVMSS en production, vous **devez**:

- Créer les rôles `PVMSS_Service` et `PVMSS_Admin` avec les privilèges attendus.
- Créer les utilisateurs Proxmox correspondants, le token API et les ACL.

Les commandes `pveum` exactes et les privilèges requis sont documentés dans:

- La page d'admin intégrée `/docs/proxmox-permissions` (une fois PVMSS démarré)

Vous pouvez créer les rôles et les ACLs en utilisant le `pveum` en ligne de commande. Vous pouvez également les créer en utilisant l'interface web de Proxmox. En tant qu'utilisateur _root_, créez les rôles et les privilèges suivants :

```bash
# PVMSS_Service
pveum roleadd PVMSS_Service -privs "Sys.Audit VM.Audit VM.Allocate VM.PowerMgmt VM.Console VM.Config.CPU VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.Options VM.Config.Cloudinit VM.Snapshot VM.Snapshot.Rollback Datastore.Audit Datastore.AllocateSpace Datastore.AllocateTemplate Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Allocate SDN.Audit SDN.Use"

pveum useradd pvmss-svc@pve -comment "PVMSS service account" \
  -enable 1

pveum user token add pvmss-svc@pve pvmss-service-token --privsep 0

# PVMSS_Admin
pveum roleadd PVMSS_Admin -privs "Sys.Audit VM.Audit VM.PowerMgmt VM.Console VM.Config.CPU VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.HWType VM.GuestAgent.Audit VM.Migrate VM.Config.CDROM VM.Config.Options VM.Config.Cloudinit Datastore.Audit Datastore.AllocateSpace Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Audit Group.Allocate"

pveum useradd pvmss-admin1@pve \
  -comment "PVMSS administrator <name>" -password "strong_password" \
  -enable 1

pveum aclmod / -user pvmss-admin1@pve -role PVMSS_Admin -propagate 1
```

Les commandes `pveum` et les informations relatives aux rôles et aux privilèges requis sont détaillées dans :

- La page d'admin intégrée `/docs/proxmox-permissions` (une fois PVMSS démarré et vous êtes connecté en tant qu'administrateur)

### Créer un token API pour l'utilisateur root@pam

Si vous souhaitez utiliser PVMSS en développement, vous **devez** créer un token API pour l'utilisateur `root@pam`. C'est la manière la plus simple de commencer, mais gardez à l'esprit que c'est la moins sécurisée.

Allez dans Datacenter > Permissions > API Tokens. Cliquez sur le bouton “Add” et sélectionnez l'utilisateur `root@pam`. Tapez le nom du token API, décochez la case "Privilege Separations", et récupérez le secret (sera visible une seule fois).

Vous pouvez maintenant utiliser le token API dans les variables d'environnement `PROXMOX_API_TOKEN_NAME` et `PROXMOX_API_TOKEN_VALUE`.

### Configuration de la base de données

PVMSS utilise une base de données SQLite intégrée pour stocker toute la configuration. La base de données est initialisée automatiquement au premier démarrage et inclut :

- Les nœuds, stockages, VMBR et dépôts ISO approuvés
- Les limites de ressources VM (globales et par nœud)
- Les tags et pools utilisateurs
- Les modèles cloud-init et leurs publications sur les nœuds
- Les profils VM, les connexions aux clusters, le journal d'audit

Toute la configuration est gérée via la section **Admin** de l'interface web, qui fournit :

- Opérations CRUD complètes pour tous les éléments de configuration
- Historique d'audit de tous les changements
- Fonctionnalités d'import/export pour sauvegarde/restauration

Le fichier de base de données doit être persisté sur un volume pour survivre aux redémarrages du conteneur.

#### Tags

Le tag `pvmss` est utilisé par défaut pour les VMs créées via PVMSS, il ne peut pas et ne doit pas être supprimé. Seul les tags créés par l'admin via PVMSS peuvent être utilisés.

### Modèles cloud-init (optionnel)

Les administrateurs écrivent les modèles cloud-init dans **Admin › Modèles
cloud-init** ; les utilisateurs en choisissent un à la création d'une VM (ou
le changent ensuite dans l'onglet cloud-init de la VM). Les utilisateurs
n'écrivent jamais de YAML. L'API REST de Proxmox ne sait pas écrire de
fichiers `snippets` : PVMSS **publie** chaque modèle en SSH sur chaque nœud.

Mise en place rapide (détails, toutes les variantes de déploiement et
dépannage : [docs/cloud-init-ssh.md](docs/cloud-init-ssh.md)) :

```sh
# 1. Clé de PVMSS (poste de travail, racine du dépôt)
ssh-keygen -t ed25519 -N '' -C pvmss -f pvmss_ed25519

# 2. Type de contenu Snippets sur le stockage (un seul nœud, une fois ; ou GUI : Datacenter > Storage)
STORAGE=local
CUR=$(pvesh get /storage/$STORAGE --output-format json | perl -MJSON -0ne 'print decode_json($_)->{content}')
case ",$CUR," in *,snippets,*) ;; *) pvesm set "$STORAGE" --content "$CUR,snippets" ;; esac

# 3. Chaque nœud : assistant + utilisateur dédié « pvmss » (commande forcée, pas de shell)
NODES="192.168.1.11 192.168.1.12 192.168.1.13"; PUBKEY=$(cat pvmss_ed25519.pub)
for n in $NODES; do
  scp tools/pvmss-node-setup.sh root@"$n":/root/
  ssh root@"$n" "sh /root/pvmss-node-setup.sh --storage $STORAGE --user pvmss --key '$PUBKEY'"
done

# 4. Vérifier un nœud
ssh -i pvmss_ed25519 -o IdentitiesOnly=yes pvmss@192.168.1.11 check
```

5. Fournissez la clé privée à PVMSS : montée en lecture seule (lisible par
   l'uid 65532) et `PVMSS_SSH_KEY_FILE` (Helm : `cloudInit.sshKeySecret`).
6. **Admin › Clusters › Modifier** : stockage de snippets, utilisateur SSH
   `pvmss`, port et clés d'hôte épinglées (coller `for n in $NODES; do
   ssh-keyscan -t ed25519 $n; done`, ou enregistrer d'abord sans utilisateur
   SSH, rouvrir et **Scanner les clés d'hôte**), enregistrer. Le badge passe à
   « cloud-init : activé » et PVMSS publie le socle et tous les modèles.

PVMSS doit joindre l'IP de chaque nœud listée dans `/cluster/status` sur le
port SSH. Chaque fichier publié est immuable (`pvmss-tpl-<id>-<hash>.yml`,
socle PVMSS fusionné) : modifier un modèle publie un nouveau fichier et les VM
existantes gardent le leur. Un modèle absent du nœud d'une VM est refusé avant
la création, jamais attaché. Utilisez **Publier sur tous les nœuds** après
l'ajout ou la réinstallation d'un nœud.

### Variables d'environnement

Utilisez **soit** un `.env` (via `env_file`) **soit** des variables inline, pas les deux. Variables essentielles :

| Variable                                      | Description                                                                | Requis                | Valeur par défaut  |
| --------------------------------------------- | -------------------------------------------------------------------------- | --------------------- | ------------------ |
| `PVMSS_PORT`                                  | Port TCP d'écoute du serveur HTTP (1–65535)                                | ✅                    | - |
| `PVMSS_DB_PATH`                               | Chemin vers le fichier SQLite (volume persistant requis)                   | ✅                    | - |
| `SESSION_SECRET`                              | Secret de 32+ octets pour sessions/cookies                                 | ✅                    | - |
| `PVMSS_CLUSTER_SOURCE`                        | `proxmox` pour un vrai cluster, `fake` pour la démo (aucun défaut, exprès) | ✅                    | - |
| `LOG_LEVEL`                                   | `debug`, `info`, `warn`, `error` - minuscules uniquement                   | ✅                    | - |
| `LOG_FORMAT`                                  | `console` (lisible humainement) ou `json` (pour SIEM/collecte)             | ✅                    | - |
| `LOG_OUTPUT`                                  | `stdout`, `stderr`, ou un chemin de fichier accessible en écriture         | ✅                    | - |
| `PROXMOX_URL`                                 | URL complète de l'API (`https://host:8006/api2/json`)                      | si source = `proxmox` | - |
| `PROXMOX_API_TOKEN_NAME`                      | Nom du token Proxmox (`user@pve!token`)                                    | si source = `proxmox` | - |
| `PROXMOX_API_TOKEN_VALUE`                     | Valeur du token ci-dessus                                                  | si source = `proxmox` | - |
| `ADMIN_PASSWORD_HASH`                         | Hash bcrypt de l'admin local ; désactivé si vide                           | ❌                    | - |
| `PVMSS_SSH_KEY_FILE`                          | Clé privée SSH qui publie les modèles cloud-init sur les nœuds (utilisateur, port, clés d'hôte : Admin › Clusters) | ❌                    | - |
| `PVMSS_HOST`                                  | Adresse d'écoute (`0.0.0.0` pour toutes les interfaces)                    | ❌                    | `127.0.0.1`        |
| `PVMSS_WEB_DIR`                               | Répertoire contenant le SPA compilé                                        | ❌                    | relatif au binaire |
| `PVMSS_COOKIE_SECURE`                         | Drapeau `Secure` sur les cookies d'auth (garder `true` en production)      | ❌                    | `true`             |
| `PVMSS_INVENTORY_REFRESH_INTERVAL`            | Période de rafraîchissement de l'inventaire                                | ❌                    | `30s`              |
| `PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL` | Délai minimum entre deux rafraîchissements manuels                         | ❌                    | `5s`               |
| `PVMSS_INVENTORY_REFRESH_TIMEOUT`             | Timeout d'un rafraîchissement d'inventaire                                 | ❌                    | `15s`              |
| `PVMSS_MAX_LIST_PAGE_SIZE`                    | Taille de page maximale des endpoints de liste                             | ❌                    | `100`              |
| `PVMSS_TRUSTED_PROXY_HOPS`                    | Nombre de reverse proxies devant PVMSS (IP client / limitation de débit)   | ❌                    | `1`                |
| `TZ`                                          | Fuseau horaire du conteneur                                                | ❌                    | `UTC`              |

L'image Docker prérègle `PVMSS_DB_PATH=/data/pvmss.db`, `PVMSS_HOST=0.0.0.0` et
`PVMSS_WEB_DIR=/app/web/build` : ces trois-là peuvent rester vides en conteneur.

> Astuce : `htpasswd -bnBC 10 "admin" "MotDePasseFort" | cut -d: -f2` permet de générer `ADMIN_PASSWORD_HASH`.

#### Configuration des logs

PVMSS utilise des logs structurés basés sur `log/slog` de la bibliothèque
standard. Les trois variables sont obligatoires ; `LOG_LEVEL` est comparé en
tenant compte de la casse et n'accepte que des minuscules. `LOG_OUTPUT` accepte
`stdout`, `stderr` ou un chemin de fichier - il n'y a pas de mode « both ».

- Logs lisibles en développement :

  ```bash
  LOG_LEVEL=debug
  LOG_OUTPUT=stdout
  LOG_FORMAT=console
  ```

- Logs JSON sur stdout pour un collecteur de logs / SIEM :

  ```bash
  LOG_LEVEL=info
  LOG_OUTPUT=stdout
  LOG_FORMAT=json
  ```

- Logs JSON dans un fichier à l'intérieur du conteneur :

  ```bash
  LOG_LEVEL=info
  LOG_OUTPUT=/app/pvmss.log
  LOG_FORMAT=json
  ```

Le format JSON est une ligne par événement, avec un champ `component` (main, cluster, inventory, ...). Cela facilite l'ingestion par Fluent Bit, Filebeat ou un SIEM.

## Options de déploiement

| Plateforme          | Détails                                                                                                                                                                                                                                                    |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Docker / Podman** | Idéal pour tester ou déployer sur un seul hôte. Montez le volume de base de données et exposez `50000`.                                                                                                                                                    |
| **Docker Compose**  | Expérience recommandée : service unique, variables centralisées, environnement reproductible.                                                                                                                                                              |
| **Kubernetes**      | Utilisez [`pvmss-deployment.yaml`](pvmss-deployment.yaml) (namespace + secret + configmap + PVC + Deployment + Service). Appliquez via `kubectl apply -f pvmss-deployment.yaml`. L'ingress/HTTPRoute reste à votre charge (exemple `pvmss-httproute.yml`). |

## Démarrage rapide avec Docker run

```bash
docker run -d \
  --name pvmss \
  --restart unless-stopped \
  -p 50000:50000 \
  -v $(pwd)/pvmss.db:/data/pvmss.db \
  -e ADMIN_PASSWORD_HASH='$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e' \
  -e LOG_LEVEL=info \
  -e LOG_OUTPUT=stdout \
  -e LOG_FORMAT=console \
  -e PROXMOX_API_TOKEN_NAME='tokenName@changeMe!value' \
  -e PROXMOX_API_TOKEN_VALUE="aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa" \
  -e PROXMOX_URL=https://ip-or-name:8006/api2/json \
  -e PVMSS_CLUSTER_SOURCE=proxmox \
  -e PVMSS_PORT=50000 \
  -e PVMSS_DB_PATH="/data/pvmss.db" \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -e TZ=Europe/Paris \
  jhmmt/pvmss:latest
```

Pour activer les modèles cloud-init, montez aussi la clé SSH de PVMSS
(voir [Modèles cloud-init](#modèles-cloud-init-optionnel)) :

```bash
-v ./pvmss_ed25519:/etc/pvmss/ssh/id_ed25519:ro -e PVMSS_SSH_KEY_FILE=/etc/pvmss/ssh/id_ed25519 \
```

Pour écrire les logs JSON dans un fichier à l'intérieur du conteneur au lieu de stdout, surchargez :

```bash
-e LOG_FORMAT=json \
-e LOG_OUTPUT=/app/pvmss.log \
-v $(pwd)/pvmss.log:/app/pvmss.log \
```

L'application sera accessible sur <http://localhost:50000>.

## Démarrer avec Docker compose

1. **Créer `docker-compose.yml` :**

```yaml
services:
  pvmss:
    image: jhmmt/pvmss:latest
    container_name: pvmss
    restart: unless-stopped
    ports:
      - "50000:50000/tcp"
    environment:
      PROXMOX_API_TOKEN_NAME: "tokenName@changeMe!value"
      PROXMOX_API_TOKEN_VALUE: "aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa"
      PROXMOX_URL: "https://ip-or-name:8006/api2/json"
      PVMSS_CLUSTER_SOURCE: "proxmox"
      ADMIN_PASSWORD_HASH: "$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e"
      LOG_LEVEL: "info" # "debug", "info", "warn", "error"
      LOG_OUTPUT: "stdout" # "stdout", "stderr", ou un chemin de fichier
      LOG_FORMAT: "console" # "console", "json"
      SESSION_SECRET: "changeMeWithSomethingElseUniqueMinimum32Chars"
      PVMSS_PORT: "50000"
      PVMSS_DB_PATH: "/data/pvmss.db"
      TZ: "Europe/Paris"
    volumes:
      - ./pvmss.db:/data/pvmss.db
      # - ./pvmss.log:/app/pvmss.log # Décommentez pour persister les logs dans un fichier à l'intérieur du conteneur
      # Modèles cloud-init : clé SSH de PVMSS (voir la section plus haut),
      # plus PVMSS_SSH_KEY_FILE: "/etc/pvmss/ssh/id_ed25519" dans environment.
      # - ./pvmss_ed25519:/etc/pvmss/ssh/id_ed25519:ro
    deploy:
      resources:
        limits:
          cpus: "1"
          memory: 64M
```

Pour persister les logs dans un fichier à l'intérieur du conteneur, vous pouvez ajuster la section `environment` :

```yaml
LOG_FORMAT: "json"
LOG_OUTPUT: "/app/pvmss.log"
# Ajoutez ce volume à la section volumes
- ./pvmss.log:/app/pvmss.log
```

1. **Démarrer la stack :**

   ```bash
   docker compose up -d
   ```

2. Ouvrez un navigateur et accédez à **<http://localhost:50000>**.
3. Se connecter avec le compte admin, sur la page "Login", cliquez sur "Connexion administrateur".

## Démarrer avec Kubernetes

Utilisez le fichier [`pvmss-deployment.yaml`](pvmss-deployment.yaml) pour créer en une seule fois le namespace, le secret, le configmap, le PVC, le Deployment et le Service.

Appliquez avec `kubectl apply -f pvmss-deployment.yaml`. Vous devez fournir votre propre ingress/HTTPRoute, un exemple est fourni dans le fichier `pvmss-httproute.yml` (Gateway API).

Un chart Helm est fourni dans [`helm/`](helm/) : `helm install pvmss ./helm -f mes-values.yaml`.
`values.yaml` documente chaque variable ; `cloudInit.sshKeySecret` monte la clé
SSH de PVMSS depuis un Secret existant pour publier les modèles cloud-init.

## Exploitation

- **Logs** : `docker logs -f pvmss` ou `kubectl -n pvmss logs -f deploy/pvmss`. Passez `LOG_LEVEL=debug` pour plus de verbosité. Utilisez `LOG_FORMAT=json` avec `LOG_OUTPUT=stdout` ou un chemin de fichier pour produire des logs JSON exploitables par un SIEM ou un collecteur de logs.
- **Santé** : les logs de démarrage détaillent la connectivité Proxmox et l'état du rafraîchissement d'inventaire ; `GET /health` répond pour les sondes. La page admin "Informations de l'application" affiche des métriques, des variables d'environnement et le statut du cluster Proxmox.
- **Mises à jour** : récupérez l'image souhaitée (le tag) et redémarrez le conteneur. La configuration est stockée dans la base de données SQLite et persiste automatiquement.
- **Documentation intégrée** : les pages `/docs` sont insérées une seule fois en base et jamais écrasées, les modifications admin survivent donc aux mises à jour. Pour récupérer un texte intégré plus récent après une mise à jour, supprimez la page dans **Admin › Documentation** et redémarrez, ou collez le nouveau contenu depuis `server/internal/docs/seed/`.

## Limites connues

- Pas encore d'audit de sécurité complet.
- OIDC : l'interrupteur par cluster existe mais la connexion n'est pas implémentée (l'endpoint répond 501).
- Le changement de mot de passe n'est disponible que par API (`POST /api/v1/auth/password`), pas encore de page.
- Conteneurs LXC, sauvegardes, migration à chaud / HA, SDN et règles de pare-feu restent dans Proxmox.

### Prochaines évolutions majeures

- Connexion OpenID Connect / SSO ;
- Migration de VM entre les nœuds Proxmox ;

Toute suggestion et contribution sont les bienvenues via issues ou pull requests. Les prochaines versions et ajouts de fonctionnalités seront documentés ici : <https://github.com/julienhmmt/pvmss/projects?query=is%3Aopen>.

## Licence

PVMSS par Julien HOMMET est distribué sous **GNU AGPL v3**. Voir <https://www.gnu.org/licenses/agpl-3.0.html>.

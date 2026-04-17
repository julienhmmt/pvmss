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

PVMSS est une application stateless (backend Go + frontend HTML/CSS) qui s'appuie exclusivement sur les API Proxmox pour toutes les actions. Ses objectifs :

- **Sécurité par défaut** : sessions par utilisateur.
- **Simplicité d'exploitation** : image conteneur prête à l'emploi, limites de ressources configurables, sélection de stockage compatible cluster.
- **Centrée utilisateur** : formulaires clairs et guidés, documentation intégrée.

> ⚠️ Le projet reste en développement actif. Consultez la section [Limites connues](#limites-connues) avant un déploiement en production.

## Fonctionnalités

### Utilisateurs finaux

- Création de VM avec choix CPU/RAM/disque/ISO/réseau/tag (EFI, TPM, multi-cartes réseau, bus disque, modèle de carte réseau, etc.).
- Accès console via noVNC directement dans le portail (proxy WebSocket avec cookies de session).
- Démarrage, arrêt, redémarrage, suppression et redimensionnement des VM existantes.
- Recherche par VMID ou nom, consultation des métriques temps réel (CPU, RAM, stockage, réseau, uptime).
- Espace profil pour gérer ses VM et réinitialiser son mot de passe.
- Interface disponible en **français** et **anglais**.

### Administrateurs

- Validation des nœuds, stockages, VMBR et dépôts ISO exposés aux utilisateurs.
- Gestion des tags et des pools utilisateurs.
- Définition de limites globales par VM et plafonds par nœud (CPU, RAM, disque, nombre de NIC/disques).
- Documentation et page d'informations applicatives (runtime, environnement, statut cluster).
- **Panneau de configuration unifié** : Une interface unique pour gérer toute la configuration (limites VM, limites de nœud, éléments d'inventaire, modèles cloud-init, profils VM, configuration SFTP) avec journal d'audit et fonctionnalités d'import/export.

## Architecture en un coup d'œil

- **Backend** : Go 1.25+, client RESTy pour les API Proxmox, templates HTML basiques.
- **Frontend** : Basé sur Bulma avec CSS personnalisé.
- **Authentification** : token API Proxmox pour le backend, sessions utilisateur pour l'UI.

## Configuration

### Roles et permissions (obligatoire)

PVMSS utilise des rôles et des ACLs Proxmox pour fonctionner correctement (compte service, comptes admin, pools utilisateurs).

Avant d'utiliser PVMSS en production, vous **devez**:

- Créer les rôles `PVMSS_Service` et `PVMSS_Admin` avec les privilèges attendus.
- Créer les utilisateurs Proxmox correspondants, le token API et les ACL.

Les commandes `pveum` exactes et les privilèges requis sont documentés dans:

- `backend/docs/proxmox-permissions.fr.md` (dans ce dépôt)
- La page d'admin intégrée `/docs/proxmox-permissions` (une fois PVMSS démarré)

Vous pouvez créer les rôles et les ACLs en utilisant le `pveum` en ligne de commande. Vous pouvez également les créer en utilisant l'interface web de Proxmox. En tant qu'utilisateur _root_, créez les rôles et les privilèges suivants :

```bash
# PVMSS_Service
pveum roleadd PVMSS_Service -privs "Sys.Audit VM.Audit VM.Allocate VM.PowerMgmt VM.Console VM.Config.CPU VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.Options VM.Config.Cloudinit VM.Snapshot VM.Snapshot.Rollback Datastore.Audit Datastore.AllocateSpace Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Allocate SDN.Audit SDN.Use"

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

- `backend/docs/proxmox-permissions.fr.md` (dans ce dépôt)
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
- Les templates cloud-init et configuration SFTP
- Les profils VM

Toute la configuration est gérée via la section **Admin** de l'interface web, qui fournit :

- Opérations CRUD complètes pour tous les éléments de configuration
- Historique d'audit de tous les changements
- Fonctionnalités d'import/export pour sauvegarde/restauration

Le fichier de base de données doit être persisté sur un volume pour survivre aux redémarrages du conteneur.

#### Tags

Le tag `pvmss` est utilisé par défaut pour les VMs créées via PVMSS, il ne peut pas et ne doit pas être supprimé. Seul les tags créés par l'admin via PVMSS peuvent être utilisés.

### Variables d'environnement

Utilisez **soit** un `.env` (via `env_file`) **soit** des variables inline, pas les deux. Variables essentielles :

| Variable                  | Description                                                    | Requis | Valeur par défaut    |
| ------------------------- | -------------------------------------------------------------- | :----: | -------------------- |
| `ADMIN_PASSWORD_HASH`     | Hash bcrypt pour l'admin                                       |   ✅   | —                    |
| `SESSION_SECRET`          | Secret de 32+ octets pour sessions/cookies                     |   ✅   | —                    |
| `PROXMOX_API_TOKEN_NAME`  | Nom du token Proxmox (`user@pve!token`)                        |   ✅   | —                    |
| `PROXMOX_API_TOKEN_VALUE` | Valeur du token ci-dessus                                      |   ✅   | —                    |
| `PROXMOX_URL`             | URL complète de l'API (`https://host:8006/api2/json`)          |   ✅   | —                    |
| `PROXMOX_VERIFY_SSL`      | `true` pour certificats valides, `false` sinon                 |   ❌   | `false`              |
| `PVMSS_ENV`               | `production/prod` ou `development/dev/developpement`           |   ❌   | `production`         |
| `PVMSS_OFFLINE`           | `true` pour désactiver les appels Proxmox                      |   ❌   | `false`              |
| `PVMSS_DB_PATH`          | Chemin vers le fichier SQLite DB (volume persistant requis)    |   ✅   | `/data/pvmss.db`     |
| `LOG_LEVEL`               | Verbosité des logs (`debug`, `info`, `warn`, `error`)          |   ❌   | `INFO`               |
| `LOG_OUTPUT`              | Destination des logs : `stdout`, `file` ou `both`              |   ❌   | `stdout`             |
| `LOG_FILE_PATH`           | Chemin du fichier log si `LOG_OUTPUT` = `file` ou `both`       |   ❌   | —                    |
| `LOG_FORMAT`              | `console` (lisible humainement) ou `json` (pour SIEM/collecte) |   ❌   | `console`            |
| `TZ`                      | Fuseau horaire du conteneur                                    |   ❌   | `UTC`                |

> Astuce : `htpasswd -bnBC 10 "admin" "MotDePasseFort" | cut -d: -f2` permet de générer `ADMIN_PASSWORD_HASH`.

#### Configuration des logs

PVMSS utilise des logs structurés basés sur [zerolog](https://github.com/rs/zerolog).

- Logs lisibles en développement :

  ```bash
  LOG_LEVEL=DEBUG
  LOG_OUTPUT=stdout
  LOG_FORMAT=console
  ```

- Logs JSON sur stdout pour un collecteur de logs / SIEM :

  ```bash
  LOG_LEVEL=INFO
  LOG_OUTPUT=stdout
  LOG_FORMAT=json
  ```

- Logs JSON sur stdout **et** dans un fichier dans le conteneur :

  ```bash
  LOG_LEVEL=INFO
  LOG_OUTPUT=both
  LOG_FORMAT=json
  LOG_FILE_PATH=/app/pvmss.log
  ```

Le format JSON est une ligne par événement, avec des champs comme `component`, `operation`, `reason` et `event_category` (auth, VM, admin, sécurité, console, Proxmox). Cela facilite l'ingestion par Fluent Bit, Filebeat ou un SIEM.

## Options de déploiement

| Plateforme          | Détails                                                                                                                                                                                                                                                    |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Docker / Podman** | Idéal pour tester ou déployer sur un seul hôte. Montez le volume de base de données et exposez `50000`.                                                                                                                                                 |
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
  -e LOG_LEVEL=INFO \
  -e LOG_OUTPUT=stdout \
  -e LOG_FORMAT=console \
  -e PROXMOX_API_TOKEN_NAME='tokenName@changeMe!value' \
  -e PROXMOX_API_TOKEN_VALUE="aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa" \
  -e PROXMOX_URL=https://ip-or-name:8006/api2/json \
  -e PROXMOX_VERIFY_SSL=false \
  -e PVMSS_ENV="prod" \
  -e PVMSS_OFFLINE="false" \
  -e PVMSS_DB_PATH="/data/pvmss.db" \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -e TZ=Europe/Paris \
  jhmmt/pvmss:0.3.0
```

Pour écrire également les logs JSON dans un fichier à l'intérieur du conteneur (tout en conservant stdout), vous pouvez surcharger :

```bash
-e LOG_OUTPUT=both \
-e LOG_FORMAT=json \
-e LOG_FILE_PATH=/app/pvmss.log \
-v $(pwd)/pvmss.log:/app/pvmss.log \
```

L'application sera accessible sur <http://localhost:50000>.

## Démarrer avec Docker compose

1. **Créer `docker-compose.yml` :**

```yaml
services:
  pvmss:
    image: jhmmt/pvmss:0.3.0
    container_name: pvmss
    restart: unless-stopped
    ports:
      - "50000:50000/tcp"
    environment:
      PROXMOX_API_TOKEN_NAME: "tokenName@changeMe!value"
      PROXMOX_API_TOKEN_VALUE: "aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa"
      PROXMOX_URL: "https://ip-or-name:8006/api2/json"
      PROXMOX_VERIFY_SSL: "false"
      ADMIN_PASSWORD_HASH: "$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e"
      LOG_LEVEL: "INFO" # "DEBUG", "INFO", "WARN", "ERROR"
      LOG_OUTPUT: "stdout" # "stdout", "file", "both"
      LOG_FORMAT: "console" # "console", "json"
      SESSION_SECRET: "changeMeWithSomethingElseUnique"
      PVMSS_ENV: "production" # "prod", "development", "dev", "developpement"
      PVMSS_OFFLINE: "false" # "true" ou "false"
      PVMSS_DB_PATH: "/data/pvmss.db"
      TZ: "Europe/Paris"
    volumes:
      - ./pvmss.db:/data/pvmss.db
      # - ./pvmss.log:/app/pvmss.log # Décommentez pour persister les logs dans un fichier à l'intérieur du conteneur
    deploy:
      resources:
        limits:
          cpus: "1"
          memory: 64M
```

Pour persister les logs dans un fichier à l'intérieur du conteneur, vous pouvez ajuster la section `environment` :

```yaml
LOG_OUTPUT: "both"
LOG_FORMAT: "json"
LOG_FILE_PATH: "/app/pvmss.log"
# Ajoutez ce volume à la section volumes
- ./pvmss.log:/app/pvmss.log
```

2. **Démarrer la stack :**

   ```bash
   docker compose up -d
   ```

3. Ouvrez un navigateur et accédez à **<http://localhost:50000>**.
4. Se connecter avec le compte admin, sur la page "Login", cliquez sur "Connexion administrateur".

## Démarrer avec Kubernetes

Utilisez le fichier [`pvmss-deployment.yaml`](pvmss-deployment.yaml) pour créer en une seule fois le namespace, le secret, le configmap, le PVC, le Deployment et le Service.

Appliquez avec `kubectl apply -f pvmss-deployment.yaml`. Vous devez fournir votre propre ingress/HTTPRoute, un exemple est fourni dans le fichier `pvmss-httproute.yml` (Gateway API).

## Exploitation

- **Logs** : `docker logs -f pvmss` ou `kubectl -n pvmss logs -f deploy/pvmss`. Passez `LOG_LEVEL=DEBUG` pour plus de verbosité. Utilisez `LOG_FORMAT=json` et `LOG_OUTPUT=stdout` ou `file` pour produire des logs JSON exploitables par un SIEM ou un collecteur de logs.
- **Santé** : les logs de démarrage détaillent la connectivité Proxmox, l'état du mode hors-ligne et les métriques runtime. La page admin "Informations de l'application" affiche des métriques, des variables d'environnement et le statut du cluster Proxmox.
- **Mises à jour** : récupérez l'image souhaitée (le tag) et redémarrez le conteneur. La configuration est stockée dans la base de données SQLite et persiste automatiquement.

## Limites connues

- Pas encore d'audit de sécurité complet.
- L'application n'est pas encore dynamique, le travail est en cours pour optimiser les interactions.

### Prochaines évolutions majeures

- Intégration OpenID Connect / SSO ;
- Migration de VM entre les nœuds Proxmox ;

Toute suggestion et contribution sont les bienvenues via issues ou pull requests. Les prochaines versions et ajouts de fonctionnalités seront documentés ici : <https://github.com/julienhmmt/pvmss/projects?query=is%3Aopen>.

## Licence

PVMSS par Julien HOMMET est distribué sous **GNU AGPL v3**. Voir <https://www.gnu.org/licenses/agpl-3.0.html>.

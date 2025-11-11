# Proxmox VM Self-Service (PVMSS)

[![Lint](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml)

Version anglaise : [README.md](README.md)

PVMSS est un portail web léger et en libre-service pour Proxmox Virtual Environment (PVE). Il permet aux utilisateurs de créer et de gérer des machines virtuelles (VM) sans avoir besoin d'un accès direct à l'interface web de Proxmox. L'application est conçue pour être simple, rapide et facile à déployer en tant que conteneur (Docker, Podman, Kubernetes).

⚠️ L'application est actuellement en version de développement et présente des limites, listées en bas de page.

## Fonctionnalités

### Pour les utilisateurs

- **Créer une VM** : Créer une nouvelle machine virtuelle avec des ressources personnalisables (CPU, RAM, stockage, ISO, réseau, tag).
- **Accès console VM** : Accès console noVNC direct aux machines virtuelles via un client VNC web intégré.
- **Gestion des VM** : Démarrer, arrêter, redémarrer et supprimer des machines virtuelles, modifier les ressources.
- **Recherche de VM** : Trouver des machines virtuelles par VMID ou son nom.
- **Détails des VM** : Afficher les informations complètes des VM incluant le statut, la description, l'uptime, CPU, mémoire, utilisation disque et configuration réseau.
- **Gestion du profil** : Consulter et gérer ses VM, réinitialiser son mot de passe.
- **Multi-langue** : L'interface est disponible en français et en anglais.

### Pour les administrateurs

- **Gestion des nœuds** : Afficher les nœuds Proxmox disponibles, dans le cluster.
- **Gestion du pool d'utilisateurs** : Ajouter ou supprimer des utilisateurs dans l'application.
- **Gestion des tags** : Créer et gérer des tags pour l'organisation des VM.
- **Gestion des ISO** : Configurer les images ISO disponibles pour l'installation de VM.
- **Configuration réseau** : Gérer les ponts réseau disponibles (VMBRs) pour le réseau des VM, et la quantité de carte réseau attachable par VM.
- **Gestion du stockage** : Configurer les emplacements de stockage pour les disques des VM, et la quantité de disque attachable par VM.
- **Limites de ressources** : Définir les limites de CPU, RAM utilisable par nœud Proxmox et les limites de CPU, RAM et disque utilisable par VM.
- **Documentation administrateur** : Documentation administrateur accessible depuis le panneau d'administration.

## Déploiement de l'application avec Docker

Suivez ces instructions pour lancer PVMSS localement avec Docker.

### Prérequis

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)

### Configuration des variables d'environnement

Vous pouvez utiliser le fichier d'exemple fourni `env.example` pour créer votre propre fichier `.env`. Ou vous pouvez modifier le fichier d'exemple directement.

**Configuration :**

- `ADMIN_PASSWORD_HASH` : Un hash bcrypt du mot de passe pour le panneau d'administration. Vous pouvez en générer un à l'aide d'un outil en ligne ou d'un script simple.
- `LOG_LEVEL` : Définir le niveau de log de l'application : `INFO` ou `DEBUG` (par défaut : `INFO`).
- `PROXMOX_API_TOKEN_NAME` : Le nom de votre token API Proxmox pour les opérations backend (ex : `user@pve!token`).
- `PROXMOX_API_TOKEN_VALUE` : La valeur secrète de votre token API.
- `PROXMOX_URL` : L'URL complète vers votre endpoint API Proxmox (ex : `https://proxmox.example.com:8006/api2/json`).
- `PROXMOX_VERIFY_SSL` : Définir à `false` si vous utilisez un certificat auto-signé sur Proxmox (par défaut : `false`).
- `PVMSS_ENV` : Définir l'environnement de l'application : `production`, `prod` (active les cookies sécurisés et les en-têtes HSTS) ou `development`, `dev`, `developpement` (par défaut : `production`).
- `PVMSS_OFFLINE` : Définir à `true` pour activer le mode déconnecté (désactive tous les appels API Proxmox). Utile pour le développement ou lorsque Proxmox n'est pas disponible (par défaut : `false`).
- `PVMSS_SETTINGS_PATH` : Chemin où se trouve le fichier de configuration (par défaut, `/app/settings.json`).
- `SESSION_SECRET` : Clé secrète pour le chiffrement des sessions (changez pour une chaîne aléatoire unique, par exemple `$ openssl rand -hex 32`).

### Lancer le conteneur

Avec Docker en cours d'exécution, exécutez la commande suivante depuis la racine du projet :

```bash
# Créez le fichier `docker-compose.yml` avec le contenu suivant :
---
services:
  pvmss:
    image: jhmmt/pvmss:0.2.0
    container_name: pvmss
    restart: unless-stopped
    ports:
      - "50000:50000/tcp"
    # Use either the .env file for environment variables
    # or the environment variables in the docker-compose.yml file.
    # env_file:
    #  - .env
    environment:
      # Proxmox VE settings
      PROXMOX_API_TOKEN_NAME: "tokenName@changeMe!value"
      PROXMOX_API_TOKEN_VALUE: "aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa"
      PROXMOX_URL: "https://ip-or-name:8006/api2/json"
      PROXMOX_VERIFY_SSL: "false"
      # PVMSS settings
      ADMIN_PASSWORD_HASH: "$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e"
      LOG_LEVEL: "INFO"
      SESSION_SECRET: "changeMeWithSomethingElseUnique"
      PVMSS_ENV: "prod" # Environment: production/prod or development/dev/developpement
      PVMSS_OFFLINE: "false"
      PVMSS_SETTINGS_PATH: "/app/settings.json"
      TZ: "Europe/Paris"
    volumes:
      - ./settings.json:/app/settings.json
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 64M

# Démarrer le conteneur en mode détaché
docker compose up -d
```

Ou lancez le conteneur avec `docker run` :

```bash
docker run -d \
  --name pvmss \
  --restart unless-stopped \
  -p 50000:50000 \
  -v $(pwd)/settings.json:/app/settings.json \
  -e ADMIN_PASSWORD_HASH="$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e" \
  -e LOG_LEVEL=INFO \
  -e PROXMOX_API_TOKEN_NAME="tokenName@changeMe!value" \
  -e PROXMOX_API_TOKEN_VALUE="aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa" \
  -e PROXMOX_URL=https://ip-or-name:8006/api2/json \
  -e PROXMOX_VERIFY_SSL=false \
  -e PVMSS_ENV="prod" \
  -e PVMSS_OFFLINE="false" \
  -e PVMSS_SETTINGS_PATH="/app/settings.json" \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -e TZ=Europe/Paris \
  jhmmt/pvmss:0.2.0
```

L'application sera disponible à l'adresse [http://localhost:50000](http://localhost:50000).

### Consulter les logs

Pour voir les logs de l'application, exécutez :

```bash
docker logs -f pvmss
```

## Déploiement de l'application dans Kubernetes

Un manifest comportant toutes les informations nécessaires est disponible à la racine de ce dépôt, nommé `pvmss-deployment.yaml`. Ce manifest contient :

- Un namespace
- Un service account
- Un secret
- Un configmap
- Un persistent volume claim (pvc)
- Un deployment
- Un service

Pour déployer l'application dans Kubernetes, exécutez la commande suivante :

```bash
kubectl apply -f pvmss-deployment.yaml
```

La gestion de l'ingress ou de la gateway-api est à votre charge. Un exemple de manifest http-route est disponible à la racine de ce dépôt, nommé `pvmss-httproute.yml`.

## Limitations / Liste 'à faire'

- Il n'y a pas eu de tests rigoureux de sécurité, attention lors du déploiement.
- Pas (encore) de support de Cloud-Init.
- Seulement un seul hôte Proxmox est correctement supporté. Les clusters Proxmox ne sont pas encore bien gérés.
- Pas (encore) de support d'OpenID Connect
- Nécessité d'un meilleur système de journalisation, dans un fichier et la possibilité d'envoyer avec un serveur distant, dans un format syslog.

## Licence

PVMSS by Julien HOMMET is licensed under Creative Commons Attribution-NonCommercial-NoDerivatives 4.0 International. To view a copy of this license, visit <https://creativecommons.org/licenses/by-nc-nd/4.0/>.

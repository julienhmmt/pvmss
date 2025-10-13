# Proxmox VM Self-Service (PVMSS)

[![Lint](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml)

Version anglaise : [README.md](README.md)

PVMSS est un portail web léger et en libre-service pour Proxmox Virtual Environment (PVE). Il permet aux utilisateurs de créer et de gérer des machines virtuelles (VM) sans avoir besoin d'un accès direct à l'interface web de Proxmox. L'application est conçue pour être simple, rapide et facile à déployer en tant que conteneur Docker.

⚠️ L'application est actuellement en version de développement et présente des limites, listées en bas de page.

## Fonctionnalités

### Pour les utilisateurs

- **Créer une VM** : Créer une nouvelle machine virtuelle avec des ressources personnalisables (CPU, RAM, stockage).
- **Accès console VM** : Accès console noVNC direct aux machines virtuelles via un client VNC web intégré.
- **Gestion des VM** : Démarrer, arrêter, redémarrer et supprimer des machines virtuelles.
- **Recherche de VM** : Trouver des machines virtuelles par VMID ou son nom.
- **Détails des VM** : Afficher les informations complètes des VM incluant le statut, la description, l'uptime, CPU, mémoire, utilisation disque et configuration réseau.
- **Gestion du profil** : Consulter et gérer ses propres VM, réinitialiser son mot de passe.
- **Multi-langue** : L'interface est disponible en anglais et en français avec détection automatique de la langue.

### Pour les administrateurs

- **Gestion des nœuds** : Configurer et gérer les nœuds Proxmox disponibles pour le déploiement de VM.
- **Gestion du pool d'utilisateurs** : Ajouter ou supprimer des utilisateurs avec génération automatique de mots de passe.
- **Gestion des tags** : Créer et gérer des tags pour l'organisation des VM.
- **Gestion des ISO** : Configurer les images ISO disponibles pour l'installation de VM.
- **Configuration réseau** : Gérer les ponts réseau disponibles (VMBRs) pour le réseau des VM.
- **Gestion du stockage** : Configurer les emplacements de stockage pour les disques des VM.
- **Limites de ressources** : Définir les limites de CPU, RAM et disque pour la création de VM.
- **Documentation** : Documentation utilisateur intégrée accessible depuis le panneau d'administration.

## Démarrage

Suivez ces instructions pour lancer PVMSS localement avec Docker.

### Prérequis

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)

### 1. Configurer les variables d'environnement

Vous pouvez utiliser le fichier d'exemple fourni `env.example` pour créer votre propre fichier `.env`. Ou vous pouvez modifier le fichier d'exemple directement.

**Configuration :**

- `ADMIN_PASSWORD_HASH` : Un hash bcrypt du mot de passe pour le panneau d'administration. Vous pouvez en générer un à l'aide d'un outil en ligne ou d'un script simple.
- `LOG_LEVEL` : Définir le niveau de log de l'application : `INFO` ou `DEBUG` (par défaut : `INFO`).
- `PROXMOX_API_TOKEN_NAME` : Le nom de votre token API Proxmox pour les opérations backend (ex : `user@pve!token`).
- `PROXMOX_API_TOKEN_VALUE` : La valeur secrète de votre token API.
- `PROXMOX_URL` : L'URL complète vers votre endpoint API Proxmox (ex : `https://proxmox.example.com:8006/api2/json`).
- `PROXMOX_VERIFY_SSL` : Définir à `false` si vous utilisez un certificat auto-signé sur Proxmox (par défaut : `false`).
- `SESSION_SECRET` : Clé secrète pour le chiffrement des sessions (changez pour une chaîne aléatoire unique, par exemple `$ openssl rand -hex 32`).

### 2. Lancer le conteneur

Avec Docker en cours d'exécution, exécutez la commande suivante depuis la racine du projet :

```bash
# Démarrer le conteneur en mode détaché
docker compose up -d
```

Ou lancez le conteneur avec `docker run` :

```bash
docker run -d \
  --name pvmss \
  --restart unless-stopped \
  -p 50000:50000 \
  -v $(pwd)/backend/settings.json:/app/settings.json \
  -e ADMIN_PASSWORD_HASH="$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e" \
  -e LOG_LEVEL=INFO \
  -e PROXMOX_API_TOKEN_NAME="tokenName@changeMe!value" \
  -e PROXMOX_API_TOKEN_VALUE="aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa" \
  -e PROXMOX_URL=https://ip-or-name:8006/api2/json \
  -e PROXMOX_VERIFY_SSL=false \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -e TZ=Europe/Paris \
  jhmmt/pvmss:0.1
```

L'application sera disponible à l'adresse [http://localhost:50000](http://localhost:50000).

### 3. Consulter les logs

Pour voir les logs de l'application, exécutez :

```bash
docker compose logs -f pvmss
```

## Architecture

PVMSS suit une architecture client-serveur moderne avec des fonctionnalités avancées :

- **Backend** : Une application Go qui sert l'interface web et communique avec l'API Proxmox. Elle gère toute la logique métier, y compris l'authentification, les opérations VM, le rendu des templates et la fonctionnalité de proxy console.
- **Frontend** : HTML, CSS et JavaScript minimal standards. Utilise le [framework CSS Bulma](https://bulma.io/) pour un design propre et responsive avec noVNC intégré pour l'accès console.
- **Accès console** : Intégration noVNC intégrée avec proxy basé sur les sessions pour un accès console VM transparent, supportant les déploiements HTTP et HTTPS.
- **Gestion d'état** : Gestion d'état thread-safe avec injection de dépendances pour une opération robuste.
- **Conteneurisation** : L'application entière est empaquetée et déployée en tant que conteneur Docker unique.

## Limitations

- L'application est conçue pour être utilisée en tant que conteneur Docker unique.
- Seulement un seul hôte Proxmox est supporté (hors cluster).
- Il n'y a pas eu de tests rigoureux de sécurité, attention lors du déploiement.
- Pas de support de Cloud-Init.

## Licence

PVMSS  © 2025 by Julien HOMMET is licensed under Creative Commons Attribution-NonCommercial-NoDerivatives 4.0 International. To view a copy of this license, visit <https://creativecommons.org/licenses/by-nc-nd/4.0/>.

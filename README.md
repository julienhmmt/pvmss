# Proxmox VM Self-Service (PVMSS)

[![Lint](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml)

PVMSS is a lightweight, self-service web portal for Proxmox Virtual Environment (PVE). It allows users to perform basic virtual machine (VM) management tasks without needing direct access to the Proxmox web UI. The application is designed to be simple, fast, and easy to deploy as a Docker container.

## Architecture

PVMSS follows a simple client-server architecture:

-   **Backend**: A Go application that serves the web interface and communicates with the Proxmox API. It handles all business logic, including authentication, VM operations, and template rendering.
-   **Frontend**: Standard HTML, CSS, and minimal JavaScript. It uses the [Bulma CSS framework](https://bulma.io/) for a clean and responsive design. The frontend is rendered server-side using Go's `html/template` package.
-   **Containerization**: The entire application is packaged and deployed as a single Docker container using a multi-stage build for a small final image size.

## Project Structure

The repository is organized into the following key directories:

```
.
├── backend/         # All Go source code for the web server and API logic.
│   ├── main.go      # Main application entry point, server setup, and routing.
│   ├── i18n.go      # Internationalization (i18n) logic.
│   └── ...          # Other Go source files.
├── frontend/        # All static assets and HTML templates.
│   ├── css/         # CSS stylesheets.
│   ├── js/          # JavaScript files.
│   └── *.html       # HTML page and layout templates.
├── proxmox/         # Go package for interacting with the Proxmox API.
├── .env             # Local environment variables (you must create this).
├── docker-compose.yml # Docker Compose file for easy deployment.
└── Dockerfile       # Dockerfile for building the application image.
```

## Getting Started

Follow these instructions to get PVMSS running locally using Docker.

### Prerequisites

-   [Docker](https://docs.docker.com/get-docker/)
-   [Docker Compose](https://docs.docker.com/compose/install/)

### 1. Configure Environment Variables

Create a `.env` file in the root of the project. You can copy the provided example:

```sh
cp .env.example .env
```

Now, edit the `.env` file with your Proxmox API credentials and other settings:

-   `PROXMOX_URL`: The full URL to your Proxmox API endpoint (e.g., `https://proxmox.example.com:8006/api2/json`).
-   `PROXMOX_API_TOKEN_NAME`: The name of your Proxmox API token (e.g., `user@pve!token`).
-   `PROXMOX_API_TOKEN_VALUE`: The secret value of your API token.
-   `PVMSS_ADMIN_PASSWORD_HASH`: A bcrypt hash of the password for the admin panel. You can generate one using an online tool or a simple script.
-   `PROXMOX_VERIFY_SSL`: Set to `false` if you are using a self-signed certificate on Proxmox.
-   `LOG_LEVEL`: Set the application log level (`debug`, `info`, `warn`, `error`).

### 2. Build and Run the Container

With Docker running, execute the following command from the project root:

```sh
# Build the image and start the container in detached mode
docker-compose up --build -d
```

The application will be available at [http://localhost:50000](http://localhost:50000).

### 3. View Logs

To see the application logs, run:

```sh
docker-compose logs -f pvmss
```

## Features

### For Users

-   **VM Search**: Find virtual machines by VMID or name. Results are filtered to only show VMs tagged with `pvmss`.
-   **Detailed Information**: View key VM details like status, CPU, and memory.
-   **Multi-language**: The interface is available in English and French.

### For Administrators

-   **Tag Management**: Add or remove tags used to organize VMs.
-   **Resource Management**: Configure available ISOs, network bridges (VMBRs), and resource limits for VM creation.

---

# Proxmox VM Self-Service (PVMSS) - En Français

PVMSS est un portail web libre-service léger pour Proxmox Virtual Environment (PVE). Il permet aux utilisateurs d'effectuer des tâches de gestion de base sur leurs machines virtuelles (VM) sans nécessiter un accès direct à l'interface web de Proxmox. L'application est conçue pour être simple, rapide et facile à déployer en tant que conteneur Docker.

## Architecture

PVMSS suit une architecture client-serveur simple :

-   **Backend**: Une application Go qui sert l'interface web et communique avec l'API Proxmox. Elle gère toute la logique métier, y compris la connexion, les opérations sur les VM et le rendu des templates.
-   **Frontend**: HTML, CSS et JavaScript minimalistes. Il utilise le framework CSS [Bulma](https://bulma.io/) pour un design propre et réactif. Le frontend est rendu côté serveur à l'aide du package `html/template` de Go.
-   **Conteneurisation**: L'ensemble de l'application est packagé et déployé comme un unique conteneur Docker, utilisant un build multi-étapes pour une image finale de petite taille.

## Structure du Projet

Le dépôt est organisé comme suit :

```
.
├── backend/         # Code source Go du serveur web et de la logique API.
│   ├── main.go      # Point d'entrée, configuration du serveur et routage.
│   ├── i18n.go      # Logique d'internationalisation (i18n).
│   └── ...          # Autres fichiers source Go.
├── frontend/        # Assets statiques et templates HTML.
│   ├── css/         # Feuilles de style CSS.
│   ├── js/          # Fichiers JavaScript.
│   └── *.html       # Templates de page et de layout HTML.
├── proxmox/         # Package Go pour interagir avec l'API Proxmox.
├── .env             # Variables d'environnement locales (à créer).
├── docker-compose.yml # Fichier Docker Compose pour un déploiement facile.
└── Dockerfile       # Dockerfile pour construire l'image de l'application.
```

## Démarrage Rapide

Suivez ces instructions pour lancer PVMSS localement avec Docker.

### Prérequis

-   [Docker](https://docs.docker.com/get-docker/)
-   [Docker Compose](https://docs.docker.com/compose/install/)

### 1. Configurer les Variables d'Environnement

Créez un fichier `.env` à la racine du projet. Vous pouvez copier l'exemple fourni :

```sh
cp .env.example .env
```

Ensuite, modifiez le fichier `.env` avec vos identifiants d'API Proxmox et autres paramètres :

-   `PROXMOX_URL`: L'URL complète de votre API Proxmox (ex: `https://proxmox.example.com:8006/api2/json`).
-   `PROXMOX_API_TOKEN_NAME`: Le nom de votre jeton d'API Proxmox (ex: `user@pve!token`).
-   `PROXMOX_API_TOKEN_VALUE`: La valeur secrète de votre jeton d'API.
-   `PVMSS_ADMIN_PASSWORD_HASH`: Un hash bcrypt du mot de passe pour le panneau d'administration.
-   `PROXMOX_VERIFY_SSL`: Mettez à `false` si vous utilisez un certificat auto-signé sur Proxmox.
-   `LOG_LEVEL`: Définit le niveau de log de l'application (`debug`, `info`, `warn`, `error`).

### 2. Construire et Lancer le Conteneur

Avec Docker en cours d'exécution, lancez la commande suivante depuis la racine du projet :

```sh
# Construit l'image et démarre le conteneur en mode détaché
docker-compose up --build -d
```

L'application sera disponible à l'adresse [http://localhost:50000](http://localhost:50000).

### 3. Consulter les Logs

Pour voir les logs de l'application, exécutez :

```sh
docker-compose logs -f pvmss
```

## Fonctionnalités

### Pour les Utilisateurs

-   **Recherche de VM**: Trouvez des machines virtuelles par VMID ou par nom. Les résultats sont filtrés pour n'afficher que les VM avec le tag `pvmss`.
-   **Informations Détaillées**: Visualisez les détails clés des VM comme le statut, le CPU et la mémoire.
-   **Multilingue**: L'interface est disponible en anglais et en français.

### Pour les Administrateurs

-   **Gestion des Tags**: Ajoutez ou supprimez des tags pour organiser les VM.
-   **Gestion des Ressources**: Configurez les ISOs, les ponts réseau (VMBRs) et les limites de ressources disponibles pour la création de VM.

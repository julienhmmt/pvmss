# Proxmox VM Self-Service (PVMSS)

[![Lint](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml)

French version: [README.fr.md](README.fr.md)

PVMSS is a lightweight, self-service web portal for Proxmox Virtual Environment (PVE). It allows users to create and manage virtual machines (VMs) without needing direct access to the Proxmox web UI. The application is designed to be simple, fast, and easy to deploy as a container (Docker, Podman, Kubernetes).

⚠️ This application is currently in development and has limits, which are listed at the end of this document.

## Features

### For users

- **Create VM**: Create a new virtual machine with customizable resources (CPU, RAM, storage, ISO, network, tag).
- **VM console access**: Direct noVNC console access to virtual machines through an integrated web-based VNC client.
- **VM management**: Start, stop, restart, and delete virtual machines, update their resources.
- **VM search**: Find virtual machines by VMID or name.
- **VM details**: View comprehensive VM information including status, description, uptime, CPU, memory, disk usage, and network configuration.
- **Profile management**: View and manage own VM, reset password.
- **Multi-language**: The interface is available in French and English.

### For administrators

- **Node management**: Configure and manage Proxmox nodes available for VM deployment.
- **User pool management**: Add or remove users with automatic password generation.
- **Tag management**: Create and manage tags for VM organization.
- **ISO management**: Configure available ISO images for VM installation.
- **Network configuration**: Manage available network bridges (VMBRs) for VM networking, and the number of network interfaces per VM.
- **Storage management**: Configure storage locations for VM disks, and the number of disks per VM.
- **Resource limits**: Set CPU, RAM, and disk limits per Proxmox nodes and VM creation.
- **Documentation**: Admin documentation accessible from the admin panel.

## Getting started with Docker

Follow these instructions to get PVMSS running locally using Docker.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)

### Configure environment variables

You can use the provided example file `env.example` to create your own `.env` file. Or, you can modify the example file directly.

**Settings:**

- `ADMIN_PASSWORD_HASH`: A bcrypt hash of the password for the admin panel. You can generate one using an online tool or a simple script.
- `LOG_LEVEL`: Set the application log level: `INFO` or `DEBUG` (default: `INFO`).
- `PROXMOX_API_TOKEN_NAME`: The name of your Proxmox API token for backend operations (e.g., `user@pve!token`).
- `PROXMOX_API_TOKEN_VALUE`: The secret value of your API token.
- `PROXMOX_URL`: The full URL to your Proxmox API endpoint (e.g., `https://proxmox.example.com:8006/api2/json`).
- `PROXMOX_VERIFY_SSL`: Set to `false` if you are using a self-signed certificate on Proxmox (default: `false`).
- `PVMSS_ENV`: Set the application environment: `production`, `prod` (enables secure cookies and HSTS headers) or `development`, `dev`, `developpement` (default: `production`).
- `PVMSS_OFFLINE`: Set to `true` to enable offline mode (disables all Proxmox API calls). Useful for development or when Proxmox is unavailable (default: `false`).
- `PVMSS_SETTINGS_PATH`: Path to the settings file (default: `/app/settings.json`).
- `SESSION_SECRET`: Secret key for session encryption (change to a unique random string, like `$ openssl rand -hex 32`).

### Run the container

With Docker running, execute the following command from the project root:

```bash
# Create the `docker-compose.yml` file and paste this content:
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

# Start the container in detached mode
docker compose up -d
```

Or run the container with `docker run`:

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

The application will be available at [http://localhost:50000](http://localhost:50000).

### View logs

To see the application logs, run:

```bash
docker logs -f pvmss
```

## Deployment of the application in Kubernetes

A manifest containing all the necessary information is available at the root of this repository, named `pvmss-deployment.yaml`. This manifest contains :

- A namespace
- A service account
- A secret
- A configmap
- A persistent volume claim (pvc)
- A deployment
- A service

To deploy the application in Kubernetes, run the following command :

```bash
kubectl apply -f pvmss-deployment.yaml
```

The management of the ingress or gateway-api is up to you. An example of manifest http-route is available at the root of this repository, named `pvmss-httproute.yml`.

## Limitations / To Do list

- There are no security tests done, be careful using this app.
- No Cloud-Init support (yet).
- Only one node Proxmox is currently supported. Proxmox cluster are not yet correctly handled.
- No OpenID Connect support (yet).
- Need a better logging system, with the ability to log to a file and to be sent to a remote server (syslog like format).

## License

PVMSS by Julien HOMMET is licensed under Creative Commons Attribution-NonCommercial-NoDerivatives 4.0 International. To view a copy of this license, visit <https://creativecommons.org/licenses/by-nc-nd/4.0/>

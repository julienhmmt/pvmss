# Proxmox VM Self-Service (PVMSS)

[![Lint](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml)

French version: [README.fr.md](README.fr.md)

PVMSS is a lightweight, self-service web portal for Proxmox Virtual Environment (PVE). It allows users to create and manage virtual machines (VMs) without needing direct access to the Proxmox web UI. The application is designed to be simple, fast, and easy to deploy as a Docker container.

⚠️ This application is currently in development and has limits, which are listed at the end of this document.

## Features

### For users

- **Create VM**: Create a new virtual machine with customizable resources (CPU, RAM, storage).
- **VM Console Access**: Direct noVNC console access to virtual machines through an integrated web-based VNC client.
- **VM Management**: Start, stop, restart, and delete virtual machines.
- **VM Search**: Find virtual machines by VMID or name.
- **VM Details**: View comprehensive VM information including status, description, uptime, CPU, memory, disk usage, and network configuration.
- **Profile Management**: View and manage own VM, reset password.
- **Multi-language**: The interface is available in English and French with automatic language detection.

### For administrators

- **Node Management**: Configure and manage Proxmox nodes available for VM deployment.
- **User Pool Management**: Add or remove users with automatic password generation.
- **Tag Management**: Create and manage tags for VM organization.
- **ISO Management**: Configure available ISO images for VM installation.
- **Network Configuration**: Manage available network bridges (VMBRs) for VM networking.
- **Storage Management**: Configure storage locations for VM disks.
- **Resource Limits**: Set CPU, RAM, and disk limits for VM creation.
- **Documentation**: Built-in user documentation accessible from the admin panel.

## Getting started

Follow these instructions to get PVMSS running locally using Docker.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)

### 1. Configure environment variables

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
- `SESSION_SECRET`: Secret key for session encryption (change to a unique random string, like `$ openssl rand -hex 32`).

### 2. Run the container

With Docker running, execute the following command from the project root:

```bash
# Start the container in detached mode
docker compose up -d
```

Or run the container with `docker run`:

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

The application will be available at [http://localhost:50000](http://localhost:50000).

### 3. View logs

To see the application logs, run:

```bash
docker compose logs -f pvmss
```

## Architecture

PVMSS follows a modern client-server architecture with advanced features:

- **Backend**: A Go application that serves the web interface and communicates with the Proxmox API. It handles all business logic, including authentication, VM operations, template rendering, and console proxy functionality.
- **Frontend**: Standard HTML, CSS, and minimal JavaScript. It uses the [Bulma CSS framework](https://bulma.io/) for a clean and responsive design with integrated noVNC for console access.
- **Console Access**: Built-in noVNC integration with session-based proxy for seamless VM console access, supporting both HTTP and HTTPS deployments.
- **State Management**: Thread-safe state management with dependency injection for robust operation.
- **Containerization**: The entire application is packaged and deployed as a single Docker container.

## Limitations

- This application is designed to be used as a single Docker container.
- Only one Proxmox host is supported (not clusters).
- There are no security tests done, be careful using this app.
- No Cloud-Init support.

## License

PVMSS  © 2025 by Julien HOMMET is licensed under Creative Commons Attribution-NonCommercial-NoDerivatives 4.0 International. To view a copy of this license, visit <https://creativecommons.org/licenses/by-nc-nd/4.0/>

# Proxmox VM Self-Service (PVMSS)

[![Lint](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml)

PVMSS is a lightweight, self-service web portal for Proxmox Virtual Environment (PVE). It allows users to create and manage virtual machines (VMs) without needing direct access to the Proxmox web UI. The application is designed to be simple, fast, and easy to deploy as a Docker container.

## Architecture

PVMSS follows a modern client-server architecture with advanced features:

- **Backend**: A Go application that serves the web interface and communicates with the Proxmox API. It handles all business logic, including authentication, VM operations, template rendering, and console proxy functionality.
- **Frontend**: Standard HTML, CSS, and minimal JavaScript. It uses the [Bulma CSS framework](https://bulma.io/) for a clean and responsive design with integrated noVNC for console access.
- **Console Access**: Built-in noVNC integration with session-based proxy for seamless VM console access, supporting both HTTP and HTTPS deployments.
- **State Management**: Thread-safe state management with dependency injection for robust operation.
- **Containerization**: The entire application is packaged and deployed as a single Docker container.

## Getting Started

Follow these instructions to get PVMSS running locally using Docker.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)

### 1. Configure environment variables

Create a `.env` file in the root of the project. You can copy the provided example:

```bash
cp env.example .env
```

Now, edit the `.env` file with your Proxmox API credentials and other settings:

**Required Settings:**

- `PROXMOX_URL`: The full URL to your Proxmox API endpoint (e.g., `https://proxmox.example.com:8006/api2/json`).
- `PROXMOX_USER`: Proxmox username for user authentication (e.g., `root@pam`).
- `PROXMOX_PASSWORD`: Proxmox password for user authentication.
- `PROXMOX_API_TOKEN_NAME`: The name of your Proxmox API token for backend operations (e.g., `user@pve!token`).
- `PROXMOX_API_TOKEN_VALUE`: The secret value of your API token.
- `ADMIN_PASSWORD_HASH`: A bcrypt hash of the password for the admin panel. You can generate one using an online tool or a simple script.
- `SESSION_SECRET`: Secret key for session encryption (change to a unique random string).

**Optional Settings:**

- `PROXMOX_VERIFY_SSL`: Set to `false` if you are using a self-signed certificate on Proxmox (default: `true`).
- `PROXMOX_PORT`: Proxmox server port (default: `8006`).
- `LOG_LEVEL`: Set the application log level: `INFO` or `DEBUG` (default: `INFO`).
- `PORT`: PVMSS server port (default: `50000`).

### 2. Build and run the container

With Docker running, execute the following command from the project root:

```bash
# Build the image and start the container in detached mode
docker-compose up --build -d
```

The application will be available at [http://localhost:50000](http://localhost:50000).

### 3. View logs

To see the application logs, run:

```bash
docker-compose logs -f pvmss
```

## Features

### For users

- **Create VM**: Create a new virtual machine with customizable resources (CPU, RAM, storage).
- **VM Console Access**: Direct noVNC console access to virtual machines through an integrated web-based VNC client.
- **VM Management**: Start, stop, restart, and delete virtual machines.
- **VM Search**: Find virtual machines by VMID or name with real-time search.
- **VM Details**: View comprehensive VM information including status, uptime, CPU, memory, disk usage, and network configuration.
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

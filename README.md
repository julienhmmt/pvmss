# Proxmox VM Self-Service (PVMSS)

[![Lint](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml)

PVMSS is a lightweight, self-service web portal for Proxmox Virtual Environment (PVE). It allows users to create and manage virtual machines (VMs) without needing direct access to the Proxmox web UI. The application is designed to be simple, fast, and easy to deploy as a Docker container.

## Architecture

PVMSS follows a simple client-server architecture:

- **Backend**: A Go application that serves the web interface and communicates with the Proxmox API. It handles all business logic, including authentication, VM operations, and template rendering.
- **Frontend**: Standard HTML, CSS, and minimal JavaScript. It uses the [Bulma CSS framework](https://bulma.io/) for a clean and responsive design.
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

- `PROXMOX_URL`: The full URL to your Proxmox API endpoint (e.g., `https://proxmox.example.com:8006/api2/json`).
- `PROXMOX_API_TOKEN_NAME`: The name of your Proxmox API token (e.g., `user@pve!token`).
- `PROXMOX_API_TOKEN_VALUE`: The secret value of your API token.
- `PVMSS_ADMIN_PASSWORD_HASH`: A bcrypt hash of the password for the admin panel. You can generate one using an online tool or a simple script.
- `PROXMOX_VERIFY_SSL`: Set to `false` if you are using a self-signed certificate on Proxmox.
- `LOG_LEVEL`: Set the application log level (`debug`, `info`, `warn`, `error`).

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

- **Create VM**: Create a new virtual machine.
- **VM search**: Find virtual machines by VMID or name.
- **VM details**: View key VM details like status, CPU, and memory.
- **Multi-language**: The interface is available in English and French.

### For administrators

- **User management**: Add or remove users.
- **Tag management**: Add or remove tags used to organize VMs.
- **Resource management**: Configure available ISOs, network bridges (VMBRs), and resource limits for VM creation.

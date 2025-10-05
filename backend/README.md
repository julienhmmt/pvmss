# PVMSS - Backend

Proxmox VM Self-Service backend service built with Go.

## 🚀 Quick Start

### Prerequisites

- Go 1.24+
- Docker
- Proxmox VE 7.0+ cluster

### Using Docker

Build and run with Docker:

```bash
docker build -t pvmss-backend .
docker run -p 50000:50000 pvmss-backend
```

## 📡 API Endpoints

### Public Routes

- `GET /` - Main landing page
- `GET /health` - Health check endpoint
- `GET /login` - User login page
- `POST /login` - User authentication
- `GET /admin/login` - Admin login page
- `POST /admin/login` - Admin authentication
- `GET /logout` - User logout
- `GET /css/*` - Static CSS files
- `GET /js/*` - Static JavaScript files
- `GET /components/*` - Static component files (noVNC, etc.)

### User Routes (Authenticated)

- `GET /search` - VM search page
- `POST /search` - Search VMs by name or VMID
- `GET /create` - VM creation form
- `POST /create` - Create new VM
- `GET /vm/details` - VM details page
- `GET /vm/status` - Get VM status
- `POST /vm/start` - Start VM
- `POST /vm/stop` - Stop VM
- `POST /vm/restart` - Restart VM
- `POST /vm/shutdown` - Graceful VM shutdown
- `POST /vm/delete` - Delete VM
- `POST /vm/console` - Get console access URL
- `GET /vm/console-window` - Console window page
- `GET /vm/console-proxy` - HTTP proxy for noVNC
- `GET /vm/console-websocket` - WebSocket proxy for VNC
- `GET /profile` - User profile page
- `POST /profile/update` - Update user profile
- `GET /docs` - User documentation

### Admin Routes (Admin Authenticated)

- `GET /admin` - Admin dashboard
- `GET /admin/nodes` - Node management
- `POST /admin/nodes/add` - Add node
- `POST /admin/nodes/remove` - Remove node
- `GET /admin/tags` - Tag management
- `POST /admin/tags/add` - Add tag
- `POST /admin/tags/remove` - Remove tag
- `GET /admin/iso` - ISO management
- `POST /admin/iso/add` - Add ISO
- `POST /admin/iso/remove` - Remove ISO
- `GET /admin/vmbr` - Network bridge management
- `POST /admin/vmbr/add` - Add VMBR
- `POST /admin/vmbr/remove` - Remove VMBR
- `GET /admin/storage` - Storage management
- `POST /admin/storage/add` - Add storage
- `POST /admin/storage/remove` - Remove storage
- `GET /admin/limits` - Resource limits configuration
- `POST /admin/limits/update` - Update resource limits
- `GET /admin/userpool` - User pool management
- `POST /admin/userpool/add` - Add user
- `POST /admin/userpool/remove` - Remove user
- `GET /admin/docs` - Admin documentation

## 🔧 Configuration

### Required Environment Variables

- `PROXMOX_URL` - Proxmox API URL (e.g., `https://proxmox.example.com:8006/api2/json`)
- `PROXMOX_USER` - Proxmox username for authentication (e.g., `root@pam`)
- `PROXMOX_PASSWORD` - Proxmox password
- `PROXMOX_API_TOKEN_NAME` - Proxmox API token name (e.g., `user@pve!token`)
- `PROXMOX_API_TOKEN_VALUE` - Proxmox API token secret
- `ADMIN_PASSWORD_HASH` - Bcrypt hash of admin password
- `SESSION_SECRET` - Secret key for session encryption

### Optional Environment Variables

- `PORT` - Server port (default: `50000`)
- `LOG_LEVEL` - Logging level: `INFO` or `DEBUG` (default: `INFO`)
- `PROXMOX_VERIFY_SSL` - Verify SSL certificates (default: `true`, set to `false` for self-signed certs)
- `PROXMOX_PORT` - Proxmox server port (default: `8006`)

## 🐳 Docker Compose

```bash
docker compose up --build --no-cache
```

## 📦 Dependencies

### Core Dependencies

- [Telmate Proxmox](https://github.com/Telmate/proxmox-api-go) - Proxmox VE API client library
- [zerolog](https://github.com/rs/zerolog) - Fast and structured logging
- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket implementation for VNC proxy
- [gorilla/sessions](https://github.com/gorilla/sessions) - Session management
- [gorilla/csrf](https://github.com/gorilla/csrf) - CSRF protection
- [golang.org/x/crypto](https://golang.org/x/crypto) - Cryptographic operations (bcrypt)
- [nicksnyder/go-i18n](https://github.com/nicksnyder/go-i18n) - Internationalization support
- [joho/godotenv](https://github.com/joho/godotenv) - Environment variable loading

## 🔄 Development

Update dependencies:

```bash
go get -u
go mod tidy
```

## 📝 License

MIT

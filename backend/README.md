# PVMSS - Backend

Proxmox VM Self-Service backend service.

## 🚀 Quick Start

### Prerequisites

- Go 1.24+
- Docker

### Using Docker

Build and run with Docker:

```bash
docker build -t pvmss-backend .
docker run -p 50000:50000 pvmss-backend
```

## 📡 API Endpoints

- `GET /` - Service status
- `GET /health` - Health check

## 🔧 Configuration

Environment variables:
- `PORT` - Server port (default: `50000`)
- `LOG_LEVEL` - Logging level (default: `info`)

## 🐳 Docker Compose

```bash
docker compose up --build --no-cache
```

## 📦 Dependencies

- [Telmate Proxmox](https://github.com/Telmate/proxmox-api-go) - Telmate's Golang library to use hthe Proxmox VE API
- [zerolog](https://github.com/rs/zerolog) - Fast and structured logging

## 🔄 Development

Update dependencies:

```bash
go get -u
go mod tidy
```

## 📝 License

MIT

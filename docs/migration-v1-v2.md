# Migration guide: v1 → v2

PVMSS v2 replaces the monolithic `settings.json` configuration file with a
two-tier system:

- **Environment variables** for secrets and infrastructure settings (Proxmox
  credentials, signing keys).
- **Embedded SQLite database** (`pvmss.db`) for all user-editable configuration
  (nodes, storages, limits, tags, cloud-init templates, VM profiles, …).

This document walks through upgrading an existing v1 deployment.

---

## Before you start

1. **Back up your data.**

   ```sh
   # Docker Compose
   cp settings.json settings.json.v1-backup
   cp pvmss.db pvmss.db.bak   # if it already exists

   # Kubernetes
   kubectl exec -n pvmss deploy/pvmss -- \
     cp /data/pvmss.db /data/pvmss.db.bak
   ```

2. Note your current `settings.json` path (typically `/app/settings.json` or
   `/data/settings.json` depending on deployment).

---

## Step 1 — Extract the JWT secret from `settings.json`

v1 stored the JWT signing key inside `settings.json`:

```json
{
  "jwt_secret": "…your-old-secret…"
}
```

Copy the value — you will set it as the `JWT_SECRET` environment variable.
**Important**: The `JWT_SECRET` must be at least 32 bytes. If your v1 `jwt_secret` is shorter, generate a new one using `openssl rand -hex 48`.

**Note**: v2 still reads `settings.json` on first boot to automatically migrate your configuration to the new database. After migration completes, `settings.json` is no longer used and can be safely removed.

---

## Step 2 — Set required environment variables

Add the following variables to your deployment (Docker Compose `.env` /
`environment:` block, Kubernetes Secret, Helm `values.yaml`, etc.).

| Variable | Description | Required |
| --- | --- | --- |
| `JWT_SECRET` | Signing key for `/api/v1/` tokens (min 32 chars) | ✅ |
| `SESSION_SECRET` | Encryption key for server-side sessions (min 32 chars) | ✅ |
| `ADMIN_PASSWORD_HASH` | bcrypt hash of the admin password (starts with `$2`) | ✅ |
| `PROXMOX_URL` | Proxmox API base URL, e.g. `https://pve.example.com:8006/api2/json` | ✅ (unless `PVMSS_OFFLINE=true`) |
| `PROXMOX_API_TOKEN_NAME` | Proxmox API token ID, e.g. `root@pam!pvmss` | ✅ (unless offline) |
| `PROXMOX_API_TOKEN_VALUE` | Proxmox API token secret | ✅ (unless offline) |
| `PVMSS_DB_PATH` | Path to the SQLite file (default: `/data/pvmss.db`) | optional |

### Generate fresh secrets

```sh
# JWT_SECRET / SESSION_SECRET — 48 random hex chars (> 32 bytes)
openssl rand -hex 48

# Admin password hash
htpasswd -bnBC 12 "" '<your-password>' | tr -d ':\n'
```

---

## Step 3 — Mount a persistent volume for the database

The SQLite database must survive container restarts.

### Docker Compose

```yaml
services:
  pvmss:
    environment:
      PVMSS_DB_PATH: "/data/pvmss.db"
    volumes:
      - pvmss_data:/data        # persistent named volume

volumes:
  pvmss_data: {}
```

Remove the old `./settings.json:/app/settings.json` bind mount.
The `PVMSS_SETTINGS_PATH` variable is no longer needed for ongoing operation, but may still be used for migration detection if you keep the old file in a custom location.

### Kubernetes

The `pvmss-data-pvc` PersistentVolumeClaim (replaces `pvmss-settings-pvc`)
mounts to `/data`.  Update your manifest or re-apply the bundled
`pvmss-deployment.yaml`.

### Helm

```sh
helm upgrade pvmss ./helm \
  --set secrets.jwtSecret="<JWT_SECRET>" \
  --set secrets.sessionSecret="<SESSION_SECRET>" \
  --set secrets.adminPasswordHash="<ADMIN_PASSWORD_HASH>" \
  --set secrets.pveApiToken="<PROXMOX_API_TOKEN_VALUE>" \
  --set deployment.env.PROXMOX_URL="https://pve.example.com:8006/api2/json" \
  --set deployment.env.PROXMOX_API_TOKEN_NAME="root@pam!pvmss"
```

---

## Step 4 — Deploy the new version

```sh
# Docker Compose
docker compose pull
docker compose up -d

# Kubernetes
kubectl apply -f pvmss-deployment.yaml

# Helm
helm upgrade pvmss ./helm --values values.yaml
```

On first start with a `settings.json` present, PVMSS automatically migrates
all settings into SQLite and marks bootstrap as complete.  A log line like the
following confirms success:

```text
INFO migration complete: 3 nodes, 5 ISOs, 2 VMBRs, 4 tags, …
```

---

## Step 5 — Verify the admin UI

1. Open `http(s)://<pvmss-host>/admin`.
2. Check **Nodes**, **Storage**, **Tags**, **Limits** — all existing
   configuration should be present.
3. Make a small change (e.g. add a tag) and confirm it persists after a
   container restart.

---

## Step 6 — Remove `settings.json`

Once you have verified that the admin UI reflects your old configuration:

```sh
# Docker Compose — remove the bind mount, then delete the file
docker compose down
# edit docker-compose.yml: remove the settings.json volume entry
docker compose up -d
rm settings.json          # optional — the file is now ignored

# Kubernetes — delete the old ConfigMap if you created one
kubectl delete configmap pvmss-config -n pvmss
```

---

## Rollback procedure

If you need to revert to v1:

1. Stop the v2 container.
2. Restore the binary and `settings.json` backup.
3. Restore the old env vars (remove `JWT_SECRET`; put the value back into
   `settings.json` under `jwt_secret`).
4. Start the v1 container.

The SQLite database (`pvmss.db`) is ignored by v1 and will not cause problems.

---

## Troubleshooting

| Symptom | Cause | Fix |
| --- | --- | --- |
| `JWT_SECRET is required` on startup | Missing env var | Set `JWT_SECRET` |
| `JWT_SECRET must be at least 32 bytes` | Secret too short | Generate a longer secret |
| Admin UI shows empty lists | Migration not triggered | Ensure `settings.json` was readable on first boot |
| Settings lost after restart | DB not on persistent volume | Check volume mount for `/data` |
| `pvmss.db` permission denied | Wrong file ownership | `chown 65532:65532 pvmss.db` |

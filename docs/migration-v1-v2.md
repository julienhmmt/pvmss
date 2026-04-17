# Migration Guide: v1 to v2 (Settings Architecture)

This guide explains the migration from the legacy settings.json-based configuration to the new SQLite database-backed settings system.

## Overview

The v2 architecture introduces a database-backed settings system with:
- SQLite database for persistent configuration storage
- In-memory cache for zero-latency reads
- Audit log for tracking all configuration changes
- Typed API endpoints for each settings section
- Unified admin settings panel with grouped sections

## Why Deletions Are Disabled in the Unified Panel

The unified settings panel (Phase 9) intentionally disables delete operations for the following reasons:

### 1. Data Integrity

- **Referential Integrity**: Many settings are referenced by running VMs or user workflows. Deleting them could break existing deployments.
- **Audit Trail**: Disabling deletions ensures a complete history of all configuration changes is preserved.
- **Recovery**: Deleted data cannot be recovered without database backups.

### 2. Safe Alternative: Disable Instead

Instead of deleting, use the `enabled` flag to disable items:
- **Cloud-Init Templates**: Set `enabled: false` to hide from VM creation UI
- **VM Profiles**: Set `enabled: false` to hide from VM creation UI
- **Inventory Items**: Remove from enabled lists (nodes, storages, ISOs, network bridges, tags)

This approach:
- Preserves the configuration for future reference
- Allows easy re-enabling if needed
- Maintains audit trail continuity
- Prevents accidental data loss

### 3. Future Considerations

If delete functionality is needed in the future, it should:
- Require explicit confirmation with warnings about dependencies
- Check for active VMs or workflows using the item
- Archive deleted items rather than permanently removing them
- Require admin privileges with elevated permissions

## Migration Steps

### 1. Automatic Migration

On first run with v2:
- The application automatically migrates existing settings.json to SQLite
- All configuration is preserved with the same values
- The original settings.json is not modified (backup recommended)

### 2. Manual Export/Import

If you need to transfer configuration between instances:

```bash
# Export database
curl -X GET http://localhost:50001/api/v1/admin/db/export -o pvmss.db.backup

# Import database
curl -X POST http://localhost:50001/api/v1/admin/db/import \
  -F "file=@pvmss.db.backup"
```

### 3. Environment Variables

Ensure these environment variables are set:

```bash
# Required for v2
PVMSS_DB_PATH=/app/data/pvmss.db
PVMSS_JWT_SECRET=your-secret-key

# Proxmox credentials (unchanged)
PVE_HOST=https://pve.example.com:8006
PVE_USER=administrator@pve
PVE_PASSWORD=your-password
PVE_SKIP_TLS_VERIFY=false
```

## API Changes

### New Endpoints

- `GET /api/v1/admin/settings/overview` - Unified settings snapshot
- `POST /api/v1/admin/settings/upsert` - Add/update settings
- `GET /api/v1/admin/db/export` - Export database backup
- `POST /api/v1/admin/db/import` - Import database backup
- `GET /api/v1/admin/audit` - Query audit log

### Deprecated Endpoints (Phase 8)

These legacy endpoints will be removed after one release cycle:
- `GET /api/v1/admin/settings` (JSON dump of all settings)
- `POST /api/v1/admin/settings` (Bulk settings update)

## Rollback

If you need to rollback to v1:

1. Stop the v2 application
2. Restore your settings.json backup
3. Start the v1 application with the old configuration

The SQLite database file is the only new state - no other files are modified.

## Support

For issues or questions about the migration:
- Check the audit log for change history
- Export the database for debugging
- Review the Phase 9 documentation in the tasks file

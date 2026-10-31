# Cloud-init - removed

The cloud-init feature (paused 2026-10-20) was deleted on 2026-10-31. The
full feature is recoverable from git history before the removal commits.

Deleted end to end:

- admin cloud-init templates (`/admin/cloudinit-templates`,
  `catalog_cloudinit_templates` table)
- the generated PVMSS baseline (`/admin/baseline`, `vm_baseline_state` table)
- per-VM cloud-init writes (PUT `/cloudinit`, document and SSH-key routes,
  `cicustom` attachment, `SetCloudInitConfig`/`AttachCloudInitSnippet`)
- cloud-image catalog and image-based VM creation (`/admin/images`,
  `catalog_images` table, `import-from` create path, `ImageSpec`)
- the cluster snippet-storage setting (`clusters.snippet_storage` column,
  `GET /api/v1/admin/snippet-storages`) and every snippet check/presence API
- the admin cluster form's cloud-init fields and the `cloudInitCapable`
  template flag

Kept on purpose:

- `GET /api/v1/vms/{cluster}/{vmid}/cloudinit` - the VM Connect tab reads
  the SSH user from the live cloud-init config.
- `POST /api/v1/vms/{cluster}/{vmid}/console-password` and its guest-agent
  helpers (`SetCloudInitPassword`, `PingGuestAgent`, `GetCloudInitConfig`).
- Profile SSH-key API (`GET/POST/DELETE /api/v1/profile/ssh-keys`) and the
  `cloudinit` package's SSH-key validation/slug helpers.
- The `vm_cloudinit_snippets` and `vm_cloudinit_documents` tables, so the
  VM-delete path can still warn an operator about `pvmss-*.yml` files a
  pre-removal deployment left on the nodes. Remove those files by hand on
  the nodes; PVMSS never creates new ones.

Why it went: the Proxmox API cannot write `snippets` files, so the admin had
to copy-paste content onto nodes by hand - too fragile and too complex to
operate. If a VM-access feature returns, it should drive the guest agent or
the native `ciuser`/`sshkeys`/`ipconfig0` keys at create time instead.

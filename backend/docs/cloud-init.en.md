# Cloud-Init support in PVMSS

## Overview

PVMSS provides cloud-init support for Proxmox VMs, allowing users to apply advanced cloud-init configurations during VM creation. This feature enables automated VM provisioning with:

- **Custom users and SSH keys**
- **Network configuration** (DHCP or static IP)
- **Custom scripts and packages** via YAML templates
- **Advanced cloud-init configurations** using the `cicustom` parameter

This feature is optional and requires additional setup due to limitations in the Proxmox API.

## Why Cloud-Init is optional and requires setup

### Proxmox API limitations

The Proxmox API has significant limitations regarding cloud-init snippet uploads:

- The standard `/nodes/{node}/storage/{storage}/upload` endpoint with `content=snippets` is not supported in many Proxmox versions
- Even when snippets storage is configured and enabled, the API may return HTTP 400 "bad request" errors
- This limitation affects all Proxmox users regardless of their storage configuration

### Workaround: SSH/SFTP upload

Due to these API limitations, PVMSS implements cloud-init snippet upload via SFTP (SSH File Transfer Protocol), similar to how popular Terraform providers (Telmate and bpg/proxmox) handle this limitation:

- **Telmate provider**: Uses SSH/SCP to upload snippets directly to `/var/lib/vz/snippets/`
- **bpg provider**: Uses SFTP with PAM accounts, documented as "Snippets Cannot Be Uploaded by Non-PAM Accounts"

You can read more about this approach in the Terraform bpg provider documentation: <https://registry.terraform.io/providers/bpg/proxmox/latest/docs#ssh-connection>

> **Note**: PVMSS uses the Go `pkg/sftp` library which implements the SFTP protocol natively. No SSH or SFTP binaries are required in the PVMSS container - only the private key file needs to be accessible.

This approach requires:

1. A dedicated PAM account on Proxmox nodes
2. SSH key pair (generated on the Proxmox node)
3. Private key mounted into the PVMSS container
4. Proper filesystem permissions for the snippets directory

## Setup requirements

All commands below are executed on the Proxmox node as `root`.

### Step 1: Create the PAM account

```bash
# Create dedicated user (no shell login)
useradd --create-home --shell /usr/sbin/nologin pvmss-snippets

# Create .ssh directory
mkdir -p /home/pvmss-snippets/.ssh
chmod 700 /home/pvmss-snippets/.ssh
chown pvmss-snippets:pvmss-snippets /home/pvmss-snippets/.ssh
```

### Step 2: Generate SSH key pair

Generate the key pair directly on the Proxmox node. The private key will be mounted into the PVMSS container.

```bash
# Create directory for PVMSS SSH keys (will be mounted to the container)
mkdir -p /etc/pvmss/keys
chmod 700 /etc/pvmss/keys

# Generate Ed25519 key pair (no passphrase for automation)
ssh-keygen -t ed25519 -f /etc/pvmss/keys/pvmss_snippets_ed25519 -N "" -C "pvmss-snippets@pvmss"

# Set proper permissions
chmod 600 /etc/pvmss/keys/pvmss_snippets_ed25519
chmod 644 /etc/pvmss/keys/pvmss_snippets_ed25519.pub
```

### Step 3: Add public key to authorized_keys

```bash
# Copy the public key to the user's authorized_keys
cat /etc/pvmss/keys/pvmss_snippets_ed25519.pub >> /home/pvmss-snippets/.ssh/authorized_keys
chmod 600 /home/pvmss-snippets/.ssh/authorized_keys
chown pvmss-snippets:pvmss-snippets /home/pvmss-snippets/.ssh/authorized_keys
```

### Step 4: Configure SSH server for SFTP-only access

Since the user has `/usr/sbin/nologin` as shell, you must configure SSH to use `internal-sftp` subsystem:

```bash
# Edit SSH server configuration
nano /etc/ssh/sshd_config
```

Add at the end of the file:

```ini
# PVMSS SFTP-only access for cloud-init snippets
Match User pvmss-snippets
    ForceCommand internal-sftp
    ChrootDirectory /var/lib/vz
    AllowTcpForwarding no
    X11Forwarding no
```

Restart SSH:

```bash
systemctl reload sshd
```

> **Important**: With `ChrootDirectory /var/lib/vz`, the SFTP user sees `/var/lib/vz` as root (`/`). The path in PVMSS settings becomes `/snippets` instead of `/var/lib/vz/snippets`.

### Step 5: Set filesystem permissions for snippets directory

```bash
# Install ACL tools if not already installed
apt install acl

# Alternatively, use ACLs for fine-grained permissions
setfacl -m u:pvmss-snippets:rwx /var/lib/vz/snippets
setfacl -R -m u:pvmss-snippets:rwX /var/lib/vz/snippets

# Verify permissions
sudo -u pvmss-snippets ls -la /var/lib/vz/snippets
sudo -u pvmss-snippets touch /var/lib/vz/snippets/test-pvmss && rm /var/lib/vz/snippets/test-pvmss && echo "OK"
```

### Step 6: Mount the private key into the PVMSS container

⚠️ Sensitive information: The private key file contains credentials that must be kept secure.

Mount the private key into the container:

```yaml
# docker-compose.yml
services:
  pvmss:
    image: jhmmt/pvmss:1.0
    volumes:
      - ./pvmss_snippets_ed25519:/app/pvmss_snippets_ed25519:ro
    # ... other configuration
```

Or with `docker run`:

```bash
docker run \
  -v ./pvmss_snippets_ed25519:/app/pvmss_snippets_ed25519:ro \
  jhmmt/pvmss:1.0
```

### Step 6a: Kubernetes deployment

Get the content of the private key file, store it into a local file, and create a Kubernetes secret:

```bash
kubectl -n pvmss create secret generic pvmss-snippets-key \
  --from-file=private_key=pvmss_snippets_ed25519
```

Then reference this secret in your deployment configuration by mounting it as a volume:

```yaml

```

### Step 7: Configure PVMSS settings

In your `settings.json`, enable SFTP and configure the connection:

```json
{
  "cloudinit_sftp": {
    "enabled": true,
    "host": "YOUR_PROXMOX_NODE_IP_OR_HOSTNAME",
    "port": 22,
    "username": "pvmss-snippets",
    "privateKeyPath": "/app/pvmss_snippets_ed25519",
    "snippetBaseDir": "/snippets"
  }
}
```

> **Note**: If you configured `ChrootDirectory /var/lib/vz` in Step 4, use `/snippets` as the `snippetBaseDir`. If you did NOT use chroot, use the full path `/var/lib/vz/snippets`.

### Configuration options

| Option | Description | Example |
|--------|-------------|--------|
| `enabled` | Enable/disable SFTP uploads | `true` |
| `host` | Proxmox node IP or hostname | `"192.168.1.100"` |
| `port` | SSH port | `22` |
| `username` | PAM user for SFTP | `"pvmss-snippets"` |
| `privateKeyPath` | Path to private key inside container | `"/app/pvmss_snippets_ed25519"` |
| `snippetBaseDir` | Snippets directory (relative to chroot if used) | `"/snippets"` |

### Disabling SFTP (basic cloud-init only)

If you cannot configure SFTP, disable it and use only basic cloud-init parameters:

```json
{
  "cloudinit_sftp": {
    "enabled": false
  }
}
```

With this configuration, you can still use `ciuser`, `cipassword`, `sshkeys`, and `ipconfig0`, but not custom YAML templates.

## How it works

### Template management (admin)

1. **Create templates**: Admins create cloud-init templates in the admin panel (`/admin/cloudinit`)
2. **YAML validation**: Templates must start with `#cloud-config` header and contain valid YAML
3. **Enable/disable**: Templates can be enabled or disabled without deletion
4. **Local storage**: Templates are stored in `settings.json` (not on Proxmox storage)

### VM creation (user)

1. **Template selection**: User selects a cloud-init template in the VM creation form
2. **Snippet upload**: PVMSS uploads the YAML content via SFTP to the Proxmox node's snippets directory (filename: `pvmss-{vmid}.yml`)
3. **VM configuration**: PVMSS creates the VM with cloud-init parameters:
   - `ciuser`, `cipassword`, `sshkeys` for user configuration
   - `ipconfig0` for network configuration
   - `cicustom` parameter pointing to the uploaded snippet (e.g., `user=local:snippets/pvmss-100.yml`)
4. **Cloud-init drive**: Proxmox creates an ISO containing the cloud-init configuration
5. **Boot process**: The VM boots and cloud-init applies the configuration

## Security considerations

- **Dedicated account**: The `pvmss-snippets` account has no shell access and is restricted to SFTP
- **Minimal permissions**: The account only has write access to the snippets directory
- **SSH key authentication**: No passwords are stored or transmitted
- **Chroot isolation**: The SFTP user is chrooted to `/var/lib/vz`, limiting filesystem access
- **Automatic cleanup**: Cloud-init snippets are automatically deleted when the associated VM is deleted

## Troubleshooting

### Common issues

1. **Permission denied**: Check filesystem permissions on the snippets directory
2. **SSH authentication failed**: Verify the SSH key is properly configured and accessible
3. **Snippet not found**: Ensure the storage supports snippets content type
4. **Network issues**: Verify PVMSS can reach the Proxmox node on port 22

## Limitations

- **SSH connection required**: Requires SSH access to Proxmox nodes for snippet upload
- **Single node**: Currently supports snippet upload to a single Proxmox node (multi-node cluster support is limited)
- **Template validation**: Only YAML syntax is validated, not cloud-init semantic correctness
- **No template variables**: Templates are static; user-specific values must be set via the basic cloud-init fields (user, password, SSH keys)

### SFTP upload fails

1. **Check SFTP status**: Go to `/admin/cloudinit` to see the SFTP configuration status
2. **Verify private key**: Ensure the key file exists and has correct permissions (600)
3. **Test SFTP connection manually**:

   ```bash
   # Test SFTP connection (not SSH - the user has no shell)
   sftp -i /path/to/pvmss_snippets_ed25519 pvmss-snippets@YOUR_PROXMOX_HOST
   ```

4. **Check snippets directory permissions**: The user must have write access to `/var/lib/vz/snippets`

5. **"packet too long" error**: This error occurs when SSH server sends unexpected data before SFTP negotiation. Ensure the `Match User` block with `ForceCommand internal-sftp` is properly configured in `/etc/ssh/sshd_config`.

6. **Verify SSH config**: Make sure the `Match User` block is at the **end** of the `sshd_config` file.

### Cloud-init not applied

1. **Verify template content**: Templates must start with `#cloud-config`
2. **Check VM cloud-init drive**: The VM should have `ide2` configured as cloudinit
3. **Check Proxmox logs**: `/var/log/pve/tasks/` contains task logs
4. **VM console**: Check `/var/log/cloud-init.log` inside the VM

## Alternative approaches

If you cannot use the SSH approach:

1. **Manual snippets**: Create snippets manually on Proxmox nodes and reference them in PVMSS (add the path to the snippet in the cicustom parameter, e.g., `user=local:snippets/my-template.yaml`)
2. **Basic cloud-init**: Use only basic cloud-init parameters (ciuser, cipassword, ipconfig0)
3. **External configuration**: Use external configuration management tools like Ansible

## Example templates

### Basic user setup

```yaml
#cloud-config
users:
  - name: admin
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ssh-ed25519 AAAAC3... user@example.com
```

### Package installation

```yaml
#cloud-config
package_update: true
package_upgrade: true
packages:
  - vim
  - htop
  - curl
  - git
```

### Custom script execution

```yaml
#cloud-config
runcmd:
  - echo "Hello from cloud-init" > /tmp/hello.txt
  - systemctl enable --now docker
```

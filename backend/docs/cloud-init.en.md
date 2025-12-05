# Cloud-Init support in PVMSS

## Overview

PVMSS provides cloud-init support for Proxmox VMs, allowing users to apply advanced cloud-init configurations during VM creation. This feature is optional and requires additional setup due to limitations in the Proxmox API.

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

### Step 4: Set filesystem permissions for snippets directory

```bash
# Install ACL tools if not already installed
apt install acl

# For 'local' storage, snippets are typically in /var/lib/vz/snippets
setfacl -m u:pvmss-snippets:rwx /var/lib/vz/snippets
setfacl -R -m u:pvmss-snippets:rwX /var/lib/vz/snippets

# Verify permissions
sudo -u pvmss-snippets ls -la /var/lib/vz/snippets
sudo -u pvmss-snippets touch /var/lib/vz/snippets/test-pvmss && rm /var/lib/vz/snippets/test-pvmss && echo "OK"
```

### Step 5: Mount the private key into the PVMSS container

⚠️ sensitive information: The private key file contains credentials that must be kept secure.

Get the content of the private key file previously generated (`/etc/pvmss/keys/pvmss_snippets_ed25519`), set its permissions correctly, and mount it into the container:

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
docker run -v ./pvmss_snippets_ed25519:/app/pvmss_snippets_ed25519:ro jhmmt/pvmss:1.0
```

### Step 5a: Kubernetes deployment

Get the content of the private key file, store it into a local file, and create a Kubernetes secret:

```bash
kubectl -n pvmss create secret generic pvmss-snippets-key \
  --from-file=private_key=pvmss_snippets_ed25519
```

Then reference this secret in your deployment configuration by mounting it as a volume:

```yaml

```

### Step 6: Configure PVMSS settings

In your `settings.json`, enable SFTP and configure the connection:

```json
{
  "cloudinit_sftp": {
    "enabled": true,
    "host": "YOUR_PROXMOX_NODE_IP_OR_HOSTNAME",
    "port": 22,
    "username": "pvmss-snippets",
    "privateKeyPath": "/app/pvmss_snippets_ed25519",
    "snippetBaseDir": "/var/lib/vz/snippets"
  }
}
```

## How it works

1. **Template selection**: User selects a cloud-init template in the VM creation form
2. **Snippet upload**: PVMSS uploads the YAML content via SFTP to the Proxmox node
3. **VM configuration**: PVMSS sets the `cicustom` parameter to reference the uploaded snippet
4. **Boot process**: Proxmox creates a cloud-init drive with the custom configuration

## Security considerations

- **Dedicated account**: The `pvmss-snippets` account has no shell access and is restricted to SFTP
- **Minimal permissions**: The account only has write access to the snippets directory
- **SSH key authentication**: No passwords are stored or transmitted

## Troubleshooting

### Common issues

1. **Permission denied**: Check filesystem permissions on the snippets directory
2. **SSH authentication failed**: Verify the SSH key is properly configured and accessible
3. **Snippet not found**: Ensure the storage supports snippets content type
4. **Network issues**: Verify PVMSS can reach the Proxmox node on port 22

## Limitations

- **SSH connection required**: Requires SSH access to Proxmox nodes for snippet upload
- **Single node**: Currently supports snippet upload to a single Proxmox node

## Alternative approaches

If you cannot use the SSH approach:

1. **Manual snippets**: Create snippets manually on Proxmox nodes and reference them in PVMSS (add the path to the snippet in the cicustom parameter, e.g., `user=local:snippets/my-template.yaml`)
2. **Basic cloud-init**: Use only basic cloud-init parameters (ciuser, cipassword, ipconfig0)
3. **External configuration**: Use external configuration management tools like Ansible

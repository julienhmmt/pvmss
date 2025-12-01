# Cloud-Init Support in PVMSS

## Overview

PVMSS provides cloud-init support for Proxmox VMs, allowing users to apply advanced cloud-init configurations during VM creation. This feature is optional and requires additional setup due to limitations in the Proxmox API.

## Why Cloud-Init is optional and requires setup

### Proxmox API limitations

The Proxmox API has significant limitations regarding cloud-init snippet uploads:

- The standard `/nodes/{node}/storage/{storage}/upload` endpoint with `content=snippets` is not supported in many Proxmox versions
- Even when snippets storage is configured and enabled, the API may return HTTP 400 "bad request" errors
- This limitation affects all Proxmox users regardless of their storage configuration

### Workaround: SSH/SFTP upload

Due to these API limitations, PVMSS implements cloud-init snippet upload via SSH/SFTP, similar to how popular Terraform providers (Telmate and bpg/proxmox) handle this limitation:

- **Telmate provider**: Uses SSH/SCP to upload snippets directly to `/var/lib/vz/snippets/`
- **bpg provider**: Uses SFTP with PAM accounts, documented as "Snippets Cannot Be Uploaded by Non-PAM Accounts"

This approach requires:

1. A dedicated PAM account on Proxmox nodes
2. SSH key authentication
3. Proper filesystem permissions for the snippets directory

## Setup Requirements

### 1. Create PAM Account on Each Proxmox Node

```bash
# Create dedicated user (no shell login)
sudo useradd --create-home --shell /usr/sbin/nologin pvmss-snippets

# Create .ssh directory and set permissions
sudo mkdir -p /home/pvmss-snippets/.ssh
sudo chmod 700 /home/pvmss-snippets/.ssh
sudo chown pvmss-snippets:pvmss-snippets /home/pvmss-snippets/.ssh

# Add SSH public key (replace with your actual public key)
echo 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... your-public-key-here' | sudo tee /home/pvmss-snippets/.ssh/authorized_keys
sudo chmod 600 /home/pvmss-snippets/.ssh/authorized_keys
sudo chown pvmss-snippets:pvmss-snippets /home/pvmss-snippets/.ssh/authorized_keys
```

### 2. Set Filesystem Permissions

```bash
# For 'local' storage, snippets are typically in /var/lib/vz/snippets
sudo setfacl -m u:pvmss-snippets:rwx /var/lib/vz/snippets
sudo setfacl -R -m u:pvmss-snippets:rwX /var/lib/vz/snippets

# Verify permissions
sudo -u pvmss-snippets ls -la /var/lib/vz/snippets
```

### 3. Test SFTP Access

```bash
# Test from the PVMSS server
sftp -oIdentityFile=/path/to/private-key pvmss-snippets@<node-ip>
cd /var/lib/vz/snippets
put test.yml
```

## PVMSS Configuration

### SSH Key Storage

Store the private SSH key securely on the PVMSS server:

```bash
# Create directory for SSH keys
sudo mkdir -p /etc/pvmss/keys
sudo chmod 700 /etc/pvmss/keys

# Place private key
sudo cp /path/to/private-key /etc/pvmss/keys/pvmss_snippets_ed25519
sudo chmod 600 /etc/pvmss/keys/pvmss_snippets_ed25519
sudo chown pvmss:pvmss /etc/pvmss/keys/pvmss_snippets_ed25519
```

### PVMSS Settings

Add the following configuration to your PVMSS settings:

```json
{
  "cloudInitSFTP": {
    "enabled": true,
    "host": "pve-node-1",
    "port": 22,
    "username": "pvmss-snippets",
    "privateKeyPath": "/etc/pvmss/keys/pvmss_snippets_ed25519",
    "snippetBaseDir": "/var/lib/vz/snippets"
  }
}
```

## How It Works

1. **Template Selection**: User selects a cloud-init template in the VM creation form
2. **Snippet Upload**: PVMSS uploads the YAML content via SFTP to the Proxmox node
3. **VM Configuration**: PVMSS sets the `cicustom` parameter to reference the uploaded snippet
4. **Boot Process**: Proxmox creates a cloud-init drive with the custom configuration

## Security Considerations

- **Dedicated Account**: The `pvmss-snippets` account has no shell access and is restricted to SFTP
- **Minimal Permissions**: The account only has write access to the snippets directory
- **SSH Key Authentication**: No passwords are stored or transmitted
- **Chroot Consideration**: For additional security, you can configure SSH chroot for the account

## Troubleshooting

### Common Issues

1. **Permission Denied**: Check filesystem permissions on the snippets directory
2. **SSH Authentication Failed**: Verify the SSH key is properly configured and accessible
3. **Snippet Not Found**: Ensure the storage supports snippets content type
4. **Network Issues**: Verify PVMSS can reach the Proxmox node on port 22

### Debug Commands

```bash
# Check user permissions
sudo -u pvmss-snippets touch /var/lib/vz/snippets/test

# Verify SSH configuration
sudo sshd -T | grep -i match

# Test SFTP connection manually
sftp -oIdentityFile=/etc/pvmss/keys/pvmss_snippets_ed25519 -oPort=22 pvmss-snippets@<node-ip>
```

## Limitations

- **Single Node**: Currently supports snippet upload to a single Proxmox node
- **Storage Type**: Works with local storage; shared storage requires additional configuration
- **Proxmox Version**: May not work on very old Proxmox versions without snippets support

## Alternative Approaches

If you cannot use the SSH/SFTP approach:

1. **Manual Snippets**: Create snippets manually on Proxmox nodes and reference them in PVMSS
2. **Basic Cloud-Init**: Use only basic cloud-init parameters (ciuser, cipassword, ipconfig0)
3. **External Configuration**: Use external configuration management tools like Ansible

## Future Improvements

Potential enhancements for cloud-init support:

- Multi-node support with automatic node selection
- Shared storage support for cluster-wide snippets
- Integration with secret management systems
- Automatic account provisioning scripts

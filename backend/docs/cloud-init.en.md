# Cloud-Init support in PVMSS

## Overview

PVMSS provides cloud-init support for Proxmox VMs, allowing users to apply advanced cloud-init configurations during VM creation. This feature is optional and requires additional setup due to limitations in the Proxmox API.

## Why Cloud-Init is optional and requires setup

### Proxmox API limitations

The Proxmox API has significant limitations regarding cloud-init snippet uploads:

- The standard `/nodes/{node}/storage/{storage}/upload` endpoint with `content=snippets` is not supported in many Proxmox versions
- Even when snippets storage is configured and enabled, the API may return HTTP 400 "bad request" errors
- This limitation affects all Proxmox users regardless of their storage configuration

### Workaround: SSH upload

Due to these API limitations, PVMSS implements cloud-init snippet upload via SSH, similar to how popular Terraform providers (Telmate and bpg/proxmox) handle this limitation:

- **Telmate provider**: Uses SSH/SCP to upload snippets directly to `/var/lib/vz/snippets/`
- **bpg provider**: Uses SFTP with PAM accounts, documented as "Snippets Cannot Be Uploaded by Non-PAM Accounts"

You can read more about this approach in the Terraform bpg provider documentation: <https://registry.terraform.io/providers/bpg/proxmox/latest/docs#ssh-connection>

This approach requires:

1. A dedicated PAM account on Proxmox nodes
2. SSH key authentication
3. Proper filesystem permissions for the snippets directory

## Setup requirements

### Create a PAM Account on the proxmox node you connect to with PVMSS

As the `root@pam` user, do the following:

```bash
# Create dedicated user (no shell login)
useradd --create-home --shell /usr/sbin/nologin pvmss-snippets

# Create .ssh directory and set permissions
mkdir -p /home/pvmss-snippets/.ssh
chmod 700 /home/pvmss-snippets/.ssh
chown pvmss-snippets:pvmss-snippets /home/pvmss-snippets/.ssh

# Add SSH public key (replace with your actual public key)
echo 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... your-public-key-here' | tee /home/pvmss-snippets/.ssh/authorized_keys
chmod 600 /home/pvmss-snippets/.ssh/authorized_keys
chown pvmss-snippets:pvmss-snippets /home/pvmss-snippets/.ssh/authorized_keys
```

### Set filesystem permissions

```bash
# For 'local' storage, snippets are typically in /var/lib/vz/snippets
apt install acl
setfacl -m u:pvmss-snippets:rwx /var/lib/vz/snippets
setfacl -R -m u:pvmss-snippets:rwX /var/lib/vz/snippets

# Verify permissions
sudo -u pvmss-snippets ls -la /var/lib/vz/snippets
sudo -u pvmss-snippets touch /var/lib/vz/snippets/test-from-pvmss
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

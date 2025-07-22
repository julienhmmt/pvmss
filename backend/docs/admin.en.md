# PVMSS Administrator Guide

## Overview

This guide covers all administrative features and workflows available in PVMSS (Proxmox Virtual Machine Self-Service), including system configuration, user management, and maintenance.

## Administrative Features

### System Configuration
- **ISO Management**: Control which ISO images are available to users for VM creation
- **Storage Management**: View and manage available storage resources
- **Network Bridges (VMBR)**: Configure available network bridges for VM networking
- **Resource Limits**: Set limits on CPU, memory, and storage resources
- **Security Settings**: Manage authentication and access control

### User Management
- **Access Control**: Configure user permissions and authentication
- **Resource Quotas**: Set per-user resource limits
- **Audit Logging**: Monitor user activities and system changes

### Monitoring and Maintenance
- **System Health**: Monitor PVMSS application status and performance
- **Log Management**: Review application logs for troubleshooting
- **Backup Configuration**: Ensure critical settings are backed up
- **Updates**: Keep PVMSS updated with latest security patches

### Getting Started
1. Access the admin panel at `/admin` (requires authentication)
2. Review and configure ISO images for user VM creation
3. Configure network bridges and storage options
4. Set appropriate resource limits based on your infrastructure
5. Monitor system logs regularly for any issues

### Security Best Practices
- Regularly update admin passwords using the provided hash generator
- Review user access logs periodically
- Keep the PVMSS application updated
- Monitor Proxmox API access and usage
- Use HTTPS in production environments

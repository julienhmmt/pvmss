# PVMSS user guide

PVMSS (Proxmox Virtual Machine Self-Service) is an intuitive web application that simplifies the creation, management, and console access of virtual machines.

The application is available in both French and English.

To access main features such as virtual machine creation and console access, you must log in using the credentials provided by your organization's administrator.

## Quick start guide

1. **Application login**: Log in to PVMSS using your credentials to access virtual machine creation and management features.
2. **Virtual machine search**: Use the search function to locate a specific virtual machine and view its details.
3. **Creating a virtual machine**: Click the "Create virtual machine" button to open the configuration form, then fill in the required parameters.
4. **Console access**: Once the virtual machine is created, click the "Console" button to connect to its interface.

## Main features

### Creating a virtual machine

To create a virtual machine, access the configuration form via the "Create VM" button after logging in to PVMSS. The following parameters must be set:

- **Name and Description**: Enter a unique name and description to identify your virtual machine.
- **Operating System**: Select an ISO image from a predefined list provided by administrators to install the operating system.
- **Resources**: Configure the necessary resources, including number of CPU cores, RAM, storage, and network bridge.
- **Tags**: Add predefined tags to organize and facilitate searching for your virtual machines.

You can only create one machine at a time, and you cannot modify the resources of an existing virtual machine.

### Searching for a virtual machine

Use the search function to locate a virtual machine by its *name* or *VMID* (unique identifier assigned to each virtual machine). A list of results will be displayed based on the search criteria. Click the "*Details*" button to view the virtual machine's information and manage its status.

### Managing a virtual machine

An intuitive panel allows you to manage your virtual machines, access their details, and monitor their usage.

- **Control**: Start, stop, or restart the virtual machine.
- **Monitoring**: View real-time status, uptime, and resource usage (CPU, memory, storage).
- **Details and Modifications**: Access configuration information and modify the description or tags if needed.

## Best practices

- Shut down your virtual machines properly to prevent data loss.
- Use clear names that comply with your organization's standards for your virtual machines.
- Contact your administrator for any additional resource requests.
- Never share your login credentials to ensure the security of your account and virtual machines.

## Support

The PVMSS application is managed by your organization's IT team. Contact your administrator for assistance in the following cases:

- Lost password (self-service password reset is not available).
- Difficulties creating a virtual machine.
- Issues accessing the console.

## Known limitations

The PVMSS application does not allow you to:

- Modify virtual machine resources (CPU, memory, storage, network bridge) after creation.
- Change your login username and password.
- Create LXC containers.

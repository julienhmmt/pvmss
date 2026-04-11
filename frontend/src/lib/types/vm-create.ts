/** Node option for VM creation. */
export interface VMCreateNodeOption {
  name: string;
  disabled: boolean;
  reason?: string;
}

/** Storage option for VM creation. */
export interface VMCreateStorageOption {
  name: string;
  node: string;
}

/** Bridge option for VM creation. */
export interface VMCreateBridgeOption {
  name: string;
  node: string;
  description?: string;
}

/** ISO option for VM creation. */
export interface VMCreateISOOption {
  volid: string;
  name: string;
}

/** Cloud-init template option for VM creation. */
export interface VMCreateCITemplate {
  id: string;
  name: string;
  description: string;
}

/** Resource range (min/max). */
export interface VMCreateRange {
  min: number;
  max: number;
}

/** Resource limits for VM creation. */
export interface VMCreateLimits {
  sockets: VMCreateRange;
  cores: VMCreateRange;
  ram: VMCreateRange;
  disk: VMCreateRange;
}

/** Response from GET /api/v1/vm-create/settings. */
export interface VMCreateSettings {
  nodes: VMCreateNodeOption[];
  storages: VMCreateStorageOption[];
  bridges: VMCreateBridgeOption[];
  isos: VMCreateISOOption[];
  tags: string[];
  cloudinit_templates: VMCreateCITemplate[];
  limits: VMCreateLimits;
  max_network_cards: number;
  max_disk_per_vm: number;
  max_vm_per_user: number;
  remaining_vms: number;
  proxmox_connected: boolean;
  allow_custom_yaml: boolean;
}

/** Disk in the VM creation request. */
export interface VMCreateDisk {
  size_gb: number;
}

/** Network card in the VM creation request. */
export interface VMCreateNetwork {
  bridge: string;
  model: string;
  mac: string;
  vlan: number;
  rate_limit: string;
  mtu: number;
  enabled: boolean;
}

/** Cloud-init configuration in the VM creation request. */
export interface VMCreateCloudInit {
  user: string;
  password: string;
  ssh_keys: string;
  ip_config: string;
  ip: string;
  gateway: string;
  dns: string;
  template_id: string;
}

/** Request body for POST /api/v1/vms. */
export interface VMCreateRequest {
  name: string;
  node: string;
  storage: string;
  description: string;
  iso: string;
  tags: string[];
  sockets: number;
  cores: number;
  memory_mb: number;
  disks: VMCreateDisk[];
  networks: VMCreateNetwork[];
  enable_efi: boolean;
  enable_tpm: boolean;
  disk_bus: string;
  start_vm: boolean;
  cloud_init?: VMCreateCloudInit;
}

/** Response from POST /api/v1/vms. */
export interface VMCreateResponse {
  vmid: number;
  name: string;
  node: string;
  cloud_init_warning?: string;
}

/** Network card model options. */
export const NETWORK_MODELS = [
  { value: "virtio", label: "VirtIO (recommended)" },
  { value: "e1000", label: "Intel E1000" },
  { value: "e1000e", label: "Intel E1000E" },
  { value: "rtl8139", label: "Realtek RTL8139" },
  { value: "vmxnet3", label: "VMware VMXNet3" },
] as const;

/** Disk bus options. */
export const DISK_BUSES = [
  { value: "virtio", label: "VirtIO Block (recommended)" },
  { value: "scsi", label: "SCSI" },
  { value: "sata", label: "SATA" },
  { value: "ide", label: "IDE" },
] as const;

/** IP configuration modes. */
export const IP_CONFIG_MODES = [
  { value: "dhcp", label: "DHCP" },
  { value: "static", label: "Static" },
] as const;

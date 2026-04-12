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

/** VM profile for simplified creation mode (admin-configured, from backend). */
export interface VMProfileConfig {
  id: string;
  name: string;
  description: string;
  sockets: number;
  cores: number;
  ram_gb: number;
  disk_gb: number;
  disk_bus: string;
  /** Empty string = auto-select. */
  node?: string;
  /** Empty string = auto-select. */
  storage?: string;
  icon: string;
  color: string;
  enabled: boolean;
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
  vm_profiles: VMProfileConfig[];
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

/** Color key → Tailwind CSS class mapping for VM profile cards. */
export const PROFILE_COLOR_CLASSES: Record<string, { bg: string; icon: string }> = {
  blue:    { bg: "bg-blue-100 dark:bg-blue-900/30",    icon: "text-blue-600 dark:text-blue-400" },
  violet:  { bg: "bg-violet-100 dark:bg-violet-900/30", icon: "text-violet-600 dark:text-violet-400" },
  emerald: { bg: "bg-emerald-100 dark:bg-emerald-900/30", icon: "text-emerald-600 dark:text-emerald-400" },
  teal:    { bg: "bg-teal-100 dark:bg-teal-900/30",    icon: "text-teal-600 dark:text-teal-400" },
  amber:   { bg: "bg-amber-100 dark:bg-amber-900/30",  icon: "text-amber-600 dark:text-amber-400" },
  rose:    { bg: "bg-rose-100 dark:bg-rose-900/30",    icon: "text-rose-600 dark:text-rose-400" },
  indigo:  { bg: "bg-indigo-100 dark:bg-indigo-900/30", icon: "text-indigo-600 dark:text-indigo-400" },
  sky:     { bg: "bg-sky-100 dark:bg-sky-900/30",      icon: "text-sky-600 dark:text-sky-400" },
  orange:  { bg: "bg-orange-100 dark:bg-orange-900/30", icon: "text-orange-600 dark:text-orange-400" },
  gray:    { bg: "bg-gray-100 dark:bg-gray-900/30",    icon: "text-gray-600 dark:text-gray-400" },
};

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
  ramGb: number;
  diskGb: number;
  diskBus: string;
  /** Empty string = auto-select. */
  node?: string;
  /** Empty string = auto-select. */
  storage?: string;
  icon: string;
  color: string;
  enabled: boolean;
  /** Enable EFI/UEFI boot (secure boot). */
  enableEfi?: boolean;
}

/** Response from GET /api/v1/vm-create/settings. */
export interface VMCreateSettings {
  nodes: VMCreateNodeOption[];
  storages: VMCreateStorageOption[];
  bridges: VMCreateBridgeOption[];
  isos: VMCreateISOOption[];
  tags: string[];
  cloudinitTemplates: VMCreateCITemplate[];
  /** True when at least one cloud-init template is enabled by an admin. */
  cloudInitAvailable: boolean;
  limits: VMCreateLimits;
  maxNetworkCards: number;
  maxDiskPerVm: number;
  maxVmPerUser: number;
  remainingVms: number;
  proxmoxConnected: boolean;
  allowCustomYaml: boolean;
  vmProfiles: VMProfileConfig[];
}

/** Disk in the VM creation request. */
export interface VMCreateDisk {
  sizeGb: number;
}

/** Network card in the VM creation request. */
export interface VMCreateNetwork {
  bridge: string;
  model: string;
  mac: string;
  vlan: number;
  rateLimit: string;
  mtu: number;
  enabled: boolean;
}

/** Cloud-init configuration in the VM creation request. */
export interface VMCreateCloudInit {
  user: string;
  password: string;
  sshKeys: string;
  ipConfig: string;
  ip: string;
  gateway: string;
  dns: string;
  templateId: string;
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
  memoryMb: number;
  disks: VMCreateDisk[];
  networks: VMCreateNetwork[];
  enableEfi: boolean;
  enableTpm: boolean;
  diskBus: string;
  startVm: boolean;
  cloudInit?: VMCreateCloudInit;
}

/** Response from POST /api/v1/vms. */
export interface VMCreateResponse {
  vmid: number;
  name: string;
  node: string;
  upid?: string;
  cloudInitWarning?: string;
}

/** Network card model options (labelKey points to i18n path). */
export const NETWORK_MODELS = [
  { value: "virtio", labelKey: "vmCreate.hardware.networkModelOptions.virtio" },
  { value: "e1000", labelKey: "vmCreate.hardware.networkModelOptions.e1000" },
  { value: "e1000e", labelKey: "vmCreate.hardware.networkModelOptions.e1000e" },
  { value: "rtl8139", labelKey: "vmCreate.hardware.networkModelOptions.rtl8139" },
  { value: "vmxnet3", labelKey: "vmCreate.hardware.networkModelOptions.vmxnet3" },
] as const;

/** Disk bus options (labelKey points to i18n path). */
export const DISK_BUSES = [
  { value: "virtio", labelKey: "vmCreate.hardware.diskBusOptions.virtio" },
  { value: "scsi", labelKey: "vmCreate.hardware.diskBusOptions.scsi" },
  { value: "sata", labelKey: "vmCreate.hardware.diskBusOptions.sata" },
  { value: "ide", labelKey: "vmCreate.hardware.diskBusOptions.ide" },
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

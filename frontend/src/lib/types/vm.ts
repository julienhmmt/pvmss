/**
 * VM status type.
 */
export type VMStatus = "running" | "stopped" | "paused";

/**
 * VM lifecycle actions accepted by the user VM API.
 */
export type VMAction = "start" | "stop" | "shutdown" | "reboot" | "suspend" | "resume";

/**
 * Disk configuration.
 */
export interface Disk {
  /** Disk index (e.g., "0", "1") */
  index: string;
  /** Disk bus type (ide, sata, virtio, scsi) */
  bus: string;
  /** Storage location */
  storage: string;
  /** Disk size in GB */
  sizeGb: number;
  /** Raw Proxmox config string */
  raw: string;
  /** Whether this disk is configured as boot disk */
  isBoot?: boolean;
}

/**
 * API disk configuration alias.
 */
export type DiskInfo = Disk;

/**
 * Network card configuration.
 */
export interface NetworkCard {
  /** Network card index (e.g., "0", "1") */
  index: string;
  /** MAC address */
  mac: string;
  /** Network card model (virtio, e1000, etc.) */
  model: string;
  /** Network bridge */
  bridge: string;
  /** Optional: VLAN tag */
  tag?: number;
  /** Optional: Firewall enabled */
  firewall?: boolean;
  /** Optional: Rate limit */
  rate?: string;
  /** Optional: IP addresses */
  ips?: string[];
  /** Optional: Link down flag */
  linkDown?: boolean;
  /** Optional: Display label for model */
  modelLabel?: string;
  /** Optional: i18n suffix for model */
  modelTranslationSuffix?: string;
  /** Optional: MTU value */
  mtu?: string;
}

/**
 * API network interface alias.
 */
export type NetworkInterface = NetworkCard;

/**
 * Cloud-Init configuration.
 */
export interface CloudInitConfig {
  /** Optional: Default user */
  user?: string;
  /** Optional: SSH public keys */
  sshKeys?: string;
  /** Optional: IP configuration */
  ipConfig?: string;
  /** Optional: DNS nameserver */
  nameserver?: string;
}

/**
 * API cloud-init configuration alias.
 */
export type CloudInitInfo = CloudInitConfig;

/**
 * Complete VM information.
 */
export interface VM {
  /** VM ID */
  vmid: number;
  /** VM name */
  name: string;
  /** Proxmox node */
  node: string;
  /** VM status */
  status: VMStatus;
  /** CPU usage percentage */
  cpu: number;
  /** Number of CPU cores */
  cpus: number;
  /** Number of sockets */
  sockets: number;
  /** Number of cores per socket */
  cores: number;
  /** Used memory in MB */
  memMb: number;
  /** Maximum memory in MB */
  maxMemMb: number;
  /** Disk usage in MB */
  diskMb: number;
  /** VM uptime in seconds */
  uptime: number;
  /** Semicolon-separated tags */
  tags: string;
  /** Optional: VM description */
  description?: string;
  /** Array of disks */
  disks: Disk[];
  /** Array of network cards */
  networks: NetworkCard[];
  /** Optional: Cloud-Init configuration */
  cloudInit?: CloudInitConfig;
  /** Whether VM has CD-ROM */
  hasCdrom: boolean;
  /** Optional: Current mounted ISO */
  currentIso?: string;
  /** Whether EFI/UEFI is enabled */
  efiEnabled: boolean;
  /** Whether TPM is enabled */
  tpmEnabled: boolean;
}

/**
 * Detailed VM configuration returned by the VM details API.
 */
export type VMConfig = VM;

/**
 * VM snapshot metadata.
 */
export interface Snapshot {
  /** Snapshot name */
  name: string;
  /** Snapshot description */
  description: string;
  /** Snapshot creation time as Unix timestamp */
  snaptime: number;
  /** Whether snapshot includes RAM state */
  vmstate: number;
  /** Parent snapshot name */
  parent: string;
  /** Whether this entry is the current VM state */
  current: boolean;
}

/**
 * Snapshot list response with configured limit.
 */
export interface SnapshotList {
  /** Snapshot entries */
  snapshots: Snapshot[];
  /** Maximum allowed non-current snapshots */
  maxAllowed: number;
}

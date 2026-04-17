/**
 * VM status type.
 */
export type VMStatus = "running" | "stopped" | "paused";

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
}

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
}

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

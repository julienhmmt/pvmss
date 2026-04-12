/**
 * Resource range with minimum and maximum values.
 */
export interface ResourceRange {
  /** Minimum value */
  min: number;
  /** Maximum value */
  max: number;
}

/**
 * VM resource limits configuration.
 */
export interface VMResourceLimits {
  /** Socket count range */
  sockets: ResourceRange;
  /** Core count range */
  cores: ResourceRange;
  /** RAM range in MB */
  ram: ResourceRange;
  /** Disk size range in GB */
  disk: ResourceRange;
}

/**
 * Application limits configuration.
 */
export interface LimitsConfig {
  /** Default VM resource limits */
  vm: VMResourceLimits;
  /** Per-node VM resource limits */
  nodes: Record<string, VMResourceLimits>;
  /** Maximum snapshots per VM */
  maxSnapshots: number;
  /** Maximum network cards per VM */
  maxNetworkCards: number;
  /** Maximum disks per VM */
  maxDiskPerVm: number;
  /** Maximum VMs per user */
  maxVmPerUser: number;
}

/**
 * Cloud-Init template configuration.
 */
export interface CloudInitTemplate {
  /** Template ID */
  id: string;
  /** Template name */
  name: string;
  /** Template description */
  description: string;
  /** Storage location */
  storage: string;
  /** Filename */
  filename: string;
  /** YAML content */
  yaml_content: string;
  /** Whether the template is enabled */
  enabled: boolean;
}

/**
 * VM profile configuration for quick VM creation.
 */
export interface VMProfileConfig {
  /** Profile ID */
  id: string;
  /** Profile name */
  name: string;
  /** Profile description */
  description: string;
  /** Number of sockets */
  sockets: number;
  /** Number of cores */
  cores: number;
  /** RAM in GB */
  ram_gb: number;
  /** Disk size in GB */
  disk_gb: number;
  /** Disk bus type */
  disk_bus: string;
  /** Optional: Preferred Proxmox node */
  node?: string;
  /** Optional: Preferred storage */
  storage?: string;
  /** Icon identifier */
  icon: string;
  /** Color code */
  color: string;
  /** Whether the profile is enabled */
  enabled: boolean;
}

/**
 * Application settings configuration.
 */
export interface AppSettings {
  /** Enabled Proxmox nodes */
  enabled_nodes: string[];
  /** Enabled storage locations */
  enabled_storages: string[];
  /** Available ISO images */
  isos: string[];
  /** Resource limits configuration */
  limits: LimitsConfig;
  /** Available tags */
  tags: string[];
  /** Available network bridges */
  vmbrs: string[];
  /** Cloud-Init templates */
  cloudinit_templates: CloudInitTemplate[];
  /** VM profiles */
  vm_profiles: VMProfileConfig[];
  /** Whether custom YAML is allowed */
  allow_custom_yaml: boolean;
}

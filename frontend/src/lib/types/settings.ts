import type { VMProfileConfig } from "./vm-create";

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
  yamlContent: string;
  /** Whether the template is enabled */
  enabled: boolean;
}

/**
 * Application settings configuration.
 */
export interface AppSettings {
  /** Enabled Proxmox nodes */
  enabledNodes: string[];
  /** Enabled storage locations */
  enabledStorages: string[];
  /** Available ISO images */
  isos: string[];
  /** Resource limits configuration */
  limits: LimitsConfig;
  /** Maximum network cards per VM */
  maxNetworkCards: number;
  /** Maximum disks per VM */
  maxDiskPerVm: number;
  /** Maximum VMs per user */
  maxVmPerUser: number;
  /** Available tags */
  tags: string[];
  /** Available network bridges */
  vmbrs: string[];
  /** Cloud-Init templates */
  cloudinitTemplates: CloudInitTemplate[];
  /** VM profiles */
  vmProfiles: VMProfileConfig[];
  /** Whether custom YAML is allowed */
  allowCustomYaml: boolean;
}

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
  yaml_content: string;
  /** Whether the template is enabled */
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
  /** Maximum network cards per VM */
  max_network_cards: number;
  /** Maximum disks per VM */
  max_disk_per_vm: number;
  /** Maximum VMs per user */
  max_vm_per_user: number;
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

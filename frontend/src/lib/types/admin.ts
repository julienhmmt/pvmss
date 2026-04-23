export interface Node {
  name: string;
  status: string;
  cpu: number;
  maxCpu: number;
  cpuSockets: number;
  memory: number;
  maxMemory: number;
  disk: number;
  maxDisk: number;
  uptime: number;
  userEnabled: boolean;
}

export interface Storage {
  storage: string;
  type: string;
  content: string;
  total: number;
  used: number;
  free: number;
  node: string;
  enabled: boolean;
}

export interface VM {
  vmid: number;
  name: string;
  node: string;
  status: string;
  cpu: number;
  cpus: number;
  mem: number;
  maxMem: number;
  maxDisk: number;
  uptime: number;
  tags: string;
}

export interface Pool {
  poolId: string;
  comment: string;
  members: string[];
  vmCount: number;
}

export interface Tag {
  name: string;
  vmCount: number;
}

export interface ResourceRange {
  min: number;
  max: number;
}

export interface Limits {
  vm: {
    sockets: ResourceRange;
    cores: ResourceRange;
    ram: ResourceRange;
    disk: ResourceRange;
  };
  nodes: Record<
    string,
    {
      sockets: ResourceRange;
      cores: ResourceRange;
      ram: ResourceRange;
      disk: ResourceRange;
    }
  >;
  maxSnapshots: number;
  maxNetworkCards: number;
  maxDiskPerVm: number;
  maxVmPerUser: number;
}

export interface VMBR {
  iface: string;
  type: string;
  active: boolean;
  bridgePorts: string;
  node: string;
  enabled: boolean;
}

export interface CloudInitTemplate {
  id: string;
  name: string;
  description: string;
  storage: string;
  filename: string;
  yamlContent: string;
  enabled: boolean;
}

export interface ISO {
  volid: string;
  name: string;
  size: number;
  storage: string;
  node: string;
  enabled: boolean;
}

export interface ClusterInfo {
  isCluster: boolean;
  clusterName: string;
  nodeCount: number;
}

export interface SFTPStatus {
  enabled: boolean;
  host?: string;
  username?: string;
  keyExists: boolean;
  isConfigured: boolean;
  statusText: string;
  statusType: "success" | "warning" | "danger";
}

export interface AppInfo {
  version: string;
  environment: string;
  goVersion: string;
  platform: string;
  proxmoxConnected: boolean;
  proxmoxUrl: string;
  offlineMode: boolean;
  totalNodes: number;
  totalVms: number;
  clusterInfo?: ClusterInfo;
  envVars?: Record<string, string>;
}

export type VMAction = "start" | "stop" | "shutdown" | "reboot" | "reset";

export interface AuditEntry {
  id: number;
  tableName: string;
  recordId: string;
  action: string;
  oldValue: string;
  newValue: string;
  changedBy: string;
  changedAt: string;
}

export interface AuditLogResponse {
  entries: AuditEntry[];
  limit: number;
  offset: number;
}

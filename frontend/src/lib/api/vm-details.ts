import { api } from "./client";

export interface DiskInfo {
  index: string;
  bus: string;
  storage: string;
  size_gb: number;
  raw: string;
}

export interface CloudInitInfo {
  user?: string;
  ssh_keys?: string;
  ip_config?: string;
  nameserver?: string;
}

/**
 * Network interface configuration from API response.
 * Field names match Proxmox config format (e.g., "tag" for VLAN).
 */
export interface NetworkInterface {
  index: string; // e.g., "net0", "net1"
  mac: string;
  model: string;
  bridge: string;
  tag?: number; // VLAN tag from Proxmox config (0 = none)
  firewall?: boolean;
  rate?: string;
  ips?: string[];
  link_down?: boolean;
  model_label?: string;
  model_translation_suffix?: string;
  mtu?: string;
}

export interface VMConfig {
  vmid: number;
  name: string;
  node: string;
  status: string;
  cpu: number;
  cpus: number;
  sockets: number;
  cores: number;
  mem_mb: number;
  max_mem_mb: number;
  disk_mb: number;
  uptime: number;
  tags: string;
  description: string;
  networks: NetworkInterface[];
  disks: DiskInfo[];
  has_cdrom: boolean;
  current_iso: string;
  efi_enabled: boolean;
  tpm_enabled: boolean;
  cloud_init?: CloudInitInfo;
}

export interface VMMetrics {
  status: string;
  cpu: number;
  mem_mb: number;
  max_mem_mb: number;
}

export interface Snapshot {
  name: string;
  description: string;
  snaptime: number;
  vmstate: number;
  parent: string;
  current: boolean;
}

export interface SnapshotList {
  snapshots: Snapshot[];
  max_allowed: number;
}

export interface ISOOption {
  volid: string;
  name: string;
  node: string;
  enabled: boolean;
}

export interface VMBROption {
  iface: string;
  node: string;
  enabled: boolean;
}

export interface VMSettings {
  available_isos: ISOOption[];
  available_vmbrs: VMBROption[];
  available_tags: string[];
  limits: {
    min_sockets: number;
    max_sockets: number;
    min_cores: number;
    max_cores: number;
    min_ram_gb: number;
    max_ram_gb: number;
  };
  max_snapshots: number;
}

export async function getVMConfig(vmid: number): Promise<VMConfig> {
  return api.get<VMConfig>(`/api/v1/vms/${vmid}/config`);
}

export async function getVMMetrics(vmid: number): Promise<VMMetrics> {
  return api.get<VMMetrics>(`/api/v1/vms/${vmid}/metrics`);
}

export async function getVMSnapshots(vmid: number): Promise<SnapshotList> {
  return api.get<SnapshotList>(`/api/v1/vms/${vmid}/snapshots`);
}

export async function createSnapshot(
  vmid: number,
  name: string,
  description = "",
): Promise<void> {
  await api.post(`/api/v1/vms/${vmid}/snapshots`, { name, description });
}

export async function deleteSnapshot(
  vmid: number,
  name: string,
): Promise<void> {
  await api.delete(`/api/v1/vms/${vmid}/snapshots/${name}`);
}

export async function rollbackSnapshot(
  vmid: number,
  name: string,
): Promise<void> {
  await api.post(`/api/v1/vms/${vmid}/snapshots/${name}/rollback`, {});
}

export async function updateVMConfig(
  vmid: number,
  data: { description?: string; tags?: string },
): Promise<void> {
  await api.patch(`/api/v1/vms/${vmid}/config`, data);
}

export async function getVMSettings(vmid: number): Promise<VMSettings> {
  return api.get<VMSettings>(`/api/v1/vms/${vmid}/settings`);
}

export function deleteVM(vmid: number): Promise<void> {
  return api.delete(`/api/v1/vms/${vmid}`);
}

/**
 * Network update request for API.
 * Field names match backend struct (e.g., "vlan" instead of "tag").
 */
export interface NetworkUpdateRequest {
  index: string; // "net0"…"net9"; empty = new card (backend auto-assigns)
  model: string; // virtio, e1000, e1000e, rtl8139, vmxnet3
  bridge: string;
  mac?: string;
  vlan: number; // VLAN tag for API request (0 = none)
  rate: string; // MB/s or ""
  firewall: boolean;
}

export interface VMHardwareUpdate {
  node: string;
  sockets: number;
  cores: number;
  memory_mb: number;
  tags?: string;
  networks?: NetworkUpdateRequest[];
  delete_networks?: string[];
}

export interface VMHardwareUpdateResponse {
  success: boolean;
  restarted: boolean;
  message: string;
}

export async function updateVMHardware(
  vmid: number,
  data: VMHardwareUpdate,
): Promise<VMHardwareUpdateResponse> {
  return api.put<VMHardwareUpdateResponse>(
    `/api/v1/vms/${vmid}/hardware`,
    data,
  );
}

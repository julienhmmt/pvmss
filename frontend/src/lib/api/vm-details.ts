import { api } from "./client";
import type { VMStatus } from "$lib/types/vm";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

export interface DiskInfo {
  index: string;
  bus: string;
  storage: string;
  sizeGb: number;
  raw: string;
  isBoot: boolean;
}

export interface CloudInitInfo {
  user?: string;
  sshKeys?: string;
  ipConfig?: string;
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
  linkDown?: boolean;
  modelLabel?: string;
  modelTranslationSuffix?: string;
  mtu?: string;
}

export interface VMConfig {
  vmid: number;
  name: string;
  node: string;
  status: VMStatus;
  cpu: number;
  cpus: number;
  sockets: number;
  cores: number;
  memMb: number;
  maxMemMb: number;
  diskMb: number;
  uptime: number;
  tags: string;
  description: string;
  networks: NetworkInterface[];
  disks: DiskInfo[];
  hasCdrom: boolean;
  currentIso: string;
  efiEnabled: boolean;
  tpmEnabled: boolean;
  cloudInit?: CloudInitInfo;
}

export interface VMMetrics {
  status: VMStatus;
  cpu: number;
  memMb: number;
  maxMemMb: number;
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
  maxAllowed: number;
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

export interface StorageOption {
  storage: string;
  node?: string;
  enabled: boolean;
}

export interface VMSettings {
  availableIsos: ISOOption[];
  availableVmbrs: VMBROption[];
  availableTags: string[];
  availableStorages: StorageOption[];
  limits: {
    minSockets: number;
    maxSockets: number;
    minCores: number;
    maxCores: number;
    minRamGb: number;
    maxRamGb: number;
    minDiskGb: number;
    maxDiskGb: number;
    maxDisksPerVm: number;
    maxSnapshots: number;
  };
}

export async function getVMConfig(vmid: number): Promise<VMConfig> {
  const res = await api.get<Record<string, unknown>>(`/api/v1/vms/${vmid}/config`);
  return transformKeysToCamelCase<VMConfig>(res);
}

export async function getVMMetrics(vmid: number): Promise<VMMetrics> {
  const res = await api.get<Record<string, unknown>>(`/api/v1/vms/${vmid}/metrics`);
  return transformKeysToCamelCase<VMMetrics>(res);
}

export async function getVMSnapshots(vmid: number): Promise<SnapshotList> {
  const res = await api.get<Record<string, unknown>>(`/api/v1/vms/${vmid}/snapshots`);
  return transformKeysToCamelCase<SnapshotList>(res);
}

export async function createSnapshot(
  vmid: number,
  name: string,
  description = "",
): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>({ name, description });
  await api.post(`/api/v1/vms/${vmid}/snapshots`, payload);
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
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(data);
  await api.patch(`/api/v1/vms/${vmid}/config`, payload);
}

export async function getVMSettings(vmid: number): Promise<VMSettings> {
  const res = await api.get<Record<string, unknown>>(`/api/v1/vms/${vmid}/settings`);
  return transformKeysToCamelCase<VMSettings>(res);
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
  memoryMb: number;
  tags?: string;
  networks?: NetworkUpdateRequest[];
  deleteNetworks?: string[];
}

export interface VMHardwareUpdateResponse {
  success: boolean;
  restarted: boolean;
  message: string;
}

export async function addDisk(
  vmid: number,
  data: { storage: string; sizeGb: number; bus: string },
): Promise<{ status: string; disk: string }> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(data);
  const res = await api.post<Record<string, unknown>>(
    `/api/v1/vms/${vmid}/disks`,
    payload,
  );
  return transformKeysToCamelCase<{ status: string; disk: string }>(res);
}

export async function resizeDisk(
  vmid: number,
  diskId: string,
  addGB: number,
): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>({ addGb: addGB });
  await api.put(`/api/v1/vms/${vmid}/disks/${encodeURIComponent(diskId)}/resize`, payload);
}

export async function deleteDisk(
  vmid: number,
  diskId: string,
): Promise<void> {
  await api.delete(`/api/v1/vms/${vmid}/disks/${encodeURIComponent(diskId)}`);
}

export async function updateVMHardware(
  vmid: number,
  data: VMHardwareUpdate,
): Promise<VMHardwareUpdateResponse> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(data);
  const res = await api.put<Record<string, unknown>>(
    `/api/v1/vms/${vmid}/hardware`,
    payload,
  );
  return transformKeysToCamelCase<VMHardwareUpdateResponse>(res);
}

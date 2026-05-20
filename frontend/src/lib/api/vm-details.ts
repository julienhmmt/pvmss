import { api } from "./client";
import type {
  CloudInitInfo,
  DiskInfo,
  NetworkInterface,
  SnapshotList,
  VMConfig,
  VMStatus,
} from "$lib/types/vm";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

export type {
  CloudInitInfo,
  DiskInfo,
  NetworkInterface,
  Snapshot,
  SnapshotList,
  VMConfig,
} from "$lib/types/vm";

export interface VMMetrics {
  status: VMStatus;
  cpu: number;
  memMb: number;
  maxMemMb: number;
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
  const res = await api.get<VMConfig>(`/api/v1/vms/${vmid}/config`);
  return transformKeysToCamelCase<VMConfig>(res);
}

export async function getVMMetrics(vmid: number): Promise<VMMetrics> {
  const res = await api.get<VMMetrics>(`/api/v1/vms/${vmid}/metrics`);
  return transformKeysToCamelCase<VMMetrics>(res);
}

export async function getVMSnapshots(vmid: number): Promise<SnapshotList> {
  const res = await api.get<SnapshotList>(`/api/v1/vms/${vmid}/snapshots`);
  return transformKeysToCamelCase<SnapshotList>(res);
}

export async function createSnapshot(
  vmid: number,
  name: string,
  description = "",
  vmstate = false,
): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>({ name, description, vmstate });
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
  data: { description?: string; tags?: string; name?: string },
): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(data);
  await api.patch(`/api/v1/vms/${vmid}/config`, payload);
}

export async function updateVMCDROM(
  vmid: number,
  iso: string,
): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>({ iso });
  await api.patch(`/api/v1/vms/${vmid}/cdrom`, payload);
}

export async function disconnectVMCDROM(vmid: number): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>({ disconnect: true });
  await api.patch(`/api/v1/vms/${vmid}/cdrom`, payload);
}

export async function getVMSettings(vmid: number): Promise<VMSettings> {
  const res = await api.get<VMSettings>(`/api/v1/vms/${vmid}/settings`);
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

export interface AddDiskResponse {
  status: string;
  disk: string;
}

export async function addDisk(
  vmid: number,
  data: { storage: string; sizeGb: number; bus: string },
): Promise<AddDiskResponse> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(data);
  const res = await api.post<AddDiskResponse>(
    `/api/v1/vms/${vmid}/disks`,
    payload,
  );
  return transformKeysToCamelCase<AddDiskResponse>(res);
}

export async function resizeDisk(
  vmid: number,
  diskId: string,
  addGB: number,
): Promise<VMHardwareUpdateResponse> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>({
    disk_id: diskId,
    add_gb: addGB,
  });
  const res = await api.patch<VMHardwareUpdateResponse>(
    `/api/v1/vms/${vmid}/disks/${diskId}/resize`,
    payload,
  );
  return transformKeysToCamelCase<VMHardwareUpdateResponse>(res);
}

export async function deleteDisk(vmid: number, diskId: string): Promise<void> {
  await api.delete(`/api/v1/vms/${vmid}/disks/${diskId}`);
}

export async function updateVMHardware(
  vmid: number,
  data: VMHardwareUpdate,
): Promise<VMHardwareUpdateResponse> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(data);
  const res = await api.put<VMHardwareUpdateResponse>(
    `/api/v1/vms/${vmid}/hardware`,
    payload,
  );
  return transformKeysToCamelCase<VMHardwareUpdateResponse>(res);
}

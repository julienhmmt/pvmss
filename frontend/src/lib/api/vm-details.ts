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

/**
 * Cloud-Init update payload for PUT /api/v1/vms/:id/cloudinit.
 * Fields use camelCase; the client transforms them to the backend's
 * snake_case JSON keys. An empty `password` keeps the current value
 * (Proxmox does not expose the existing password). Empty strings for
 * the other fields clear the corresponding key in Proxmox.
 */
export interface CloudInitUpdatePayload {
  user?: string;
  password?: string;
  sshKeys?: string;
  ipConfig?: string;
  nameserver?: string;
  searchdomain?: string;
}

export async function updateVMCloudInit(
  vmid: number,
  data: CloudInitUpdatePayload,
): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(data);
  await api.put(`/api/v1/vms/${vmid}/cloudinit`, payload);
}

/**
 * Custom cloud-config snippet (cicustom) attached to a VM.
 * Returned by GET /api/v1/vms/:id/cloudinit/snippet.
 */
export interface CloudInitSnippet {
  /** YAML content (empty when no snippet exists yet) */
  content: string;
  /** Snippets storage backing the snippet (may be empty until first save) */
  storage?: string;
  /** Snippet filename, e.g. pvmss-100.yml */
  filename: string;
  /** Current cicustom volid, if any */
  cicustom?: string;
  /** True when SFTP is configured and the content can be saved; false → read-only view (rendered dump) */
  editable: boolean;
}

/** Fetches the custom cloud-config YAML snippet attached to a VM. */
export async function getVMCloudInitSnippet(
  vmid: number,
): Promise<CloudInitSnippet> {
  const res = await api.get<CloudInitSnippet>(
    `/api/v1/vms/${vmid}/cloudinit/snippet`,
  );
  return transformKeysToCamelCase<CloudInitSnippet>(res);
}

/**
 * Validates and re-uploads the custom cloud-config YAML snippet for a VM via
 * SFTP, setting cicustom on the VM when it is not already pointing at the
 * snippet. Requires SFTP to be configured (the Proxmox HTTP API cannot reliably
 * write snippets).
 */
export async function updateVMCloudInitSnippet(
  vmid: number,
  content: string,
): Promise<CloudInitSnippet> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>({ content });
  const res = await api.put<CloudInitSnippet>(
    `/api/v1/vms/${vmid}/cloudinit/snippet`,
    payload,
  );
  return transformKeysToCamelCase<CloudInitSnippet>(res);
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
  const res = await api.put<VMHardwareUpdateResponse>(
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

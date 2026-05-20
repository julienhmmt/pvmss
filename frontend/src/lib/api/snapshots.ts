import { api } from "./client";
import type { Snapshot, SnapshotList } from "$lib/types/vm";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

export type { Snapshot, SnapshotList };

/**
 * Lists all snapshots for a VM.
 * @param vmid - The VM ID (must be a positive integer)
 * @returns Promise<SnapshotList> - List of snapshots with limit info
 * @throws Error if vmid is not a positive integer
 */
export async function listSnapshots(vmid: number): Promise<SnapshotList> {
  if (vmid <= 0) {
    throw new Error("Invalid VM ID: must be a positive integer");
  }
  const res = await api.get<SnapshotList>(`/api/v1/vms/${vmid}/snapshots`);
  return transformKeysToCamelCase<SnapshotList>(res);
}

/**
 * Creates a new snapshot for a VM.
 * @param vmid - The VM ID (must be a positive integer)
 * @param name - Snapshot name (must be non-empty)
 * @param description - Optional snapshot description (default: empty)
 * @param vmstate - Whether to include VM state (default: false)
 * @returns Promise<void>
 * @throws Error if vmid is not a positive integer or name is empty
 */
export async function createSnapshot(
  vmid: number,
  name: string,
  description = "",
  vmstate = false,
): Promise<void> {
  if (vmid <= 0) {
    throw new Error("Invalid VM ID: must be a positive integer");
  }
  if (!name || name.trim().length === 0) {
    throw new Error("Snapshot name cannot be empty");
  }
  const payload = transformKeysToSnakeCase<Record<string, unknown>>({ name, description, vmstate });
  await api.post(`/api/v1/vms/${vmid}/snapshots`, payload);
}

/**
 * Deletes a snapshot from a VM.
 * @param vmid - The VM ID (must be a positive integer)
 * @param name - Snapshot name (must be non-empty, will be URL-encoded)
 * @returns Promise<void>
 * @throws Error if vmid is not a positive integer or name is empty
 */
export async function deleteSnapshot(
  vmid: number,
  name: string,
): Promise<void> {
  if (vmid <= 0) {
    throw new Error("Invalid VM ID: must be a positive integer");
  }
  if (!name || name.trim().length === 0) {
    throw new Error("Snapshot name cannot be empty");
  }
  await api.delete(`/api/v1/vms/${vmid}/snapshots/${encodeURIComponent(name)}`);
}

/**
 * Rolls back a VM to a specific snapshot.
 * @param vmid - The VM ID (must be a positive integer)
 * @param name - Snapshot name (must be non-empty, will be URL-encoded)
 * @returns Promise<void>
 * @throws Error if vmid is not a positive integer or name is empty
 */
export async function rollbackSnapshot(
  vmid: number,
  name: string,
): Promise<void> {
  if (vmid <= 0) {
    throw new Error("Invalid VM ID: must be a positive integer");
  }
  if (!name || name.trim().length === 0) {
    throw new Error("Snapshot name cannot be empty");
  }
  await api.post(`/api/v1/vms/${vmid}/snapshots/${encodeURIComponent(name)}/rollback`, {});
}

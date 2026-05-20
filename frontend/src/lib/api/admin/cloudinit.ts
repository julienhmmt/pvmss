import { api } from "$lib/api/client";
import type { CloudInitTemplate, SFTPStatus } from "$lib/types/admin";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

interface CloudInitListResponse {
  templates: CloudInitTemplate[];
  sftpStatus?: SFTPStatus;
}

export async function getCloudInits(): Promise<CloudInitListResponse> {
  const response = await api.get<CloudInitListResponse>("/api/v1/admin/cloudinit");
  return transformKeysToCamelCase<CloudInitListResponse>(response);
}

export async function getCloudInitStorages(): Promise<string[]> {
  return api.get<string[]>("/api/v1/admin/cloudinit/storages");
}

export interface CreateCloudInitData {
  name: string;
  description: string;
  storage: string;
  yamlContent: string;
}

export async function createCloudInit(data: CreateCloudInitData): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(data);
  return api.post("/api/v1/admin/cloudinit", payload);
}

export async function updateCloudInit(
  id: string,
  data: CreateCloudInitData,
): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(data);
  return api.put(`/api/v1/admin/cloudinit/${encodeURIComponent(id)}`, payload);
}

export async function deleteCloudInit(id: string): Promise<void> {
  return api.delete(`/api/v1/admin/cloudinit/${encodeURIComponent(id)}`);
}

export async function toggleCloudInit(id: string): Promise<void> {
  return api.post(`/api/v1/admin/cloudinit/${encodeURIComponent(id)}/toggle`);
}

export async function toggleSFTP(): Promise<void> {
  return api.post("/api/v1/admin/cloudinit-sftp/toggle");
}

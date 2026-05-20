import { api } from "$lib/api/client";
import type { VMProfileConfig } from "$lib/types/vm-create";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

interface ProfileListResponse {
  profiles: VMProfileConfig[];
  usingDefaults: boolean;
}

export async function getProfiles(): Promise<ProfileListResponse> {
  const response = await api.get<ProfileListResponse>("/api/v1/admin/vm-profiles");
  return transformKeysToCamelCase<ProfileListResponse>(response);
}

export async function createProfile(data: Omit<VMProfileConfig, "id"> & { id?: string }): Promise<VMProfileConfig> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(data);
  const response = await api.post<VMProfileConfig>("/api/v1/admin/vm-profiles", payload);
  return transformKeysToCamelCase<VMProfileConfig>(response);
}

export async function updateProfile(id: string, data: Omit<VMProfileConfig, "id">): Promise<VMProfileConfig> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(data);
  const response = await api.put<VMProfileConfig>(`/api/v1/admin/vm-profiles/${encodeURIComponent(id)}`, payload);
  return transformKeysToCamelCase<VMProfileConfig>(response);
}

export async function deleteProfile(id: string): Promise<void> {
  return api.delete(`/api/v1/admin/vm-profiles/${encodeURIComponent(id)}`);
}

export async function toggleProfile(id: string): Promise<void> {
  return api.post(`/api/v1/admin/vm-profiles/${encodeURIComponent(id)}/toggle`);
}

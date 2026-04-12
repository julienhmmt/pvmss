import { api } from "$lib/api/client";
import type { VMProfileConfig } from "$lib/types/vm-create";

interface ProfileListResponse {
  profiles: VMProfileConfig[];
  using_defaults: boolean;
}

export function getProfiles(): Promise<ProfileListResponse> {
  return api.get("/api/v1/admin/vm-profiles");
}

export function createProfile(data: Omit<VMProfileConfig, "id"> & { id?: string }): Promise<VMProfileConfig> {
  return api.post("/api/v1/admin/vm-profiles", data);
}

export function updateProfile(id: string, data: Omit<VMProfileConfig, "id">): Promise<VMProfileConfig> {
  return api.put(`/api/v1/admin/vm-profiles/${encodeURIComponent(id)}`, data);
}

export function deleteProfile(id: string): Promise<void> {
  return api.delete(`/api/v1/admin/vm-profiles/${encodeURIComponent(id)}`);
}

export function toggleProfile(id: string): Promise<void> {
  return api.post(`/api/v1/admin/vm-profiles/${encodeURIComponent(id)}/toggle`);
}

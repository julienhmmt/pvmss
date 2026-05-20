import { api } from "$lib/api/client";
import type { Tag } from "$lib/types/admin";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

export async function getTags(): Promise<Tag[]> {
  const response = await api.get<Tag[]>("/api/v1/admin/tags");
  return transformKeysToCamelCase<Tag[]>(response);
}

export async function createTag(name: string): Promise<void> {
  return api.post("/api/v1/admin/tags", transformKeysToSnakeCase({ name }));
}

export async function deleteTag(name: string): Promise<void> {
  return api.delete(`/api/v1/admin/tags/${encodeURIComponent(name)}`);
}

/**
 * Update the Proxmox datacenter `tag-style` color-map for the given tag.
 * Pass an empty `color` to remove the entry and fall back to the auto palette.
 */
export async function setTagColor(
  name: string,
  color: string,
  textColor: string = "",
): Promise<void> {
  return api.put(
    `/api/v1/admin/tags/${encodeURIComponent(name)}/color`,
    transformKeysToSnakeCase({ color, textColor }),
  );
}

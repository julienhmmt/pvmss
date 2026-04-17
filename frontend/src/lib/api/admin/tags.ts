import { api } from "$lib/api/client";
import type { Tag } from "$lib/types/admin";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

export async function getTags(): Promise<Tag[]> {
  const response = await api.get<Record<string, unknown>[]>("/api/v1/admin/tags");
  return transformKeysToCamelCase<Tag[]>(response);
}

export async function createTag(name: string): Promise<void> {
  return api.post("/api/v1/admin/tags", transformKeysToSnakeCase({ name }));
}

export async function deleteTag(name: string): Promise<void> {
  return api.delete(`/api/v1/admin/tags/${encodeURIComponent(name)}`);
}

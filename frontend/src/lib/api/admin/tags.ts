import { api } from "$lib/api/client";
import type { Tag } from "$lib/types/admin";

export function getTags(): Promise<Tag[]> {
  return api.get("/api/v1/admin/tags");
}

export function createTag(name: string): Promise<void> {
  return api.post("/api/v1/admin/tags", { name });
}

export function deleteTag(name: string): Promise<void> {
  return api.delete(`/api/v1/admin/tags/${encodeURIComponent(name)}`);
}

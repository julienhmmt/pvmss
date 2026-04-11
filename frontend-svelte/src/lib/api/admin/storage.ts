import { api } from "$lib/api/client";
import type { Storage } from "$lib/types/admin";

export function getStorages(): Promise<Storage[]> {
  return api.get("/api/v1/admin/storage");
}

export function toggleStorage(storage: string, node: string): Promise<void> {
  return api.post("/api/v1/admin/storage/toggle", { storage, node });
}

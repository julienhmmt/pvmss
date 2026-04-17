import { api } from "$lib/api/client";
import type { Storage } from "$lib/types/admin";
import { transformKeysToCamelCase } from "$lib/utils/transform";

export async function getStorages(): Promise<Storage[]> {
  const response = await api.get<Record<string, unknown>[]>("/api/v1/admin/storage");
  return transformKeysToCamelCase<Storage[]>(response);
}

export async function toggleStorage(storage: string, node: string): Promise<void> {
  return api.post("/api/v1/admin/storage/toggle", { storage, node });
}

import { api } from "$lib/api/client";
import type { ISO } from "$lib/types/admin";
import { transformKeysToCamelCase } from "$lib/utils/transform";

export async function getISOs(): Promise<ISO[]> {
  const response = await api.get<Record<string, unknown>[]>("/api/v1/admin/iso");
  return transformKeysToCamelCase<ISO[]>(response);
}

export async function toggleISO(volid: string): Promise<void> {
  return api.post("/api/v1/admin/iso/toggle", { volid });
}

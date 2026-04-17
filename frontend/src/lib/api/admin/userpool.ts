import { api } from "$lib/api/client";
import type { Pool } from "$lib/types/admin";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

export async function getPools(): Promise<Pool[]> {
  const response = await api.get<Record<string, unknown>[]>("/api/v1/admin/userpool");
  return transformKeysToCamelCase<Pool[]>(response);
}

export interface CreatePoolData {
  poolName: string;
  password: string;
}

export async function createPool(data: CreatePoolData): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(data);
  return api.post("/api/v1/admin/userpool", payload);
}

export async function deletePool(name: string): Promise<void> {
  return api.delete(`/api/v1/admin/userpool/${encodeURIComponent(name)}`);
}

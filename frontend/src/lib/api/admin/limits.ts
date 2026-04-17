import { api } from "$lib/api/client";
import type { Limits } from "$lib/types/admin";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

export async function getLimits(): Promise<Limits> {
  const response = await api.get<Record<string, unknown>>("/api/v1/admin/limits");
  return transformKeysToCamelCase<Limits>(response);
}

export async function updateLimits(limits: Limits): Promise<void> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(limits);
  return api.put("/api/v1/admin/limits", payload);
}

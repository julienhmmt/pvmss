import { api } from "$lib/api/client";
import type { VMBR } from "$lib/types/admin";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

export async function getVMBRs(): Promise<VMBR[]> {
  const response = await api.get<Record<string, unknown>[]>("/api/v1/admin/vmbr");
  return transformKeysToCamelCase<VMBR[]>(response);
}

export async function toggleVMBR(vmbr: string, node: string): Promise<void> {
  return api.post(
    "/api/v1/admin/vmbr/toggle",
    transformKeysToSnakeCase({ vmbr, node }),
  );
}

import { api } from "./client";
import type {
  VMCreateSettings,
  VMCreateRequest,
  VMCreateResponse,
} from "$lib/types/vm-create";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

/** Fetches VM creation settings (nodes, storages, bridges, ISOs, limits, etc.). */
export async function getVMCreateSettings(): Promise<VMCreateSettings> {
  const response = await api.get<Record<string, unknown>>("/api/v1/vm-create/settings");
  return transformKeysToCamelCase<VMCreateSettings>(response);
}

/** Creates a new VM with the given configuration. */
export async function createVM(
  request: VMCreateRequest,
): Promise<VMCreateResponse> {
  const payload = transformKeysToSnakeCase<Record<string, unknown>>(request);
  const response = await api.post<Record<string, unknown>>("/api/v1/vms", payload);
  return transformKeysToCamelCase<VMCreateResponse>(response);
}

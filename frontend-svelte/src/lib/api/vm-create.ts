import { api } from "./client";
import type {
  VMCreateSettings,
  VMCreateRequest,
  VMCreateResponse,
} from "$lib/types/vm-create";

/** Fetches VM creation settings (nodes, storages, bridges, ISOs, limits, etc.). */
export async function getVMCreateSettings(): Promise<VMCreateSettings> {
  return api.get<VMCreateSettings>("/api/v1/vm-create/settings");
}

/** Creates a new VM with the given configuration. */
export async function createVM(
  request: VMCreateRequest,
): Promise<VMCreateResponse> {
  return api.post<VMCreateResponse>("/api/v1/vms", request);
}

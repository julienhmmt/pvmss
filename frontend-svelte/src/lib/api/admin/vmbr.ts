import { api } from "$lib/api/client";
import type { VMBR } from "$lib/types/admin";

export function getVMBRs(): Promise<VMBR[]> {
  return api.get("/api/v1/admin/vmbr");
}

export function toggleVMBR(vmbr: string, node: string): Promise<void> {
  return api.post("/api/v1/admin/vmbr/toggle", { vmbr, node });
}

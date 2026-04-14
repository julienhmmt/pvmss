import { api } from "$lib/api/client";
import type { VM, VMAction } from "$lib/types/admin";
import type { PaginationMetadata } from "$lib/api/vms";

export interface AdminVMPaginationParams {
  page?: number;
  limit?: number;
  search?: string;
  sort_by?: string;
  sort_order?: string;
  node?: string;
}

export interface AdminVMListPaginatedResponse {
  vms: VM[];
  pagination: PaginationMetadata;
}

export function getAllVMs(): Promise<VM[]> {
  return api.get("/api/v1/admin/vms");
}

export function getAllVMsPaginated(params: AdminVMPaginationParams = {}): Promise<AdminVMListPaginatedResponse> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.search) qs.set("search", params.search);
  if (params.sort_by) qs.set("sort_by", params.sort_by);
  if (params.sort_order) qs.set("sort_order", params.sort_order);
  if (params.node) qs.set("node", params.node);
  const query = qs.toString();
  return api.get<AdminVMListPaginatedResponse>(
    `/api/v1/admin/vms/paginated${query ? "?" + query : ""}`,
  );
}

export function vmAction(
  vmid: number,
  node: string,
  action: VMAction,
): Promise<void> {
  return api.post(`/api/v1/admin/vms/${vmid}/action`, { action, node });
}

export function deleteVM(vmid: number): Promise<void> {
  return api.delete(`/api/v1/admin/vms/${vmid}`);
}

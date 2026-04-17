import { api } from "$lib/api/client";
import type { VM, VMAction } from "$lib/types/admin";
import type { PaginationMetadata } from "$lib/api/vms";
import { transformKeysToCamelCase, transformKeysToSnakeCase } from "$lib/utils/transform";

export interface AdminVMPaginationParams {
  page?: number;
  limit?: number;
  search?: string;
  sortBy?: string;
  sortOrder?: string;
  node?: string;
}

export interface AdminVMListPaginatedResponse {
  vms: VM[];
  pagination: PaginationMetadata;
}

export async function getAllVMs(): Promise<VM[]> {
  const response = await api.get<Record<string, unknown>[]>("/api/v1/admin/vms");
  return transformKeysToCamelCase<VM[]>(response);
}

export async function getAllVMsPaginated(params: AdminVMPaginationParams = {}): Promise<AdminVMListPaginatedResponse> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.search) qs.set("search", params.search);
  if (params.sortBy) qs.set("sort_by", params.sortBy);
  if (params.sortOrder) qs.set("sort_order", params.sortOrder);
  if (params.node) qs.set("node", params.node);
  const query = qs.toString();
  const response = await api.get<Record<string, unknown>>(
    `/api/v1/admin/vms/paginated${query ? "?" + query : ""}`,
  );
  return transformKeysToCamelCase<AdminVMListPaginatedResponse>(response);
}

export async function vmAction(
  vmid: number,
  node: string,
  action: VMAction,
): Promise<void> {
  return api.post(
    `/api/v1/admin/vms/${vmid}/action`,
    transformKeysToSnakeCase({ action, node }),
  );
}

export async function deleteVM(vmid: number): Promise<void> {
  return api.delete(`/api/v1/admin/vms/${vmid}`);
}

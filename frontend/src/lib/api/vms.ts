import { api } from "./client";
import type { VM, VMAction, VMStatus } from "$lib/types/vm";
import { transformKeysToCamelCase } from "$lib/utils/transform";
import { validateSearchParams, type VMSearchParams as SearchParams, type SearchFilter } from "$lib/api/search";

/**
 * Summary view of a VM for list displays.
 * Derived from full VM type to ensure field consistency.
 */
export type VMSummary = Pick<VM, "vmid" | "name" | "node" | "status" | "cpu" | "cpus" | "memMb" | "maxMemMb" | "diskMb" | "uptime" | "tags">;

export interface PaginationMetadata {
  total: number;
  page: number;
  limit: number;
  totalPages: number;
  hasNext: boolean;
  hasPrev: boolean;
  runningCount: number;
  stoppedCount: number;
}

export interface PaginatedVMListResponse {
  vms: VMSummary[];
  pagination: PaginationMetadata;
}

export interface VMPaginationParams {
  page?: number;
  limit?: number;
  search?: string;
  sortBy?: string;
  sortOrder?: string;
  status?: string;
  node?: string;
}

export async function getVMs(): Promise<VMSummary[]> {
  const res = await api.get<PaginatedVMListResponse>("/api/v1/vms");
  const transformed = transformKeysToCamelCase<PaginatedVMListResponse>(res);
  return transformed.vms;
}

export async function vmAction(
  vmid: number,
  node: string,
  action: VMAction,
): Promise<void> {
  await api.post(`/api/v1/vms/${vmid}/action`, { action, node });
}

export async function getVMsPaginated(params: VMPaginationParams = {}): Promise<PaginatedVMListResponse> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.search) qs.set("search", params.search);
  if (params.sortBy) qs.set("sort_by", params.sortBy);
  if (params.sortOrder) qs.set("sort_order", params.sortOrder);
  const query = qs.toString();
  const res = await api.get<PaginatedVMListResponse>(
    `/api/v1/vms${query ? "?" + query : ""}`,
  );
  return transformKeysToCamelCase<PaginatedVMListResponse>(res);
}

export type SearchType = SearchFilter;

export type VMSearchParams = SearchParams;

export async function searchVMs(params: VMSearchParams): Promise<VMSummary[]> {
  validateSearchParams(params);
  const qs = new URLSearchParams();
  if (params.q) qs.set("q", params.q);
  if (params.type) qs.set("type", params.type);
  if (params.status) qs.set("status", params.status);
  if (params.node) qs.set("node", params.node);
  const query = qs.toString();
  const res = await api.get<PaginatedVMListResponse>(
    `/api/v1/vms${query ? "?" + query : ""}`,
  );
  const transformed = transformKeysToCamelCase<PaginatedVMListResponse>(res);
  return transformed.vms;
}

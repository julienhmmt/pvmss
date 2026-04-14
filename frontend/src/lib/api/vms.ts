import { api } from "./client";
import type { VMStatus } from "$lib/types/vm";
import { validateSearchParams, type VMSearchParams as SearchParams, type SearchFilter } from "$lib/api/search";

export interface VMSummary {
  vmid: number;
  name: string;
  node: string;
  status: VMStatus;
  cpu: number;
  cpus: number;
  mem_mb: number;
  max_mem_mb: number;
  disk_mb: number;
  uptime: number;
  tags: string;
}

export interface PaginationMetadata {
  total: number;
  page: number;
  limit: number;
  total_pages: number;
  has_next: boolean;
  has_prev: boolean;
  running_count: number;
  stopped_count: number;
}

export interface PaginatedVMListResponse {
  vms: VMSummary[];
  pagination: PaginationMetadata;
}

export interface VMPaginationParams {
  page?: number;
  limit?: number;
  search?: string;
  sort_by?: string;
  sort_order?: string;
}

export async function getVMs(): Promise<VMSummary[]> {
  const res = await api.get<PaginatedVMListResponse>("/api/v1/vms");
  return res.vms;
}

export async function getVMsPaginated(params: VMPaginationParams = {}): Promise<PaginatedVMListResponse> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.search) qs.set("search", params.search);
  if (params.sort_by) qs.set("sort_by", params.sort_by);
  if (params.sort_order) qs.set("sort_order", params.sort_order);
  const query = qs.toString();
  return api.get<PaginatedVMListResponse>(
    `/api/v1/vms${query ? "?" + query : ""}`,
  );
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
  return res.vms;
}

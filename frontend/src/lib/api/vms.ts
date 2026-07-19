import { api } from "./client";
import type { VM, VMAction, VMStatus } from "$lib/types/vm";
import { transformKeysToCamelCase } from "$lib/utils/transform";

/**
 * Summary view of a VM for list displays.
 * Derived from full VM type to ensure field consistency.
 */
export type VMSummary = Pick<VM, "vmid" | "name" | "node" | "status" | "cpu" | "cpus" | "memMb" | "maxMemMb" | "diskMb" | "maxDiskMb" | "uptime" | "tags">;

export interface PaginationMetadata {
  total: number;
  page: number;
  limit: number;
  totalPages: number;
  hasNext: boolean;
  hasPrev: boolean;
  runningCount: number;
  stoppedCount: number;
  /** Distinct nodes present in the (filtered) result set, before pagination.
   * Populated by the server for list/search UIs that need a node filter dropdown.
   * Optional because not all responses include it (omitempty on backend). */
  nodes?: string[];
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
  /** Optional search scope for legacy q+type behavior (vmid|name|tag). When provided with search, emits q+type instead of search for backend scoping. */
  type?: SearchType;
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
  if (params.sortBy) qs.set("sort_by", params.sortBy);
  if (params.sortOrder) qs.set("sort_order", params.sortOrder);
  if (params.status) qs.set("status", params.status);
  if (params.node) qs.set("node", params.node);

  // Scoped search: when a type (vmid|name|tag) is provided with a query,
  // emit the legacy q+type so backend applies the correct filterUserVM branch.
  // Otherwise fall back to the modern 'search' param (broad match).
  if (params.search) {
    if (params.type) {
      qs.set("q", params.search);
      qs.set("type", params.type);
    } else {
      qs.set("search", params.search);
    }
  }

  const query = qs.toString();
  const res = await api.get<PaginatedVMListResponse>(
    `/api/v1/vms${query ? "?" + query : ""}`,
  );
  return transformKeysToCamelCase<PaginatedVMListResponse>(res);
}

export type SearchType = "vmid" | "name" | "tag";

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

interface VMListResponse {
  vms: VMSummary[];
  total: number;
}

export async function getVMs(): Promise<VMSummary[]> {
  const res = await api.get<VMListResponse>("/api/v1/vms");
  return res.vms;
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
  const res = await api.get<VMListResponse>(
    `/api/v1/vms${query ? "?" + query : ""}`,
  );
  return res.vms;
}

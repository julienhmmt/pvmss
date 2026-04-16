import { api } from "$lib/api/client";
import type { AuditLogResponse } from "$lib/types/admin";

interface AuditLogParams {
  table?: string;
  limit?: number;
  offset?: number;
}

export function getAuditLog(params: AuditLogParams = {}): Promise<AuditLogResponse> {
  const qs = new URLSearchParams();
  if (params.table) qs.set("table", params.table);
  if (params.limit != null) qs.set("limit", String(params.limit));
  if (params.offset != null) qs.set("offset", String(params.offset));
  const query = qs.toString();
  return api.get(`/api/v1/admin/audit${query ? "?" + query : ""}`);
}

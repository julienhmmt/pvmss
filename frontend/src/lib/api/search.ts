import type { VMStatus } from "$lib/types/vm";

/** Minimum valid VM ID. */
const VM_ID_MIN = 1;

/** Maximum length for search query string. */
const SEARCH_QUERY_MAX_LENGTH = 100;

/**
 * Search filter type for VM search operations.
 */
export type SearchFilter = "vmid" | "name" | "tag";

/**
 * Parameters for VM search API.
 */
export interface VMSearchParams {
  /** Search query string */
  q?: string;
  /** Filter type: vmid, name, or tag */
  type?: SearchFilter;
  /** VM status filter (running, stopped, etc.) */
  status?: VMStatus;
  /** Proxmox node filter */
  node?: string;
}

/**
 * Validates search parameters to ensure they are valid.
 * @param params - Search parameters to validate
 * @throws Error if parameters are invalid
 */
export function validateSearchParams(params: VMSearchParams): void {
  if (params.type === "vmid" && params.q) {
    const vmid = parseInt(params.q, 10);
    if (isNaN(vmid) || vmid < VM_ID_MIN) {
      throw new Error(`Invalid VM ID: must be at least ${VM_ID_MIN} when searching by vmid`);
    }
  }

  if (params.status) {
    const validStatuses: readonly VMStatus[] = ["running", "stopped", "paused"] as const;
    if (!validStatuses.includes(params.status)) {
      throw new Error(`Invalid status: must be one of ${validStatuses.join(", ")}`);
    }
  }

  if (params.node && params.node.trim().length === 0) {
    throw new Error("Node name cannot be empty");
  }

  if (params.q && params.q.length > SEARCH_QUERY_MAX_LENGTH) {
    throw new Error(`Search query too long (max ${SEARCH_QUERY_MAX_LENGTH} characters)`);
  }

  if ((params.type === "name" || params.type === "tag") && !params.q) {
    throw new Error("Search query required when searching by name or tag");
  }
}

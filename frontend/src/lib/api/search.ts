import type { VMStatus } from "$lib/types/vm";

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
    if (isNaN(vmid) || vmid <= 0) {
      throw new Error("Invalid VM ID: must be a positive integer when searching by vmid");
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

  if (params.q && params.q.length > 100) {
    throw new Error("Search query too long (max 100 characters)");
  }

  if ((params.type === "name" || params.type === "tag") && !params.q) {
    throw new Error("Search query required when searching by name or tag");
  }
}

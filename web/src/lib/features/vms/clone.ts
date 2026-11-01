import { post } from '#lib/shared/api/client.js';

/** 202 body of POST /api/v1/vms/{cluster}/{vmid}/clone (same shape as create). */
export interface CloneVmAccepted {
	cluster: string;
	vmid: number;
	name: string;
	node: string;
	upid: string;
}

/** Proxmox/PVMSS VM names are DNS labels: lowercase, digits, hyphens, max 63. */
export const CLONE_NAME_PATTERN = '[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?';

/**
 * Suggests `<name>-copy`, then `-copy-2`, `-copy-3`... skipping names already
 * taken in the caller's pool. The server still enforces uniqueness; this only
 * saves the user a rejected submit.
 */
export function suggestCopyName(source: string, taken: readonly string[] = []): string {
	const base = `${source}-copy`;
	if (!taken.includes(base)) return base;

	let n = 2;
	while (taken.includes(`${base}-${n}`)) n++;

	return `${base}-${n}`;
}

/** Asks the server to full-clone an owned VM under a new name. */
export function cloneVm(cluster: string, vmid: number, name: string): Promise<CloneVmAccepted> {
	return post<CloneVmAccepted>(`/api/v1/vms/${encodeURIComponent(cluster)}/${vmid}/clone`, { name });
}

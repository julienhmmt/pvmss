import { get } from '#lib/shared/api/client.js';

/** One VM carrying a tag, as shown in the tag's VM dialog. */
export interface TagVM {
	cluster: string;
	vmid: number;
	name: string;
	node: string;
	status: string;
}

interface VmListItem extends TagVM {
	tags: string[] | null;
}

interface VmListPage {
	items: VmListItem[];
	total: number;
}

// Matches the server's default PVMSS_MAX_LIST_PAGE_SIZE, so one request per page.
const PAGE_SIZE = 100;

/**
 * Lists every VM on a cluster that carries the tag. The VM list endpoint has
 * no tag filter but returns each item's tags, so the scan runs client-side
 * over all pages (scope=all is honoured because the admin page is admin-only).
 */
export async function fetchTagVMs(cluster: string, tag: string): Promise<TagVM[]> {
	const matches: TagVM[] = [];
	for (let page = 1; ; page++) {
		const params = new URLSearchParams({
			cluster,
			scope: 'all',
			page: String(page),
			pageSize: String(PAGE_SIZE)
		});
		const result = await get<VmListPage>(`/api/v1/vms?${params.toString()}`);
		for (const vm of result.items) {
			if (vm.tags?.includes(tag)) {
				matches.push({
					cluster: vm.cluster,
					vmid: vm.vmid,
					name: vm.name,
					node: vm.node,
					status: vm.status
				});
			}
		}
		if (result.items.length === 0 || page * PAGE_SIZE >= result.total) return matches;
	}
}

import { get, post, ApiRequestError } from '#lib/shared/api/client.js';
import {
	fetchClusterOptions,
	resolveInitialAdminCluster,
	saveAdminCluster,
	type ClusterOption
} from '#lib/shared/clusters.js';
import { getContext, setContext } from 'svelte';
import { SvelteSet } from 'svelte/reactivity';
import { m } from '#lib/paraglide/messages.js';
import type { MachineDisplayStatus } from '#lib/features/vms/display-status.js';

export interface RecoveryVM {
	vmid: number;
	name: string;
	node: string;
	status: MachineDisplayStatus;
	pool?: string;
	tags: string[];
}

export interface RecoveryPool {
	name: string;
	comment: string;
	members: RecoveryVM[];
}

export interface RecoveryTag {
	name: string;
	vmCount: number;
	color: string;
}

export interface RecoveryHiddenPool {
	name: string;
	comment: string;
	memberCount: number;
}

export interface RecoveryHiddenTag {
	name: string;
	vmCount: number;
}

export interface RecoveryHiddenVM {
	vmid: number;
	name: string;
}

export interface RecoveryReport {
	cluster: string;
	scannedAt: string;
	pools: RecoveryPool[];
	looseVMs: RecoveryVM[];
	tags: RecoveryTag[];
	hiddenPools: RecoveryHiddenPool[];
	hiddenLooseVMs: RecoveryHiddenVM[];
	hiddenTags: RecoveryHiddenTag[];
}

/** Per-kind item selection for a hide or unhide call. */
export interface RecoveryDismissalSelection {
	pools?: string[];
	tags?: string[];
	vms?: number[];
}

interface AdoptResult {
	pools: string[];
	tags: { name: string; color: string }[];
}

function matchesNeedle(needle: string, value: string): boolean {
	return value.toLowerCase().includes(needle);
}

const ADMIN_RECOVERY_CONTEXT_KEY = Symbol('admin-recovery');

/** Manages the orphan-adoption report and the adopt selection per cluster. */
export class RecoveryStore {
	report = $state.raw<RecoveryReport | null>(null);
	loading = $state.raw(false);
	rescanning = $state.raw(false);
	error = $state.raw<string | null>(null);
	adopting = $state.raw(false);
	adoptError = $state.raw<string | null>(null);
	hiding = $state.raw(false);
	hideError = $state.raw<string | null>(null);
	announce = $state.raw<string | null>(null);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('default');
	selectedPools = new SvelteSet<string>();
	selectedTags = new SvelteSet<string>();
	/** Hide-only selection for loose VMs - adoption never touches them. */
	selectedVMs = new SvelteSet<number>();
	/** View filter applied to the three visible lists; hidden items are not filtered. */
	search = $state('');
	/** Latest-wins guard for load: a cluster switch must not show the previous cluster's report. */
	#loadRequestId = 0;

	/** Pools matching the search on name, comment or member VM names. */
	filteredPools = $derived.by(() => {
		const needle = this.search.trim().toLowerCase();
		const pools = this.report?.pools ?? [];
		if (!needle) return pools;
		return pools.filter(
			(pool) =>
				matchesNeedle(needle, pool.name) ||
				matchesNeedle(needle, pool.comment) ||
				pool.members.some((member) => matchesNeedle(needle, member.name))
		);
	});

	filteredTags = $derived.by(() => {
		const needle = this.search.trim().toLowerCase();
		const tags = this.report?.tags ?? [];
		if (!needle) return tags;
		return tags.filter((tag) => matchesNeedle(needle, tag.name));
	});

	filteredLooseVMs = $derived.by(() => {
		const needle = this.search.trim().toLowerCase();
		const vms = this.report?.looseVMs ?? [];
		if (!needle) return vms;
		return vms.filter((vm) => matchesNeedle(needle, vm.name) || String(vm.vmid).includes(needle));
	});

	/** Whether every filtered row of a list is selected - drives the per-card select-all checkbox. */
	allPoolsSelected = $derived(this.filteredPools.length > 0 && this.filteredPools.every((pool) => this.selectedPools.has(pool.name)));
	allTagsSelected = $derived(this.filteredTags.length > 0 && this.filteredTags.every((tag) => this.selectedTags.has(tag.name)));
	allVMsSelected = $derived(this.filteredLooseVMs.length > 0 && this.filteredLooseVMs.every((vm) => this.selectedVMs.has(vm.vmid)));

	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
			if (this.clusterOptions.length > 0) {
				this.cluster = resolveInitialAdminCluster(this.clusterOptions, this.cluster);
			}
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.recovery.loadError']();
		}
	}

	setCluster(value: string): void {
		saveAdminCluster(value);
		this.cluster = value;
		void this.load();
	}

	async load(): Promise<void> {
		const requestId = ++this.#loadRequestId;
		if (this.clusterOptions.length === 0) {
			await this.loadClusters();
			if (requestId !== this.#loadRequestId) return;
		}
		this.loading = true;
		this.error = null;
		try {
			const report = await get<RecoveryReport>(`/api/v1/admin/recovery?cluster=${encodeURIComponent(this.cluster)}`);
			if (requestId !== this.#loadRequestId) return;
			this.report = report;
			this.selectAllOrphans();
		} catch (error) {
			if (requestId !== this.#loadRequestId) return;
			this.error = error instanceof ApiRequestError ? error.message : m['admin.recovery.loadError']();
		} finally {
			this.loading = false;
		}
	}

	// rescan is the explicit "scan again" action: the GET report is cached
	// server-side and an empty report is pinned, so a fresh Proxmox read only
	// happens through this POST.
	async rescan(): Promise<void> {
		this.rescanning = true;
		this.error = null;
		try {
			this.report = await post<RecoveryReport>('/api/v1/admin/recovery/scan', { cluster: this.cluster });
			this.selectAllOrphans();
		} catch (error) {
			this.error = error instanceof ApiRequestError ? error.message : m['admin.recovery.loadError']();
		} finally {
			this.rescanning = false;
		}
	}

	private selectAllOrphans(): void {
		// Fresh report: default to adopting everything found - the admin
		// unchecks what should stay as-is on the cluster. Loose VMs are
		// hide-only, so they start unselected.
		this.selectedPools.clear();
		this.selectedTags.clear();
		this.selectedVMs.clear();
		for (const pool of this.report?.pools ?? []) this.selectedPools.add(pool.name);
		for (const tag of this.report?.tags ?? []) this.selectedTags.add(tag.name);
	}

	togglePool(name: string): void {
		if (this.selectedPools.has(name)) {
			this.selectedPools.delete(name);
		} else {
			this.selectedPools.add(name);
		}
	}

	toggleTag(name: string): void {
		if (this.selectedTags.has(name)) {
			this.selectedTags.delete(name);
		} else {
			this.selectedTags.add(name);
		}
	}

	toggleVM(vmid: number): void {
		if (this.selectedVMs.has(vmid)) {
			this.selectedVMs.delete(vmid);
		} else {
			this.selectedVMs.add(vmid);
		}
	}

	/** Select-all over the filtered rows only; toggles off when all are in. */
	toggleAllPools(): void {
		this.toggleAll(this.filteredPools.map((pool) => pool.name), this.selectedPools, this.allPoolsSelected);
	}

	toggleAllTags(): void {
		this.toggleAll(this.filteredTags.map((tag) => tag.name), this.selectedTags, this.allTagsSelected);
	}

	toggleAllVMs(): void {
		this.toggleAll(this.filteredLooseVMs.map((vm) => vm.vmid), this.selectedVMs, this.allVMsSelected);
	}

	private toggleAll<T>(items: T[], set: SvelteSet<T>, allSelected: boolean): void {
		for (const item of items) {
			if (allSelected) {
				set.delete(item);
			} else {
				set.add(item);
			}
		}
	}

	/** Hides the current pool/tag/VM selection in one call. */
	async hideSelected(): Promise<void> {
		const items = {
			pools: [...this.selectedPools],
			tags: [...this.selectedTags],
			vms: [...this.selectedVMs]
		};
		if (items.pools.length === 0 && items.tags.length === 0 && items.vms.length === 0) {
			this.hideError = m['admin.recovery.nothingToHide']();
			return;
		}
		await this.hideItems(items);
	}

	async hideItems(items: RecoveryDismissalSelection): Promise<void> {
		await this.dismiss('hide', items);
	}

	async unhideItems(items: RecoveryDismissalSelection): Promise<void> {
		await this.dismiss('unhide', items);
	}

	// dismiss posts a hide/unhide request and reloads the report so the
	// hidden section and adoptable lists reflect the server state.
	private async dismiss(path: 'hide' | 'unhide', items: RecoveryDismissalSelection): Promise<void> {
		this.hideError = null;
		this.announce = null;
		this.hiding = true;
		try {
			await post(`/api/v1/admin/recovery/${path}`, {
				cluster: this.cluster,
				pools: items.pools ?? [],
				tags: items.tags ?? [],
				vms: items.vms ?? []
			});
			this.announce = path === 'hide' ? m['admin.recovery.hidden']() : m['admin.recovery.unhidden']();
			await this.load();
		} catch (error) {
			this.hideError = error instanceof ApiRequestError ? error.message : m['admin.recovery.hideError']();
		} finally {
			this.hiding = false;
		}
	}

	/** Adopts one hidden pool or tag through the existing adopt endpoint. */
	async adoptHiddenItem(item: { pool?: string; tag?: string }): Promise<void> {
		this.hideError = null;
		this.announce = null;
		this.hiding = true;
		try {
			const result = await post<AdoptResult>('/api/v1/admin/recovery', {
				cluster: this.cluster,
				pools: item.pool ? [item.pool] : [],
				tags: item.tag ? [item.tag] : []
			});
			this.announce = m['admin.recovery.adopted']({ pools: result.pools.length, tags: result.tags.length });
			await this.load();
		} catch (error) {
			this.hideError = error instanceof ApiRequestError ? error.message : m['admin.recovery.adoptError']();
		} finally {
			this.hiding = false;
		}
	}

	async adopt(): Promise<void> {
		this.adoptError = null;
		this.announce = null;
		const pools = [...this.selectedPools];
		const tags = [...this.selectedTags];
		if (pools.length === 0 && tags.length === 0) {
			this.adoptError = m['admin.recovery.nothingSelected']();
			return;
		}
		this.adopting = true;
		try {
			const result = await post<AdoptResult>('/api/v1/admin/recovery', {
				cluster: this.cluster,
				pools,
				tags
			});
			this.announce = m['admin.recovery.adopted']({ pools: result.pools.length, tags: result.tags.length });
			await this.load();
		} catch (error) {
			this.adoptError = error instanceof ApiRequestError ? error.message : m['admin.recovery.adoptError']();
		} finally {
			this.adopting = false;
		}
	}
}

export function setAdminRecoveryContext(): RecoveryStore {
	const store = new RecoveryStore();
	setContext(ADMIN_RECOVERY_CONTEXT_KEY, store);
	return store;
}

export function getAdminRecoveryContext(): RecoveryStore {
	return getContext<RecoveryStore>(ADMIN_RECOVERY_CONTEXT_KEY);
}

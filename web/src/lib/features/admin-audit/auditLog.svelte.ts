import { get, ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';

export interface AuditEntry {
	id: number;
	actor: string;
	cluster: string;
	vmid: number;
	action: string;
	timestamp: string;
}

export interface AuditPage {
	items: AuditEntry[];
	total: number;
	page: number;
	pageSize: number;
}

export interface AuditFilter {
	cluster?: string;
	vmid?: number;
	actor?: string;
	action?: string;
	from?: string;
	to?: string;
	page?: number;
	pageSize?: number;
}

/**
 * AuditLogStore manages the admin audit log view: filter state and the
 * paginated result set. API responses are $state.raw — they are API data,
 * not form edits (constitution VII). One store instance per admin settings
 * page, via context.
 */
export class AuditLogStore {
	entries = $state.raw<AuditEntry[]>([]);
	total = $state.raw(0);
	page = $state.raw(1);
	pageSize = $state.raw(20);

	filter = $state<AuditFilter>({});

	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			// eslint-disable-next-line svelte/prefer-svelte-reactivity -- local request-building scratch var, not $state
			const params = new URLSearchParams();
			if (this.filter.cluster) params.set('cluster', this.filter.cluster);
			if (this.filter.vmid !== undefined) params.set('vmid', String(this.filter.vmid));
			if (this.filter.actor) params.set('actor', this.filter.actor);
			if (this.filter.action) params.set('action', this.filter.action);
			if (this.filter.from) params.set('from', this.filter.from);
			if (this.filter.to) params.set('to', this.filter.to);
			params.set('page', String(this.filter.page ?? 1));
			params.set('pageSize', String(this.filter.pageSize ?? 20));

			const result = await get<AuditPage>(`/api/v1/admin/audit?${params.toString()}`);
			this.entries = result.items;
			this.total = result.total;
			this.page = result.page;
			this.pageSize = result.pageSize;
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : 'failed to load audit log';
		} finally {
			this.loading = false;
		}
	}

	setFilter(filter: AuditFilter): void {
		this.filter = { ...filter, page: 1 };
		void this.load();
	}

	clearFilter(): void {
		this.filter = {};
		void this.load();
	}

	async nextPage(): Promise<void> {
		if (this.page * this.pageSize >= this.total) return;
		this.filter = { ...this.filter, page: this.page + 1 };
		await this.load();
	}

	async prevPage(): Promise<void> {
		if (this.page <= 1) return;
		this.filter = { ...this.filter, page: this.page - 1 };
		await this.load();
	}
}

const AUDIT_LOG_CONTEXT_KEY = Symbol('admin-audit-log');

export function setAuditLogContext(): AuditLogStore {
	const store = new AuditLogStore();
	setContext(AUDIT_LOG_CONTEXT_KEY, store);
	return store;
}

export function getAuditLogContext(): AuditLogStore {
	return getContext<AuditLogStore>(AUDIT_LOG_CONTEXT_KEY);
}

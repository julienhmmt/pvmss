import { get, put, ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { m } from '$lib/paraglide/messages.js';

export interface AuditConfig {
	retentionDays: number;
}

export interface PrunePreview {
	retentionDays: number;
	rowsToDelete: number;
}

/**
 * AuditRetentionStore manages the audit retention config: load current value,
 * preview the prune impact of a new value, and submit the change. The
 * confirmation flow requires a preview before the PUT is allowed, so an admin
 * cannot shrink retention without seeing how many rows would be deleted.
 */
export class AuditRetentionStore {
	retentionDays = $state.raw<number | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	preview = $state.raw<PrunePreview | null>(null);
	previewing = $state.raw(false);
	previewError = $state.raw<string | null>(null);

	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);
	saved = $state.raw(false);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			const cfg = await get<AuditConfig>('/api/v1/admin/audit/config');
			this.retentionDays = cfg.retentionDays;
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.audit.retention.loadError']();
		} finally {
			this.loading = false;
		}
	}

	async previewPrune(days: number): Promise<void> {
		this.previewing = true;
		this.previewError = null;
		this.preview = null;
		this.saved = false;
		try {
			const params = new SvelteURLSearchParams({ retention_days: String(days) });
			this.preview = await get<PrunePreview>(`/api/v1/admin/audit/prune-preview?${params.toString()}`);
		} catch (err) {
			this.previewError = err instanceof ApiRequestError ? err.message : m['admin.audit.retention.previewError']();
		} finally {
			this.previewing = false;
		}
	}

	async save(days: number): Promise<void> {
		this.saving = true;
		this.saveError = null;
		this.saved = false;
		try {
			const cfg = await put<AuditConfig>('/api/v1/admin/audit/config', { retentionDays: days });
			this.retentionDays = cfg.retentionDays;
			this.preview = null;
			this.saved = true;
		} catch (err) {
			this.saveError = err instanceof ApiRequestError ? err.message : m['admin.audit.retention.saveError']();
		} finally {
			this.saving = false;
		}
	}

	resetConfirm(): void {
		this.preview = null;
		this.previewError = null;
		this.saveError = null;
		this.saved = false;
	}
}

const AUDIT_RETENTION_CONTEXT_KEY = Symbol('admin-audit-retention');

export function setAuditRetentionContext(): AuditRetentionStore {
	const store = new AuditRetentionStore();
	setContext(AUDIT_RETENTION_CONTEXT_KEY, store);
	return store;
}

export function getAuditRetentionContext(): AuditRetentionStore {
	return getContext<AuditRetentionStore>(AUDIT_RETENTION_CONTEXT_KEY);
}

import { get, ApiRequestError } from '$lib/shared/api/client';
import { setContext, getContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

export interface BaselineState {
	generated: string;
	overridePresent: boolean;
	overrideFilename: string;
	overrideContent?: string;
	overrideError?: string;
}

/**
 * AdminBaselineStore manages the admin baseline view (issue 07). API
 * responses are $state.raw - they are API data, not form edits (constitution
 * VII). One store instance per admin baseline page, via context.
 */
export class AdminBaselineStore {
	state = $state.raw<BaselineState | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	async load(cluster: string): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.state = await get<BaselineState>(`/api/v1/admin/baseline?cluster=${encodeURIComponent(cluster)}`);
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.baseline.loadError']();
		} finally {
			this.loading = false;
		}
	}
}

const ADMIN_BASELINE_KEY = Symbol('admin-baseline');

export function setAdminBaselineContext(): AdminBaselineStore {
	const store = new AdminBaselineStore();
	setContext(ADMIN_BASELINE_KEY, store);
	return store;
}

export function getAdminBaselineContext(): AdminBaselineStore {
	return getContext<AdminBaselineStore>(ADMIN_BASELINE_KEY);
}

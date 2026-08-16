import { getContext, setContext } from 'svelte';
import { get, post, ApiRequestError } from '$lib/shared/api/client';

export type NodeStatus = 'online' | 'offline' | 'unknown';

export interface ClusterNode {
	name: string;
	status: NodeStatus;
	cpuCores: number;
	cpuUsage: number;
	memoryTotal: number;
	memoryUsed: number;
	storageTotal: number;
	storageUsed: number;
	vmCount: number;
}

interface NodesResponse {
	nodes: ClusterNode[];
	refreshedAt: string;
}

interface RefreshResponse {
	refreshedAt: string;
}

export class NodesStore {
	nodes = $state.raw<ClusterNode[]>([]);
	refreshedAt = $state.raw<string | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	errorCode = $state.raw<string | null>(null);
	errorStatus = $state.raw<number | null>(null);
	refreshing = $state.raw(false);
	refreshError = $state.raw<string | null>(null);
	refreshDisabled = $state.raw(false);

	// Timer that clears refreshDisabled once the guard window reported by the
	// server has elapsed — without it, a user who hits the guard once is
	// stuck with a disabled button for the rest of the session (T03 quickstart
	// step 6: "wait out the guard interval, click again — it works").
	#reenableTimer: ReturnType<typeof setTimeout> | null = null;

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		this.errorCode = null;
		this.errorStatus = null;
		try {
			const response = await get<NodesResponse>('/api/v1/cluster/nodes');
			this.nodes = response.nodes;
			this.refreshedAt = response.refreshedAt;
		} catch (err) {
			if (err instanceof ApiRequestError) {
				this.error = err.message;
				this.errorCode = err.code;
				this.errorStatus = err.status;
			} else {
				this.error = 'failed to load cluster nodes';
				this.errorCode = null;
				this.errorStatus = null;
			}
		} finally {
			this.loading = false;
		}
	}

	async refresh(): Promise<void> {
		this.refreshing = true;
		this.refreshError = null;
		try {
			const response = await post<RefreshResponse>('/api/v1/cluster/refresh');
			this.refreshedAt = response.refreshedAt;
			this.clearRefreshDisabled();
			await this.load();
		} catch (err) {
			if (err instanceof ApiRequestError && err.code === 'refresh_too_soon') {
				this.refreshDisabled = true;
				this.refreshError = err.message;
				this.#scheduleReenable(err.retryAfterSeconds ?? 5);
			} else {
				this.refreshError = err instanceof ApiRequestError ? err.message : 'failed to refresh';
			}
		} finally {
			this.refreshing = false;
		}
	}

	/** Re-enables the refresh button once the server's guard window has
	 * elapsed, so the user is not stuck until a page reload. */
	#scheduleReenable(seconds: number): void {
		if (this.#reenableTimer !== null) clearTimeout(this.#reenableTimer);
		this.#reenableTimer = setTimeout(() => {
			this.#reenableTimer = null;
			this.clearRefreshDisabled();
		}, seconds * 1000);
	}

	clearRefreshDisabled(): void {
		if (this.#reenableTimer !== null) {
			clearTimeout(this.#reenableTimer);
			this.#reenableTimer = null;
		}
		this.refreshDisabled = false;
		this.refreshError = null;
	}
}

const NODES_CONTEXT_KEY = Symbol('nodes');

/** Called once, by the route that owns this state (constitution VII: no module singletons). */
export function setNodesContext(): NodesStore {
	const store = new NodesStore();
	setContext(NODES_CONTEXT_KEY, store);
	return store;
}

export function getNodesContext(): NodesStore {
	return getContext<NodesStore>(NODES_CONTEXT_KEY);
}

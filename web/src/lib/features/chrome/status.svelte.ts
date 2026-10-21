import { getContext, setContext } from 'svelte';
import { get, post } from '#lib/shared/api/client.js';

/** Server-side health check entry (subset of httpapi.CheckResult). */
export interface CheckResult {
	status: string;
	detail?: string;
}

/**
 * Cluster-down detail extracted from checks.clusters.detail. The backend
 * returns the detail as a fixed string such as "1 of 1 clusters unreachable".
 * This function parses it into (unreachable, total) counts so the banner can
 * render a localized, parameterized message instead of concatenating the
 * raw English detail to a translated base message.
 */
export function clusterDownCounts(raw: HealthResponse | null): { unreachable: number; total: number } | null {
	const detail = raw?.checks?.clusters?.detail;
	if (raw?.checks?.clusters?.status !== 'unhealthy' || !detail) {
		return null;
	}

	const match = /(\d+)\s+of\s+(\d+)\s+clusters?\s+unreachable/i.exec(detail);
	if (match == null || match[1] == null || match[2] == null) {
		return null;
	}

	return { unreachable: parseInt(match[1], 10), total: parseInt(match[2], 10) };
}

/** GET /health response shape (extended by T19 with checks.clusters + demoMode). */
export interface HealthResponse {
	status: string;
	checks: Record<string, CheckResult>;
	demoMode: boolean;
	timestamp: string;
}

/** Banner severity, from most to least urgent. First match wins (data-model.md). */
export type Severity = 'none' | 'info' | 'degraded' | 'unhealthy' | 'unknown';

/** Polling cadence - roughly half of T03's 30s server refresh interval. */
const POLL_INTERVAL_MS = 15_000;
/** Consecutive failures double the wait (30s, 60s, ...) up to this cap. */
const MAX_POLL_INTERVAL_MS = 120_000;

/** Fetcher seam - overridable in tests. */
export type HealthFetcher = (signal?: AbortSignal) => Promise<HealthResponse>;

const defaultFetcher: HealthFetcher = (signal) => get<HealthResponse>('/health', signal ? { signal } : {});

/**
 * StatusState polls the extended GET /health and derives one severity for
 * StatusBanner.svelte. raw is $state.raw (constitution VII: API response data,
 * reassigned whole, never mutated in place). A failed poll sets severity to
 * "unknown" - distinct from "none", per spec.md Edge Cases: silence would
 * misreport an unreachable check as "everything is fine."
 */
export class StatusState {
	#raw = $state.raw<HealthResponse | null>(null);
	#pollFailed = $state.raw(false);
	#timer: ReturnType<typeof setTimeout> | null = null;
	#running = false;
	#failures = 0;
	#inFlight: AbortController | null = null;
	#fetcher: HealthFetcher;

	constructor(fetcher: HealthFetcher = defaultFetcher) {
		this.#fetcher = fetcher;
	}

	get raw(): HealthResponse | null {
		return this.#raw;
	}

	get severity(): Severity {
		if (this.#pollFailed) return 'unknown';
		const data = this.#raw;
		if (data === null) return 'none';
		if (data.status === 'unhealthy') return 'unhealthy';
		if (data.checks?.clusters?.status === 'unhealthy') return 'degraded';
		if (data.demoMode === true) return 'info';
		return 'none';
	}

	/** The cluster-unreachable counts when severity is 'degraded', else null. */
	get clusterDownCounts(): { unreachable: number; total: number } | null {
		return clusterDownCounts(this.#raw);
	}

	/** True when every configured cluster is unreachable. */
	get allClustersDown(): boolean {
		const counts = this.clusterDownCounts;
		return counts != null && counts.unreachable > 0 && counts.unreachable === counts.total;
	}

	/** Triggers a cluster refresh then re-polls health - the banner retry action. */
	async retryClusterConnection(): Promise<void> {
		try {
			await post('/api/v1/cluster/refresh');
		} catch {
			// The refresh may fail (cluster still down) - re-poll reports the
			// current state either way.
		}
		await this.#tick();
	}

	/**
	 * Immediate poll, then one poll per cadence while the tab is visible:
	 * nothing while hidden, an immediate poll on return, and a doubling wait
	 * while the server keeps failing. Called once on mount.
	 */
	start(): void {
		this.#running = true;
		document.addEventListener('visibilitychange', this.#onVisibility);
		void this.#tick();
	}

	/** Stops the cadence and aborts a poll in flight. */
	stop(): void {
		this.#running = false;
		document.removeEventListener('visibilitychange', this.#onVisibility);
		this.#clearTimer();
		this.#inFlight?.abort();
		this.#inFlight = null;
	}

	/** One poll: success → reassign raw whole; failure → set the failed flag.
	 *  Skipped while a previous poll is still in flight. */
	async pollOnce(): Promise<void> {
		if (this.#inFlight !== null) return;
		const controller = new AbortController();
		this.#inFlight = controller;
		try {
			const response = await this.#fetcher(controller.signal);
			if (controller.signal.aborted) return;
			this.#raw = response;
			this.#pollFailed = false;
			this.#failures = 0;
		} catch {
			if (controller.signal.aborted) return;
			this.#pollFailed = true;
			this.#failures += 1;
		} finally {
			if (this.#inFlight === controller) this.#inFlight = null;
		}
	}

	#onVisibility = (): void => {
		if (document.visibilityState === 'visible') void this.#tick();
		else this.#clearTimer();
	};

	async #tick(): Promise<void> {
		this.#clearTimer();
		await this.pollOnce();
		if (!this.#running || document.visibilityState === 'hidden') return;
		this.#clearTimer();
		const delay = Math.min(POLL_INTERVAL_MS * 2 ** this.#failures, MAX_POLL_INTERVAL_MS);
		this.#timer = setTimeout(() => void this.#tick(), delay);
	}

	#clearTimer(): void {
		if (this.#timer !== null) clearTimeout(this.#timer);
		this.#timer = null;
	}
}

const STATUS_CONTEXT_KEY = Symbol('status');

/** Called once by the app shell layout. */
export function setStatusContext(fetcher?: HealthFetcher): StatusState {
	const state = new StatusState(fetcher);
	setContext(STATUS_CONTEXT_KEY, state);
	return state;
}

export function getStatusContext(): StatusState {
	return getContext<StatusState>(STATUS_CONTEXT_KEY);
}

import { getContext, setContext } from 'svelte';
import { get } from '$lib/shared/api/client';

/** Server-side health check entry (subset of httpapi.CheckResult). */
export interface CheckResult {
	status: string;
	detail?: string;
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

/** Polling cadence — roughly half of T03's 30s server refresh interval. */
const POLL_INTERVAL_MS = 15_000;

/** Fetcher seam — overridable in tests. */
export type HealthFetcher = () => Promise<HealthResponse>;

const defaultFetcher: HealthFetcher = () => get<HealthResponse>('/health');

/**
 * StatusState polls the extended GET /health and derives one severity for
 * StatusBanner.svelte. raw is $state.raw (constitution VII: API response data,
 * reassigned whole, never mutated in place). A failed poll sets severity to
 * "unknown" — distinct from "none", per spec.md Edge Cases: silence would
 * misreport an unreachable check as "everything is fine."
 */
export class StatusState {
	#raw = $state.raw<HealthResponse | null>(null);
	#pollFailed = $state.raw(false);
	#timer: ReturnType<typeof setInterval> | null = null;
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

	/** Immediate poll, then setInterval at the fixed cadence. Called once on mount. */
	start(): void {
		void this.pollOnce();
		this.#timer = setInterval(() => void this.pollOnce(), POLL_INTERVAL_MS);
	}

	/** Clears the interval — symmetric lifecycle for testability. */
	stop(): void {
		if (this.#timer !== null) clearInterval(this.#timer);
		this.#timer = null;
	}

	/** One poll: success → reassign raw whole; failure → set the failed flag. */
	async pollOnce(): Promise<void> {
		try {
			const response = await this.#fetcher();
			this.#raw = response;
			this.#pollFailed = false;
		} catch {
			this.#pollFailed = true;
		}
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

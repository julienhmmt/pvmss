import { fetchMetricsHistory, type MetricsRange, type MetricsSample } from './metrics';

export type { MetricsRange, MetricsSample };

/**
 * MetricsStore — owns the metrics-history fetch and the selected range for
 * one VM's Overview tab. One instance per VmMetricsRow.svelte (constitution
 * VII: no module singletons), matching ConsoleStore's shape. Ticket 03 will
 * extend this same class with the live SSE ticker; ticket 02's scope is
 * history-only.
 */
export class MetricsStore {
	readonly cluster: string;
	readonly vmid: number;

	range = $state<MetricsRange>('hour');
	samples = $state<MetricsSample[]>([]);
	loading = $state(false);
	error = $state<string | null>(null);

	#requestId = 0;

	constructor(cluster: string, vmid: number) {
		this.cluster = cluster;
		this.vmid = vmid;
	}

	/** Fetches history for the current range. Safe to call repeatedly. */
	async loadHistory(): Promise<void> {
		this.loading = true;
		this.error = null;

		const requestId = ++this.#requestId;

		try {
			const history = await fetchMetricsHistory(this.cluster, this.vmid, this.range);
			// A later call may have already resolved while this one was in
			// flight (e.g. rapid range switching) — only the most recent
			// request may write into state.
			if (requestId !== this.#requestId) return;
			this.samples = history.samples;
		} catch (err) {
			if (requestId !== this.#requestId) return;
			this.error = err instanceof Error ? err.message : 'failed to load metrics history';
		} finally {
			if (requestId === this.#requestId) this.loading = false;
		}
	}

	/** Switches the range and re-fetches, unless it's already selected. */
	async setRange(range: MetricsRange): Promise<void> {
		if (this.range === range) return;
		this.range = range;
		await this.loadHistory();
	}
}

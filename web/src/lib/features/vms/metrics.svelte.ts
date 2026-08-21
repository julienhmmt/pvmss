import {
	buildMetricsStreamURL,
	fetchMetricsHistory,
	parseMetricsStreamMessage,
	type MetricsRange,
	type MetricsSample
} from './metrics';

export type { MetricsRange, MetricsSample };

export type MetricsStreamState = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'error';

const metricsMaxLiveTicks = 20;
const metricsReconnectDelayMs = 3000;

/**
 * MetricsStore — owns the metrics-history fetch, the selected range, and the
 * live SSE ticker for one VM's Overview tab. One instance per VmMetricsRow
 * (constitution VII: no module singletons), matching ConsoleStore's shape:
 * connect on mount while the VM is running, disconnect on unmount.
 */
export class MetricsStore {
	readonly cluster: string;
	readonly vmid: number;
	readonly #isRunning: () => boolean;

	range = $state<MetricsRange>('hour');
	#history = $state<MetricsSample[]>([]);
	#liveTicks = $state<MetricsSample[]>([]);
	loading = $state(false);
	#requestId = 0;
	error = $state<string | null>(null);

	streamState = $state<MetricsStreamState>('idle');
	streamError = $state<string | null>(null);

	#eventSource: EventSource | null = null;
	#reconnectTimer: ReturnType<typeof setTimeout> | null = null;

	samples = $derived([...this.#history, ...this.#liveTicks]);

	constructor(cluster: string, vmid: number, isRunning: () => boolean) {
		this.cluster = cluster;
		this.vmid = vmid;
		this.#isRunning = isRunning;
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
			this.#history = history.samples;
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

	/**
	 * Opens the SSE stream to the live metrics endpoint. Idempotent: calling
	 * while already connected is a no-op. Only opens while #isRunning() is true,
	 * matching the ConsoleBanner gate that hides the console for stopped VMs.
	 */
	connect(): void {
		if (this.#eventSource !== null) return;
		if (!this.#isRunning()) return;

		this.streamState = 'connecting';
		this.streamError = null;

		const url = buildMetricsStreamURL(this.cluster, this.vmid);
		const source = new EventSource(url);
		this.#eventSource = source;

		source.onopen = () => {
			this.streamState = 'connected';
			this.streamError = null;
		};

		source.onmessage = (event) => {
			try {
				const sample = parseMetricsStreamMessage(event.data);
				this.#mergeLiveTick(sample);
			} catch (err) {
				this.streamError = err instanceof Error ? err.message : 'invalid metrics tick';
			}
		};

		source.onerror = () => {
			this.streamState = 'reconnecting';
			this.#scheduleReconnect();
		};
	}

	/**
	 * Closes the SSE stream and cancels any pending reconnect. Safe to call
	 * when already idle.
	 */
	disconnect(): void {
		if (this.#reconnectTimer !== null) {
			clearTimeout(this.#reconnectTimer);
			this.#reconnectTimer = null;
		}

		if (this.#eventSource !== null) {
			this.#eventSource.close();
			this.#eventSource = null;
		}

		this.streamState = 'idle';
		this.streamError = null;
	}

	#mergeLiveTick(sample: MetricsSample): void {
		this.#liveTicks = [...this.#liveTicks, sample].slice(-metricsMaxLiveTicks);
	}

	#scheduleReconnect(): void {
		if (this.#reconnectTimer !== null) return;

		this.#reconnectTimer = setTimeout(() => {
			this.#reconnectTimer = null;
			this.disconnect();

			if (this.#isRunning()) {
				this.connect();
			}
		}, metricsReconnectDelayMs);
	}
}

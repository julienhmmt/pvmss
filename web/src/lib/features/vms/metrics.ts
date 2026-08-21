import { base } from '$app/paths';
import { get } from '$lib/shared/api/client';

/** The three timeframes GET .../metrics/history accepts. */
export type MetricsRange = 'hour' | 'day' | 'week';

/** One point in a VM's metrics history, matching the Go handler's DTO field-for-field. */
export interface MetricsSample {
	timestamp: string;
	cpuPercent: number;
	memoryUsedBytes: number;
	memoryMaxBytes: number;
	diskReadBytesPerSec: number;
	diskWriteBytesPerSec: number;
	netInBytesPerSec: number;
	netOutBytesPerSec: number;
}

/** The full response body of GET .../metrics/history. */
export interface MetricsHistory {
	range: MetricsRange;
	samples: MetricsSample[];
}

/** Builds the same-origin path to the metrics history endpoint. */
export function buildMetricsHistoryURL(cluster: string, vmid: number, range: MetricsRange): string {
	return `${base}/api/v1/vms/${encodeURIComponent(cluster)}/${vmid}/metrics/history?range=${range}`;
}

/** Fetches a VM's metrics history for the given range. */
export async function fetchMetricsHistory(cluster: string, vmid: number, range: MetricsRange): Promise<MetricsHistory> {
	return get<MetricsHistory>(buildMetricsHistoryURL(cluster, vmid, range));
}

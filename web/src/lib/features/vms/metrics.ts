import { get } from '$lib/shared/api/client';
import { apiPath } from '$lib/shared/api/paths';

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
	return apiPath(`/api/v1/vms/${encodeURIComponent(cluster)}/${vmid}/metrics/history?range=${range}`);
}

/** Fetches a VM's metrics history for the given range. */
export async function fetchMetricsHistory(cluster: string, vmid: number, range: MetricsRange): Promise<MetricsHistory> {
	return get<MetricsHistory>(buildMetricsHistoryURL(cluster, vmid, range));
}

/** Builds the same-origin path to the live metrics SSE endpoint. */
export function buildMetricsStreamURL(cluster: string, vmid: number): string {
	return apiPath(`/api/v1/vms/${encodeURIComponent(cluster)}/${vmid}/metrics/stream`);
}

/** One live tick from the SSE stream, shaped the same as a historical sample. */
export type MetricsStreamMessage = MetricsSample;

/**
 * Parses an SSE `message` event's `data` field into a live metrics sample.
 * Throws if the payload is not valid JSON with the expected numeric fields.
 */
export function parseMetricsStreamMessage(data: string): MetricsStreamMessage {
	const parsed: unknown = JSON.parse(data);

	if (parsed === null || typeof parsed !== 'object') {
		throw new TypeError('metrics stream message is not an object');
	}

	const sample = parsed as Record<string, unknown>;

	function number(key: string): number {
		if (typeof sample[key] !== 'number') {
			throw new TypeError(`metrics stream message missing or invalid ${key}`);
		}

		return sample[key];
	}

	return {
		timestamp: string(sample.timestamp),
		cpuPercent: number('cpuPercent'),
		memoryUsedBytes: number('memoryUsedBytes'),
		memoryMaxBytes: number('memoryMaxBytes'),
		diskReadBytesPerSec: number('diskReadBytesPerSec'),
		diskWriteBytesPerSec: number('diskWriteBytesPerSec'),
		netInBytesPerSec: number('netInBytesPerSec'),
		netOutBytesPerSec: number('netOutBytesPerSec')
	};
}

function string(value: unknown): string {
	if (typeof value !== 'string') {
		throw new TypeError('metrics stream message missing or invalid timestamp');
	}

	return value;
}

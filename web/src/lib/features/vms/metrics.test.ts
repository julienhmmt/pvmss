import { afterEach, describe, expect, it, vi } from 'vitest';
import { buildMetricsHistoryURL, buildMetricsStreamURL, fetchMetricsHistory, parseMetricsStreamMessage } from './metrics';
import { ApiRequestError } from '$lib/shared/api/client';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

afterEach(() => vi.unstubAllGlobals());

describe('buildMetricsHistoryURL', () => {
	it('builds the history endpoint path with the range query param', () => {
		expect(buildMetricsHistoryURL('default', 100, 'hour')).toBe('/api/v1/vms/default/100/metrics/history?range=hour');
	});

	it('encodes the cluster into the path', () => {
		expect(buildMetricsHistoryURL('my cluster', 202, 'day')).toBe('/api/v1/vms/my%20cluster/202/metrics/history?range=day');
	});
});

describe('fetchMetricsHistory', () => {
	it('GETs the history endpoint and returns the parsed samples', async () => {
		const body = { range: 'hour', samples: [{ timestamp: '2026-01-01T00:00:00Z', cpuPercent: 12.5, memoryUsedBytes: 1, memoryMaxBytes: 2, diskReadBytesPerSec: 3, diskWriteBytesPerSec: 4, netInBytesPerSec: 5, netOutBytesPerSec: 6 }] };
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, body));
		vi.stubGlobal('fetch', fetchMock);

		const history = await fetchMetricsHistory('default', 100, 'hour');

		expect(history).toEqual(body);
		const [path] = fetchMock.mock.calls[0] ?? [];
		expect(path).toBe('/api/v1/vms/default/100/metrics/history?range=hour');
	});

	it('throws ApiRequestError on 403 (non-owner)', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(403, { code: 'forbidden', message: 'not your VM' })));
		await expect(fetchMetricsHistory('default', 100, 'hour')).rejects.toThrow(ApiRequestError);
	});
});

describe('buildMetricsStreamURL', () => {
	it('builds the stream endpoint path without query params', () => {
		expect(buildMetricsStreamURL('default', 100)).toBe('/api/v1/vms/default/100/metrics/stream');
	});

	it('encodes the cluster into the path', () => {
		expect(buildMetricsStreamURL('my cluster', 202)).toBe('/api/v1/vms/my%20cluster/202/metrics/stream');
	});
});

describe('parseMetricsStreamMessage', () => {
	it('parses a valid SSE data payload', () => {
		const data = JSON.stringify({
			timestamp: '2026-01-01T00:00:00Z',
			cpuPercent: 12.5,
			memoryUsedBytes: 1,
			memoryMaxBytes: 2,
			diskReadBytesPerSec: 3,
			diskWriteBytesPerSec: 4,
			netInBytesPerSec: 5,
			netOutBytesPerSec: 6
		});

		const sample = parseMetricsStreamMessage(data);

		expect(sample).toEqual({
			timestamp: '2026-01-01T00:00:00Z',
			cpuPercent: 12.5,
			memoryUsedBytes: 1,
			memoryMaxBytes: 2,
			diskReadBytesPerSec: 3,
			diskWriteBytesPerSec: 4,
			netInBytesPerSec: 5,
			netOutBytesPerSec: 6
		});
	});

	it('throws when the payload is not an object', () => {
		expect(() => parseMetricsStreamMessage('"not-an-object"')).toThrow('not an object');
	});

	it('throws when a numeric field is missing', () => {
		const data = JSON.stringify({
			timestamp: '2026-01-01T00:00:00Z',
			cpuPercent: 12.5,
			memoryUsedBytes: 1,
			memoryMaxBytes: 2,
			diskReadBytesPerSec: 3,
			diskWriteBytesPerSec: 4,
			netInBytesPerSec: 5
		});

		expect(() => parseMetricsStreamMessage(data)).toThrow('netOutBytesPerSec');
	});

	it('throws when a numeric field is the wrong type', () => {
		const data = JSON.stringify({
			timestamp: '2026-01-01T00:00:00Z',
			cpuPercent: '12.5',
			memoryUsedBytes: 1,
			memoryMaxBytes: 2,
			diskReadBytesPerSec: 3,
			diskWriteBytesPerSec: 4,
			netInBytesPerSec: 5,
			netOutBytesPerSec: 6
		});

		expect(() => parseMetricsStreamMessage(data)).toThrow('cpuPercent');
	});

	it('throws when the timestamp is not a string', () => {
		const data = JSON.stringify({
			timestamp: 1234567890,
			cpuPercent: 12.5,
			memoryUsedBytes: 1,
			memoryMaxBytes: 2,
			diskReadBytesPerSec: 3,
			diskWriteBytesPerSec: 4,
			netInBytesPerSec: 5,
			netOutBytesPerSec: 6
		});

		expect(() => parseMetricsStreamMessage(data)).toThrow('timestamp');
	});
});

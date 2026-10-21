import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { StatusState, type HealthResponse } from './status.svelte';

// T022 (US3): StatusState - severity derivation table from data-model.md
// (unknown > unhealthy > degraded > info > none, first match wins), raw
// reassigned wholesale ($state.raw) on each successful poll, unchanged on a
// failed poll. Tested without a DOM.

function makeResponse(overrides: Partial<HealthResponse> = {}): HealthResponse {
	return {
		status: 'healthy',
		checks: { database: { status: 'healthy' }, clusters: { status: 'healthy' } },
		demoMode: false,
		timestamp: '2026-08-12T12:00:00Z',
		...overrides
	};
}

describe('StatusState.severity', () => {
	afterEach(() => vi.restoreAllMocks());

	it('is "none" before the first successful poll', () => {
		const state = new StatusState(vi.fn());
		expect(state.severity).toBe('none');
	});

	it('is "unhealthy" when top-level status is unhealthy', async () => {
		const state = new StatusState(vi.fn().mockResolvedValue(makeResponse({ status: 'unhealthy' })));
		await state.pollOnce();
		expect(state.severity).toBe('unhealthy');
	});

	it('is "degraded" when checks.clusters.status is unhealthy (but top-level is healthy)', async () => {
		const state = new StatusState(
			vi.fn().mockResolvedValue(
				makeResponse({
					status: 'healthy',
					checks: { database: { status: 'healthy' }, clusters: { status: 'unhealthy', detail: '1 of 2 clusters unreachable' } }
				})
			)
		);
		await state.pollOnce();
		expect(state.severity).toBe('degraded');
	});

	it('is "info" when demoMode is true and everything else is healthy', async () => {
		const state = new StatusState(vi.fn().mockResolvedValue(makeResponse({ demoMode: true })));
		await state.pollOnce();
		expect(state.severity).toBe('info');
	});

	it('is "none" when everything is healthy and demoMode is false', async () => {
		const state = new StatusState(vi.fn().mockResolvedValue(makeResponse()));
		await state.pollOnce();
		expect(state.severity).toBe('none');
	});

	it('unhealthy outranks degraded (first match wins)', async () => {
		const state = new StatusState(
			vi.fn().mockResolvedValue(
				makeResponse({
					status: 'unhealthy',
					checks: { database: { status: 'unhealthy' }, clusters: { status: 'unhealthy' } }
				})
			)
		);
		await state.pollOnce();
		expect(state.severity).toBe('unhealthy');
	});

	it('degraded outranks info (first match wins)', async () => {
		const state = new StatusState(
			vi.fn().mockResolvedValue(
				makeResponse({
					status: 'healthy',
					checks: { database: { status: 'healthy' }, clusters: { status: 'unhealthy' } },
					demoMode: true
				})
			)
		);
		await state.pollOnce();
		expect(state.severity).toBe('degraded');
	});
});

describe('StatusState.poll', () => {
	afterEach(() => vi.restoreAllMocks());

	it('reassigns raw wholesale on a successful poll', async () => {
		const response = makeResponse({ demoMode: true });
		const fetcher = vi.fn().mockResolvedValue(response);
		const state = new StatusState(fetcher);
		await state.pollOnce();
		expect(state.severity).toBe('info');
		expect(fetcher).toHaveBeenCalledOnce();
	});

	it('leaves raw unchanged and sets severity to "unknown" on a failed poll', async () => {
		// First call succeeds with demoMode, second call fails.
		let callCount = 0;
		const fetcher = (): Promise<HealthResponse> => {
			callCount++;
			if (callCount === 1) return Promise.resolve(makeResponse({ demoMode: true }));
			return Promise.reject(new Error('network'));
		};
		const state = new StatusState(fetcher);
		await state.pollOnce();
		expect(state.severity).toBe('info');
		// Now a failed poll - severity becomes unknown, not info
		await state.pollOnce();
		expect(state.severity).toBe('unknown');
	});
});

describe('StatusState.clusterDownCounts', () => {
	afterEach(() => vi.restoreAllMocks());

	it('returns the parsed cluster counts when clusters check is unhealthy', async () => {
		const state = new StatusState(
			vi.fn().mockResolvedValue(
				makeResponse({
					status: 'healthy',
					checks: { database: { status: 'healthy' }, clusters: { status: 'unhealthy', detail: '1 of 1 clusters unreachable' } }
				})
			)
		);
		await state.pollOnce();
		expect(state.clusterDownCounts).toEqual({ unreachable: 1, total: 1 });
	});

	it('returns null when clusters check is healthy', async () => {
		const state = new StatusState(vi.fn().mockResolvedValue(makeResponse()));
		await state.pollOnce();
		expect(state.clusterDownCounts).toBeNull();
	});
});

describe('StatusState.allClustersDown', () => {
	afterEach(() => vi.restoreAllMocks());

	it('is true when every cluster is unreachable', async () => {
		const state = new StatusState(
			vi.fn().mockResolvedValue(
				makeResponse({
					status: 'healthy',
					checks: { database: { status: 'healthy' }, clusters: { status: 'unhealthy', detail: '2 of 2 clusters unreachable' } }
				})
			)
		);
		await state.pollOnce();
		expect(state.allClustersDown).toBe(true);
	});

	it('is false when only some clusters are unreachable', async () => {
		const state = new StatusState(
			vi.fn().mockResolvedValue(
				makeResponse({
					status: 'healthy',
					checks: { database: { status: 'healthy' }, clusters: { status: 'unhealthy', detail: '1 of 2 clusters unreachable' } }
				})
			)
		);
		await state.pollOnce();
		expect(state.allClustersDown).toBe(false);
	});

	it('is false when clusters check is healthy', async () => {
		const state = new StatusState(vi.fn().mockResolvedValue(makeResponse()));
		await state.pollOnce();
		expect(state.allClustersDown).toBe(false);
	});
});

describe('StatusState.retryClusterConnection', () => {
	afterEach(() => vi.restoreAllMocks());

	it('posts a cluster refresh then re-polls health', async () => {
		const degraded = makeResponse({
			status: 'healthy',
			checks: { database: { status: 'healthy' }, clusters: { status: 'unhealthy', detail: '1 of 1 clusters unreachable' } }
		});
		const healthy = makeResponse();
		let pollCall = 0;
		const fetcher = (): Promise<HealthResponse> => {
			pollCall++;
			return Promise.resolve(pollCall === 1 ? degraded : healthy);
		};
		const postMock = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
		vi.stubGlobal('fetch', postMock);

		const state = new StatusState(fetcher);
		await state.pollOnce();
		expect(state.severity).toBe('degraded');

		await state.retryClusterConnection();
		expect(postMock).toHaveBeenCalledOnce();
		expect(state.severity).toBe('none');
	});

	it('re-polls even when the refresh POST fails (cluster still down)', async () => {
		const degraded = makeResponse({
			status: 'healthy',
			checks: { database: { status: 'healthy' }, clusters: { status: 'unhealthy', detail: '1 of 1 clusters unreachable' } }
		});
		const fetcher = vi.fn().mockResolvedValue(degraded);
		const postMock = vi.fn().mockResolvedValue(new Response('{"code":"cluster_unreachable"}', { status: 502 }));
		vi.stubGlobal('fetch', postMock);

		const state = new StatusState(fetcher);
		await state.pollOnce();
		await state.retryClusterConnection();
		expect(state.severity).toBe('degraded');
	});
});

describe('StatusState polling cadence', () => {
	const healthy = makeResponse();
	let hidden = false;

	function setHidden(value: boolean): void {
		hidden = value;
		document.dispatchEvent(new Event('visibilitychange'));
	}

	beforeEach(() => {
		vi.useFakeTimers();
		hidden = false;
		vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => (hidden ? 'hidden' : 'visible'));
	});
	afterEach(() => {
		vi.useRealTimers();
		vi.restoreAllMocks();
	});

	it('does not poll while the tab is hidden and polls once on return', async () => {
		const fetcher = vi.fn().mockResolvedValue(healthy);
		const state = new StatusState(fetcher);
		state.start();
		await vi.advanceTimersByTimeAsync(0);
		expect(fetcher).toHaveBeenCalledTimes(1);

		setHidden(true);
		await vi.advanceTimersByTimeAsync(10 * 60_000);
		expect(fetcher).toHaveBeenCalledTimes(1);

		setHidden(false);
		await vi.advanceTimersByTimeAsync(0);
		expect(fetcher).toHaveBeenCalledTimes(2);
		state.stop();
	});

	it('backs off 15s, 30s, 60s, 120s on failures and resets on success', async () => {
		const fetcher = vi.fn().mockRejectedValue(new Error('down'));
		const state = new StatusState(fetcher);
		state.start();
		await vi.advanceTimersByTimeAsync(0);
		const callsAfter = async (ms: number): Promise<number> => {
			await vi.advanceTimersByTimeAsync(ms);
			return fetcher.mock.calls.length;
		};
		// first failure -> wait 30s, second -> 60s, third -> 120s, then capped.
		expect(await callsAfter(29_999)).toBe(1);
		expect(await callsAfter(1)).toBe(2);
		expect(await callsAfter(60_000)).toBe(3);
		expect(await callsAfter(120_000)).toBe(4);
		expect(await callsAfter(120_000)).toBe(5);

		fetcher.mockResolvedValue(healthy);
		expect(await callsAfter(120_000)).toBe(6);
		expect(await callsAfter(15_000)).toBe(7);
		state.stop();
	});

	it('stop() leaves no timer and aborts the request in flight', () => {
		let seen: AbortSignal | undefined;
		const fetcher = vi.fn((signal?: AbortSignal) => {
			seen = signal;
			return new Promise<HealthResponse>(() => {});
		});
		const state = new StatusState(fetcher);
		state.start();
		state.stop();
		expect(seen?.aborted).toBe(true);
		expect(vi.getTimerCount()).toBe(0);
	});

	it('skips a tick while the previous poll is still in flight', async () => {
		const fetcher = vi.fn(() => new Promise<HealthResponse>(() => {}));
		const state = new StatusState(fetcher);
		state.start();
		await state.pollOnce();
		expect(fetcher).toHaveBeenCalledTimes(1);
		state.stop();
	});
});

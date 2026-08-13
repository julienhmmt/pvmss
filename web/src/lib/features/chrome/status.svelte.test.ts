import { afterEach, describe, expect, it, vi } from 'vitest';
import { StatusState, type HealthResponse } from './status.svelte';

// T022 (US3): StatusState — severity derivation table from data-model.md
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
		// Now a failed poll — severity becomes unknown, not info
		await state.pollOnce();
		expect(state.severity).toBe('unknown');
	});
});

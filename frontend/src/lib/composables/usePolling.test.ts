import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { usePolling } from './usePolling.svelte';
import { ApiRequestError } from '$lib/types/api';

const apiError = (status: number) =>
	new ApiRequestError(status, { code: 'err', message: `HTTP ${status}` });

/** Flush timers + pending microtasks in one step. */
const tick = (ms = 0) => vi.advanceTimersByTimeAsync(ms);

describe('usePolling', () => {
	let originalHidden: boolean;

	beforeEach(() => {
		originalHidden = document.hidden;
		vi.useFakeTimers();
		Object.defineProperty(document, 'hidden', {
			value: false,
			writable: true,
			configurable: true
		});
	});

	afterEach(() => {
		Object.defineProperty(document, 'hidden', {
			value: originalHidden,
			writable: true,
			configurable: true
		});
		vi.useRealTimers();
		vi.restoreAllMocks();
	});

	// ── initialization ─────────────────────────────────────────────────────────

	describe('initialization', () => {
		it('throws on invalid baseInterval', () => {
			expect(() =>
				usePolling({ fn: vi.fn().mockResolvedValue(undefined), baseInterval: 0 })
			).toThrow('baseInterval must be positive');
			expect(() =>
				usePolling({ fn: vi.fn().mockResolvedValue(undefined), baseInterval: -100 })
			).toThrow('baseInterval must be positive');
		});

		it('throws on invalid maxInterval', () => {
			expect(() =>
				usePolling({ fn: vi.fn().mockResolvedValue(undefined), maxInterval: 0 })
			).toThrow('maxInterval must be positive');
			expect(() =>
				usePolling({ fn: vi.fn().mockResolvedValue(undefined), maxInterval: -100 })
			).toThrow('maxInterval must be positive');
		});

		it('throws when maxInterval < baseInterval', () => {
			expect(() =>
				usePolling({ fn: vi.fn().mockResolvedValue(undefined), baseInterval: 10_000, maxInterval: 5_000 })
			).toThrow('maxInterval must be >= baseInterval');
		});

		it('throws on invalid multiplier', () => {
			expect(() =>
				usePolling({ fn: vi.fn().mockResolvedValue(undefined), multiplier: 0 })
			).toThrow('multiplier must be positive');
			expect(() =>
				usePolling({ fn: vi.fn().mockResolvedValue(undefined), multiplier: -2 })
			).toThrow('multiplier must be positive');
		});

		it('throws on invalid maxConsecutiveNonBackoffErrors', () => {
			expect(() =>
				usePolling({ fn: vi.fn().mockResolvedValue(undefined), maxConsecutiveNonBackoffErrors: 0 })
			).toThrow('maxConsecutiveNonBackoffErrors must be positive');
			expect(() =>
				usePolling({ fn: vi.fn().mockResolvedValue(undefined), maxConsecutiveNonBackoffErrors: -5 })
			).toThrow('maxConsecutiveNonBackoffErrors must be positive');
		});

		it('starts with default state', () => {
			const handle = usePolling({ fn: vi.fn().mockResolvedValue(undefined) });

			expect(handle.isPolling).toBe(false);
			expect(handle.isPaused).toBe(false);
			expect(handle.isBackedOff).toBe(false);
			expect(handle.nextRetryIn).toBe(0);
		});

		it('uses default baseInterval for backoffInterval at rest', () => {
			const handle = usePolling({ fn: vi.fn().mockResolvedValue(undefined) });
			expect(handle.backoffInterval).toBe(5_000);
		});

		it('respects custom baseInterval', () => {
			const handle = usePolling({
				fn: vi.fn().mockResolvedValue(undefined),
				baseInterval: 1_000,
				maxInterval: 10_000,
				multiplier: 3
			});
			expect(handle.backoffInterval).toBe(1_000);
		});
	});

	// ── start / stop lifecycle ─────────────────────────────────────────────────

	describe('start / stop', () => {
		it('sets isPolling on start', () => {
			const handle = usePolling({ fn: vi.fn().mockResolvedValue(undefined) });
			handle.start();
			expect(handle.isPolling).toBe(true);
		});

		it('invokes fn immediately on start', async () => {
			const fn = vi.fn().mockResolvedValue(undefined);
			const handle = usePolling({ fn });
			handle.start();
			await tick();
			expect(fn).toHaveBeenCalledOnce();
		});

		it('clears isPolling on stop', () => {
			const handle = usePolling({ fn: vi.fn().mockResolvedValue(undefined) });
			handle.start();
			handle.stop();
			expect(handle.isPolling).toBe(false);
		});

		it('does not invoke fn after stop', async () => {
			const fn = vi.fn().mockResolvedValue(undefined);
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick();
			handle.stop();
			await tick(5_000);
			expect(fn).toHaveBeenCalledOnce();
		});

		it('start is idempotent', async () => {
			const fn = vi.fn().mockResolvedValue(undefined);
			const handle = usePolling({ fn });
			handle.start();
			handle.start();
			await tick();
			expect(fn).toHaveBeenCalledOnce();
		});

		it('destroy prevents restart', () => {
			const fn = vi.fn().mockResolvedValue(undefined);
			const handle = usePolling({ fn });
			handle.start();
			handle.destroy();
			expect(handle.isPolling).toBe(false);

			handle.start();
			expect(handle.isPolling).toBe(false);
			expect(fn).toHaveBeenCalledOnce();
		});

		it('allows restart after stop', async () => {
			const fn = vi.fn().mockResolvedValue(undefined);
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick();
			handle.stop();
			expect(handle.isPolling).toBe(false);

			handle.start();
			expect(handle.isPolling).toBe(true);
			await tick();
			expect(fn).toHaveBeenCalledTimes(2);
		});
	});

	// ── exponential backoff ────────────────────────────────────────────────────

	describe('exponential backoff', () => {
		it('marks isBackedOff after a 5xx error', async () => {
			const fn = vi.fn().mockRejectedValue(apiError(500));
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick();
			expect(handle.isBackedOff).toBe(true);
		});

		it('doubles the interval on each failure (default multiplier)', async () => {
			const fn = vi.fn().mockRejectedValue(apiError(500));
			const handle = usePolling({ fn, baseInterval: 1_000, maxInterval: 60_000, multiplier: 2 });
			handle.start();

			// Failure 1 → backoffCount = 1 → 1000 * 2^1 = 2000
			await tick();
			expect(handle.backoffInterval).toBe(2_000);

			// Failure 2 → backoffCount = 2 → 1000 * 2^2 = 4000
			await tick(2_000);
			expect(handle.backoffInterval).toBe(4_000);

			// Failure 3 → 8000
			await tick(4_000);
			expect(handle.backoffInterval).toBe(8_000);
		});

		it('caps backoff at maxInterval', async () => {
			const fn = vi.fn().mockRejectedValue(apiError(500));
			const handle = usePolling({ fn, baseInterval: 5_000, maxInterval: 20_000, multiplier: 2 });
			handle.start();

			// 5000*2^1=10k, *2^2=20k, *2^3=40k (capped at 20k)
			await tick();
			await tick(handle.backoffInterval);
			await tick(handle.backoffInterval);

			expect(handle.backoffInterval).toBe(20_000);
		});

		it('resets after a successful response', async () => {
			const fn = vi
				.fn()
				.mockRejectedValueOnce(apiError(503))
				.mockResolvedValue(undefined);
			const handle = usePolling({ fn, baseInterval: 1_000, maxInterval: 60_000, multiplier: 2 });
			handle.start();

			await tick(); // fails → backoff
			expect(handle.isBackedOff).toBe(true);

			await tick(handle.backoffInterval); // succeeds → reset

			expect(handle.isBackedOff).toBe(false);
			expect(handle.backoffInterval).toBe(1_000);
		});
	});

	// ── error classification ───────────────────────────────────────────────────

	describe('error classification', () => {
		it.each([500, 502, 503, 504])('backs off on %i server error', async (status) => {
			const fn = vi.fn().mockRejectedValue(apiError(status));
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick();
			expect(handle.isBackedOff).toBe(true);
		});

		it('backs off on generic network error', async () => {
			const fn = vi.fn().mockRejectedValue(new Error('Network Error'));
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick();
			expect(handle.isBackedOff).toBe(true);
		});

		it('stops polling on 401', async () => {
			const fn = vi.fn().mockRejectedValue(apiError(401));
			const handle = usePolling({ fn });
			handle.start();
			await tick();
			expect(handle.isPolling).toBe(false);
		});

		it('stops polling on 403', async () => {
			const fn = vi.fn().mockRejectedValue(apiError(403));
			const handle = usePolling({ fn });
			handle.start();
			await tick();
			expect(handle.isPolling).toBe(false);
		});

		it('does not back off on 4xx client errors (non-auth)', async () => {
			const fn = vi.fn().mockRejectedValue(apiError(404));
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick();
			expect(handle.isBackedOff).toBe(false);
			expect(handle.isPolling).toBe(true);
		});

		it('continues polling after a non-backoff error', async () => {
			const fn = vi
				.fn()
				.mockRejectedValueOnce(apiError(422))
				.mockResolvedValue(undefined);
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick();
			await tick(1_000);

			expect(fn).toHaveBeenCalledTimes(2);
			expect(handle.isBackedOff).toBe(false);
		});

		it('does not back off on TypeError (programming error)', async () => {
			const fn = vi.fn().mockRejectedValue(new TypeError('Cannot read properties of undefined'));
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick();
			expect(handle.isBackedOff).toBe(false);
			expect(handle.isPolling).toBe(true);
		});

		it('does not back off on AbortError', async () => {
			// Use DOMException if available (standard browser API), otherwise mock
			const abortErr =
				typeof DOMException !== 'undefined'
					? new DOMException('Aborted', 'AbortError')
					: Object.assign(new Error('Aborted'), { name: 'AbortError' });
			const fn = vi.fn().mockRejectedValue(abortErr);
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick();
			expect(handle.isBackedOff).toBe(false);
			expect(handle.isPolling).toBe(true);
		});

		it('stops after max consecutive non-backoff errors', async () => {
			const fn = vi.fn().mockRejectedValue(apiError(404));
			const handle = usePolling({ fn, baseInterval: 100, maxConsecutiveNonBackoffErrors: 3 });
			handle.start();

			// First call happens immediately on start (1st error)
			await tick(); // flush immediate call
			expect(fn).toHaveBeenCalledTimes(1);
			expect(handle.isPolling).toBe(true);

			// 2nd error (still polling)
			await tick(100);
			expect(handle.isPolling).toBe(true);

			// 3rd error (still polling, counter = 3 which equals max)
			await tick(100);
			expect(handle.isPolling).toBe(false);
			expect(fn).toHaveBeenCalledTimes(3);
		});

		it('resets consecutive non-backoff error count on success', async () => {
			const fn = vi
				.fn()
				.mockRejectedValueOnce(apiError(404))
				.mockRejectedValueOnce(apiError(404))
				.mockRejectedValueOnce(apiError(404))
				.mockResolvedValue(undefined);
			const handle = usePolling({ fn, baseInterval: 100, maxConsecutiveNonBackoffErrors: 5 });
			handle.start();

			// First call happens immediately on start (1st error)
			await tick();
			expect(fn).toHaveBeenCalledTimes(1);
			expect(handle.isPolling).toBe(true);

			// 2 more consecutive 404 errors (total 3 errors)
			await tick(100);
			await tick(100);
			expect(handle.isPolling).toBe(true);
			expect(fn).toHaveBeenCalledTimes(3);

			// Success resets counter and continues polling
			await tick(100);
			expect(handle.isPolling).toBe(true);
			expect(fn).toHaveBeenCalledTimes(4);
		});
	});

	// ── visibility API ─────────────────────────────────────────────────────────

	describe('visibility API', () => {
		it('sets isPaused when document becomes hidden', async () => {
			const handle = usePolling({ fn: vi.fn().mockResolvedValue(undefined), baseInterval: 1_000 });
			handle.start();
			await tick();

			Object.defineProperty(document, 'hidden', { value: true, configurable: true });
			document.dispatchEvent(new Event('visibilitychange'));

			expect(handle.isPaused).toBe(true);
		});

		it('does not poll while paused', async () => {
			const fn = vi.fn().mockResolvedValue(undefined);
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick();

			Object.defineProperty(document, 'hidden', { value: true, configurable: true });
			document.dispatchEvent(new Event('visibilitychange'));

			const callsBefore = fn.mock.calls.length;
			await tick(10_000);

			expect(fn.mock.calls.length).toBe(callsBefore);
		});

		it('resumes and immediately fetches when tab becomes visible', async () => {
			const fn = vi.fn().mockResolvedValue(undefined);
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick();

			Object.defineProperty(document, 'hidden', { value: true, configurable: true });
			document.dispatchEvent(new Event('visibilitychange'));

			const callsBefore = fn.mock.calls.length;

			Object.defineProperty(document, 'hidden', { value: false, configurable: true });
			document.dispatchEvent(new Event('visibilitychange'));
			await tick();

			expect(handle.isPaused).toBe(false);
			expect(fn.mock.calls.length).toBeGreaterThan(callsBefore);
		});

		it('clears isPaused state on stop', async () => {
			const handle = usePolling({ fn: vi.fn().mockResolvedValue(undefined), baseInterval: 1_000 });
			handle.start();
			await tick();

			Object.defineProperty(document, 'hidden', { value: true, configurable: true });
			document.dispatchEvent(new Event('visibilitychange'));
			expect(handle.isPaused).toBe(true);

			handle.stop();
			expect(handle.isPaused).toBe(false);
		});

		it('resumes respecting backoff interval', async () => {
			const fn = vi.fn().mockRejectedValue(apiError(500));
			const handle = usePolling({ fn, baseInterval: 1_000, maxInterval: 60_000, multiplier: 2 });
			handle.start();

			// First call fails, enters backoff
			await tick();
			expect(handle.isBackedOff).toBe(true);
			expect(handle.backoffInterval).toBe(2_000);

			// Hide tab
			Object.defineProperty(document, 'hidden', { value: true, configurable: true });
			document.dispatchEvent(new Event('visibilitychange'));
			expect(handle.isPaused).toBe(true);

			const callsBefore = fn.mock.calls.length;

			// Show tab before backoff expires
			await tick(500);
			Object.defineProperty(document, 'hidden', { value: false, configurable: true });
			document.dispatchEvent(new Event('visibilitychange'));
			expect(handle.isPaused).toBe(false);

			// Should not immediately retry - should wait remaining backoff time
			await tick(500);
			expect(fn.mock.calls.length).toBe(callsBefore);

			// After remaining backoff time, should retry
			await tick(1_000);
			expect(fn.mock.calls.length).toBeGreaterThan(callsBefore);
		});
	});

	// ── cleanup ────────────────────────────────────────────────────────────────

	describe('cleanup', () => {
		it('removes visibilitychange listener on stop', () => {
			const spy = vi.spyOn(document, 'removeEventListener');
			const handle = usePolling({ fn: vi.fn().mockResolvedValue(undefined) });
			handle.start();
			handle.stop();
			expect(spy).toHaveBeenCalledWith('visibilitychange', expect.any(Function));
			spy.mockRestore();
		});

		it('resets backoff state on stop', async () => {
			const fn = vi.fn().mockRejectedValue(apiError(500));
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick();
			expect(handle.isBackedOff).toBe(true);

			handle.stop();
			expect(handle.isBackedOff).toBe(false);
			expect(handle.nextRetryIn).toBe(0);
		});

		it('resets nextRetryIn after successful retry', async () => {
			const fn = vi
				.fn()
				.mockRejectedValueOnce(apiError(503))
				.mockResolvedValue(undefined);
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();

			await tick(); // fails → backoff
			expect(handle.nextRetryIn).toBeGreaterThan(0);

			await tick(handle.backoffInterval); // succeeds → reset
			expect(handle.nextRetryIn).toBe(0);
		});

		it('countdown continues during slow request', async () => {
			const fn = vi.fn().mockRejectedValueOnce(apiError(500));
			const handle = usePolling({ fn, baseInterval: 1_000 });
			handle.start();
			await tick(); // fails → backoff

			const initialNextRetryIn = handle.nextRetryIn;
			expect(initialNextRetryIn).toBeGreaterThan(0);

			// Countdown should tick down
			await tick(1_000);
			expect(handle.nextRetryIn).toBeLessThan(initialNextRetryIn);
		});
	});
});

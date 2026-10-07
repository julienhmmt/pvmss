import { describe, it, expect, vi, afterEach } from 'vitest';
import { onVisibleRefresh, POLL_REFRESH_MS } from './visibility-refresh';

/**
 * Overrides the read-only `document.visibilityState` with an own property
 * and fires the event, simulating a tab switch in happy-dom.
 */
function fireVisibilityChange(state: DocumentVisibilityState): void {
	Object.defineProperty(document, 'visibilityState', {
		configurable: true,
		get: () => state
	});
	document.dispatchEvent(new Event('visibilitychange'));
}

describe('onVisibleRefresh', () => {
	afterEach(() => {
		// Drop the own-property override so the prototype getter applies again.
		Reflect.deleteProperty(document, 'visibilityState');
	});

	it('calls the callback when the document becomes visible', () => {
		const callback = vi.fn();
		const unsubscribe = onVisibleRefresh(callback);

		fireVisibilityChange('visible');

		expect(callback).toHaveBeenCalledTimes(1);
		unsubscribe();
	});

	it('does not call the callback while the document stays hidden', () => {
		const callback = vi.fn();
		const unsubscribe = onVisibleRefresh(callback);

		fireVisibilityChange('hidden');

		expect(callback).not.toHaveBeenCalled();
		unsubscribe();
	});

	it('stops listening after unsubscribe', () => {
		const callback = vi.fn();
		const unsubscribe = onVisibleRefresh(callback);

		fireVisibilityChange('visible');
		unsubscribe();
		fireVisibilityChange('visible');

		expect(callback).toHaveBeenCalledTimes(1);
	});

	it('calls the callback on every poll tick while visible', () => {
		vi.useFakeTimers();
		const callback = vi.fn();
		const unsubscribe = onVisibleRefresh(callback);

		vi.advanceTimersByTime(POLL_REFRESH_MS * 2);

		expect(callback).toHaveBeenCalledTimes(2);
		unsubscribe();
		vi.useRealTimers();
	});

	it('skips poll ticks while hidden and stops after unsubscribe', () => {
		vi.useFakeTimers();
		const callback = vi.fn();
		const unsubscribe = onVisibleRefresh(callback);

		fireVisibilityChange('hidden');
		vi.advanceTimersByTime(POLL_REFRESH_MS);
		expect(callback).not.toHaveBeenCalled();

		fireVisibilityChange('visible');
		callback.mockClear();
		unsubscribe();
		vi.advanceTimersByTime(POLL_REFRESH_MS * 2);
		expect(callback).not.toHaveBeenCalled();
		vi.useRealTimers();
	});
});

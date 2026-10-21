import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ToastRegion } from './toast.svelte';

describe('ToastRegion', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});
	afterEach(() => {
		vi.useRealTimers();
	});

	it('keeps errors for 10s and success/info for 4s', () => {
		const region = new ToastRegion();
		region.success('saved');
		region.error('failed');

		vi.advanceTimersByTime(4000);
		expect(region.items.map((item) => item.message)).toEqual(['failed']);
		vi.advanceTimersByTime(6000);
		expect(region.items).toEqual([]);
	});

	it('drops success/info before an unread error when the queue is full', () => {
		const region = new ToastRegion();
		region.error('first error');
		for (let i = 0; i < 5; i += 1) region.info(`info ${i}`);
		region.error('second error');

		const messages = region.items.map((item) => item.message);
		expect(messages).toHaveLength(6);
		expect(messages).toContain('first error');
		expect(messages).toContain('second error');
		expect(messages).not.toContain('info 0');
	});

	it('pauses the countdown while held', () => {
		const region = new ToastRegion();
		const id = region.error('failed');
		region.hold(id);
		vi.advanceTimersByTime(20_000);
		expect(region.items).toHaveLength(1);

		region.release(id);
		vi.advanceTimersByTime(10_000);
		expect(region.items).toEqual([]);
	});
});

import { afterEach, describe, expect, it, vi } from 'vitest';
import { createHealthPoll, nodeTabParam, parseNodeTab, workloadStatusCounts } from './node-details';

describe('parseNodeTab', () => {
	it.each([
		[null, 'health'],
		['', 'health'],
		['health', 'health'],
		['workloads', 'workloads'],
		['hardware', 'hardware'],
		['bogus', 'health']
	] as const)('maps %s to %s', (raw, expected) => {
		expect(parseNodeTab(raw)).toBe(expected);
	});
});

describe('nodeTabParam', () => {
	it('omits the param for the default tab and keeps the others', () => {
		expect(nodeTabParam('health')).toBeNull();
		expect(nodeTabParam('workloads')).toBe('workloads');
		expect(nodeTabParam('hardware')).toBe('hardware');
	});
});

describe('workloadStatusCounts', () => {
	it('orders known statuses first, then the rest alphabetically', () => {
		expect(workloadStatusCounts(['stopped', 'running', 'running', 'paused', 'migrating', 'stopped'])).toEqual([
			['running', 2],
			['stopped', 2],
			['paused', 1],
			['migrating', 1]
		]);
	});

	it('handles an empty workload', () => {
		expect(workloadStatusCounts([])).toEqual([]);
	});
});

describe('createHealthPoll', () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	it('runs on the interval until stopped', () => {
		vi.useFakeTimers();
		const run = vi.fn();
		const poll = createHealthPoll(run, 1000);
		poll.start();
		vi.advanceTimersByTime(3500);
		expect(run).toHaveBeenCalledTimes(3);
		poll.stop();
		vi.advanceTimersByTime(3000);
		expect(run).toHaveBeenCalledTimes(3);
	});

	it('does not double-start', () => {
		vi.useFakeTimers();
		const run = vi.fn();
		const poll = createHealthPoll(run, 1000);
		poll.start();
		poll.start();
		vi.advanceTimersByTime(1000);
		expect(run).toHaveBeenCalledTimes(1);
	});

	it('skips ticks while the document is hidden', () => {
		vi.useFakeTimers();
		const run = vi.fn();
		const spy = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden');
		const poll = createHealthPoll(run, 1000);
		poll.start();
		vi.advanceTimersByTime(3000);
		expect(run).not.toHaveBeenCalled();
		spy.mockRestore();
		vi.advanceTimersByTime(1000);
		expect(run).toHaveBeenCalledTimes(1);
	});
});

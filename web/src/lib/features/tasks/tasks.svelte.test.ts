import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TaskTrayStore, POLL_INTERVAL_MS, TASK_BUDGET_MS } from './tasks.svelte';
import { m } from '$lib/paraglide/messages.js';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

function runningResponse(upid: string): Response {
	return jsonResponse(200, { upid, state: 'running', log: [] });
}

/** Returns a fresh Response per call — a Response body can only be read once. */
function always(response: () => Response): () => Promise<Response> {
	return () => Promise.resolve(response());
}

function trackSnapshot(tray: TaskTrayStore, upid: string): void {
	tray.track({ upid, kind: 'vm_snapshot_create', vmid: 100, name: 's1', cluster: 'default' });
}

// Ticket 04: the tray must stop following a task that never reaches a
// terminal state, and must not swallow repeated non-404 poll errors silently.
describe('TaskTrayStore timeout and error handling', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	it('abandons a task still running at its deadline with an informational toast, not an error toast', async () => {
		vi.stubGlobal('fetch', vi.fn(always(() => runningResponse('UPID:a'))));
		const tray = new TaskTrayStore();
		trackSnapshot(tray, 'UPID:a');

		// A snapshot task is followed for TASK_BUDGET_MS.vm_snapshot_create;
		// the first poll after the deadline must abandon it.
		await vi.advanceTimersByTimeAsync(TASK_BUDGET_MS.vm_snapshot_create + POLL_INTERVAL_MS);

		expect(tray.tasks).toHaveLength(0);
		expect(tray.toast?.kind).toBe('info');
		expect(tray.toast?.message).toBe(m['task.takingTooLong']());
		tray.destroy();
	});

	it('abandons a task after 5 consecutive non-404 errors, naming the cause', async () => {
		vi.stubGlobal('fetch', vi.fn(always(() => jsonResponse(502, { code: 'cluster_error', message: 'cluster unreachable' }))));
		const tray = new TaskTrayStore();
		trackSnapshot(tray, 'UPID:b');

		await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 5);

		expect(tray.tasks).toHaveLength(0);
		expect(tray.toast?.kind).toBe('error');
		expect(tray.toast?.message).toBe('cluster unreachable');
		tray.destroy();
	});

	it('finishes normally when a success interrupts the error streak (4 errors then ok)', async () => {
		const fetchMock = vi.fn()
			.mockResolvedValueOnce(jsonResponse(502, { code: 'cluster_error', message: 'boom 1' }))
			.mockResolvedValueOnce(jsonResponse(502, { code: 'cluster_error', message: 'boom 2' }))
			.mockResolvedValueOnce(jsonResponse(502, { code: 'cluster_error', message: 'boom 3' }))
			.mockResolvedValueOnce(jsonResponse(502, { code: 'cluster_error', message: 'boom 4' }))
			.mockResolvedValue(jsonResponse(200, { upid: 'UPID:c', state: 'ok', log: [] }));
		vi.stubGlobal('fetch', fetchMock);
		const tray = new TaskTrayStore();
		trackSnapshot(tray, 'UPID:c');

		await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 5);

		expect(tray.tasks).toHaveLength(0);
		expect(tray.toast?.kind).toBe('success');
		tray.destroy();
	});

	it('resets the consecutive-error counter on a running poll', async () => {
		const fetchMock = vi.fn()
			.mockResolvedValueOnce(jsonResponse(502, { code: 'cluster_error', message: 'boom' }))
			.mockResolvedValueOnce(jsonResponse(502, { code: 'cluster_error', message: 'boom' }))
			.mockResolvedValueOnce(jsonResponse(502, { code: 'cluster_error', message: 'boom' }))
			.mockResolvedValueOnce(jsonResponse(502, { code: 'cluster_error', message: 'boom' }))
			// A successful (running) poll resets the counter.
			.mockResolvedValueOnce(runningResponse('UPID:d'))
			.mockImplementation(always(() => jsonResponse(502, { code: 'cluster_error', message: 'boom again' })));
		vi.stubGlobal('fetch', fetchMock);
		const tray = new TaskTrayStore();
		trackSnapshot(tray, 'UPID:d');

		// 4 errors + running + 4 errors = 9 polls: still followed (the reset
		// means the streak restarted at 4, not continued to 9).
		await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 9);
		expect(tray.tasks).toHaveLength(1);
		expect(tray.toast).toBeNull();

		// The 5th consecutive error after the reset abandons the task.
		await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
		expect(tray.tasks).toHaveLength(0);
		expect(tray.toast?.kind).toBe('error');
		expect(tray.toast?.message).toBe('boom again');
		tray.destroy();
	});

	it('stops polling immediately on a 401 instead of waiting for 5 errors', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(401, { code: 'unauthorized', message: 'session expired' }));
		vi.stubGlobal('fetch', fetchMock);
		const tray = new TaskTrayStore();
		trackSnapshot(tray, 'UPID:e');

		await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
		expect(fetchMock).toHaveBeenCalledTimes(1);

		// The polling loop is stopped: no further polls, no toast, task kept.
		await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 20);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(tray.tasks).toHaveLength(1);
		expect(tray.toast).toBeNull();
		tray.destroy();
	});

	it('keeps the existing 404 behavior: the task finishes as no-longer-known', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(404, { code: 'not_found', message: 'no such task' })));
		const tray = new TaskTrayStore();
		trackSnapshot(tray, 'UPID:f');

		await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);

		expect(tray.tasks).toHaveLength(0);
		expect(tray.toast?.kind).toBe('error');
		expect(tray.toast?.message).toBe(m['task.taskNoLongerKnown']({ name: 's1' }));
		tray.destroy();
	});

	it('keeps the anti-overlap guard: a slow poll is never overlapped', async () => {
		let resolveFirst: (response: Response) => void = () => {};
		const firstCall = new Promise<Response>((resolve) => {
			resolveFirst = resolve;
		});
		const fetchMock = vi.fn()
			.mockImplementationOnce(() => firstCall)
			.mockResolvedValue(runningResponse('UPID:g'));
		vi.stubGlobal('fetch', fetchMock);
		const tray = new TaskTrayStore();
		trackSnapshot(tray, 'UPID:g');

		// The first interval tick starts a poll that never resolves...
		await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
		expect(fetchMock).toHaveBeenCalledTimes(1);

		// ...so subsequent ticks must not start an overlapping poll.
		await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 3);
		expect(fetchMock).toHaveBeenCalledTimes(1);

		// Once the in-flight poll resolves, the next tick polls again.
		resolveFirst(runningResponse('UPID:g'));
		await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
		expect(fetchMock).toHaveBeenCalledTimes(2);
		tray.destroy();
	});
});

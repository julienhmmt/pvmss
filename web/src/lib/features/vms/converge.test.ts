import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { convergeSingle, convergeBatch, ACTION_POLL_MS, ACTION_CONVERGE_TIMEOUT_MS } from './converge';

// Mock the API client so the convergence loop never hits the network.
vi.mock('$lib/shared/api/client', () => ({
	get: vi.fn(),
	post: vi.fn(),
}));

// Import after mock so the mock is in effect.
const { get, post } = await import('$lib/shared/api/client');

function mockGet(status: string): void {
	vi.mocked(get).mockResolvedValueOnce({ status, uptime: 0 } as never);
}

function mockPost(results: Array<{ cluster: string; vmid: number; status: string }>): void {
	vi.mocked(post).mockResolvedValueOnce(results as never);
}

function mockGetError(): void {
	vi.mocked(get).mockRejectedValueOnce(new Error('network error') as never);
}

function mockPostError(): void {
	vi.mocked(post).mockRejectedValueOnce(new Error('network error') as never);
}

describe('convergeSingle', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.mocked(get).mockReset();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('converges on the first tick when status matches', async () => {
		mockGet('running');
		const onTick = vi.fn();

		const promise = convergeSingle(
			{ cluster: 'default', vmid: 100 },
			'running',
			onTick,
		);

		await vi.waitFor(() => expect(promise).resolves.toBeUndefined());

		expect(onTick).toHaveBeenCalledWith('running');
		expect(get).toHaveBeenCalledTimes(1);
	});

	it('converges after several polls', async () => {
		// First two reads return 'stopped', third returns 'running'.
		mockGet('stopped');
		mockGet('stopped');
		mockGet('running');
		const onTick = vi.fn();

		const promise = convergeSingle(
			{ cluster: 'default', vmid: 100 },
			'running',
			onTick,
		);

		// Advance through the polls.
		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);
		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);
		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);

		await vi.waitFor(() => expect(promise).resolves.toBeUndefined());

		expect(onTick).toHaveBeenLastCalledWith('running');
		expect(get).toHaveBeenCalledTimes(3);
	});

	it('expires gracefully after the timeout, accepting the last reading', async () => {
		// Always returns 'stopped' — never converges to 'running'.
		vi.mocked(get).mockResolvedValue({ status: 'stopped', uptime: 0 } as never);
		const onTick = vi.fn();

		const promise = convergeSingle(
			{ cluster: 'default', vmid: 100 },
			'running',
			onTick,
		);

		// Advance past the timeout.
		await vi.advanceTimersByTimeAsync(ACTION_CONVERGE_TIMEOUT_MS + ACTION_POLL_MS);

		await vi.waitFor(() => expect(promise).resolves.toBeUndefined());

		// The loop ended without converging — no error thrown.
		expect(onTick).toHaveBeenLastCalledWith('stopped');
	});

	it('survives intermediate read errors without terminating', async () => {
		// First read errors, second succeeds with 'running'.
		mockGetError();
		mockGet('running');
		const onTick = vi.fn();

		const promise = convergeSingle(
			{ cluster: 'default', vmid: 100 },
			'running',
			onTick,
		);

		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);
		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);

		await vi.waitFor(() => expect(promise).resolves.toBeUndefined());

		// The error was swallowed, the loop continued, and converged on the second read.
		expect(onTick).toHaveBeenCalledWith('running');
		expect(get).toHaveBeenCalledTimes(2);
	});
});

describe('convergeBatch', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.mocked(post).mockReset();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('converges on the first tick when status matches', async () => {
		mockPost([{ cluster: 'default', vmid: 100, status: 'stopped' }]);
		const onTick = vi.fn();

		const promise = convergeBatch(
			{ cluster: 'default', vmid: 100 },
			'stopped',
			onTick,
		);

		await vi.waitFor(() => expect(promise).resolves.toBeUndefined());

		expect(onTick).toHaveBeenCalledWith('stopped');
		expect(post).toHaveBeenCalledTimes(1);
	});

	it('converges after several polls', async () => {
		mockPost([{ cluster: 'default', vmid: 100, status: 'running' }]);
		mockPost([{ cluster: 'default', vmid: 100, status: 'running' }]);
		mockPost([{ cluster: 'default', vmid: 100, status: 'stopped' }]);
		const onTick = vi.fn();

		const promise = convergeBatch(
			{ cluster: 'default', vmid: 100 },
			'stopped',
			onTick,
		);

		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);
		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);
		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);

		await vi.waitFor(() => expect(promise).resolves.toBeUndefined());

		expect(onTick).toHaveBeenLastCalledWith('stopped');
		expect(post).toHaveBeenCalledTimes(3);
	});

	it('survives intermediate read errors without terminating', async () => {
		mockPostError();
		mockPost([{ cluster: 'default', vmid: 100, status: 'running' }]);
		const onTick = vi.fn();

		const promise = convergeBatch(
			{ cluster: 'default', vmid: 100 },
			'running',
			onTick,
		);

		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);
		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);

		await vi.waitFor(() => expect(promise).resolves.toBeUndefined());

		expect(onTick).toHaveBeenCalledWith('running');
		expect(post).toHaveBeenCalledTimes(2);
	});

	it('expires gracefully after the timeout', async () => {
		vi.mocked(post).mockResolvedValue([
			{ cluster: 'default', vmid: 100, status: 'running' },
		] as never);
		const onTick = vi.fn();

		const promise = convergeBatch(
			{ cluster: 'default', vmid: 100 },
			'stopped',
			onTick,
		);

		await vi.advanceTimersByTimeAsync(ACTION_CONVERGE_TIMEOUT_MS + ACTION_POLL_MS);

		await vi.waitFor(() => expect(promise).resolves.toBeUndefined());

		expect(onTick).toHaveBeenLastCalledWith('running');
	});
});

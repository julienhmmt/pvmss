import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { VmDetailStore, type VmDetailEntity } from './detail.svelte';
import { ACTION_POLL_MS } from './converge';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

const baseEntity: VmDetailEntity = {
	cluster: 'default',
	vmid: 100,
	name: 'web-01',
	node: 'pve-node-01',
	pool: 'pool-alice',
	status: 'stopped',
	tags: ['pvmss'],
	cpuCores: 2,
	memoryTotal: 4294967296,
	diskTotal: 32212254720
};

function makeStore(entity: VmDetailEntity = { ...baseEntity }): VmDetailStore {
	const store = new VmDetailStore('default', 100);
	store.entity = entity;
	return store;
}

function stubFetchSequence(responses: Array<{ status: number; body: unknown }>): {
	fetchMock: ReturnType<typeof vi.fn>;
	calls: string[];
} {
	const calls: string[] = [];
	const queue = [...responses];
	const fetchMock = vi.fn().mockImplementation((url: string) => {
		calls.push(url);
		const res = queue.shift() ?? responses[responses.length - 1]!;
		return Promise.resolve(jsonResponse(res.status, res.body));
	});
	vi.stubGlobal('fetch', fetchMock);
	return { fetchMock, calls };
}

function stubFetchError(): ReturnType<typeof vi.fn> {
	const fetchMock = vi.fn().mockRejectedValue(new Error('action failed'));
	vi.stubGlobal('fetch', fetchMock);
	return fetchMock;
}

describe('VmDetailStore.action', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		vi.useRealTimers();
	});

	it('flips status optimistically and keeps it visible while convergence is pending', async () => {
		const { calls } = stubFetchSequence([
			{ status: 200, body: { status: 'ok' } },
			{ status: 200, body: { status: 'stopped', uptime: 0 } },
			{ status: 200, body: { status: 'running', uptime: 0 } }
		]);

		const store = makeStore();
		const promise = store.action('start');

		// The optimistic flip happens synchronously before the POST resolves.
		expect(store.entity?.status).toBe('running');
		expect(store.actionInFlight).toBe(true);

		// Let the POST resolve and the first live-status read happen.
		// The first read returns 'stopped' - the entity is patched but
		// the loop hasn't converged yet (target is 'running').
		await vi.waitFor(() => expect(store.entity?.status).toBe('stopped'));
		expect(store.actionInFlight).toBe(true);

		// Advance one poll interval for the second read.
		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);

		// Second read returns 'running' - convergence.
		await vi.waitFor(() => expect(promise).resolves.toBeUndefined());

		expect(store.entity?.status).toBe('running');
		expect(store.actionInFlight).toBe(false);

		// No load() call - only the action POST and status GET.
		expect(calls).not.toContain('/api/v1/vms/default/100');
	});

	it('reverts to the exact previous status when the POST fails', async () => {
		stubFetchError();

		const store = makeStore({ ...baseEntity, status: 'stopped' });
		await store.action('start');

		expect(store.entity?.status).toBe('stopped');
		expect(store.actionInFlight).toBe(false);
		expect(store.actionError).not.toBeNull();
	});

	it('does not call load() during convergence', async () => {
		const { calls } = stubFetchSequence([
			{ status: 200, body: { status: 'ok' } },
			{ status: 200, body: { status: 'running', uptime: 0 } }
		]);

		const store = makeStore();
		const promise = store.action('start');
		await vi.waitFor(() => expect(promise).resolves.toBeUndefined());

		// get is called only for the live-status poll, never for the full
		// entity reload (which would be `GET /api/v1/vms/default/100`).
		expect(calls).not.toContain('/api/v1/vms/default/100');

		// The entity's non-status fields are unchanged - only status was
		// patched by the convergence loop.
		expect(store.entity?.name).toBe('web-01');
		expect(store.entity?.cpuCores).toBe(2);
	});

	it('converges immediately when the first live-status read matches the target', async () => {
		stubFetchSequence([
			{ status: 200, body: { status: 'ok' } },
			{ status: 200, body: { status: 'stopped', uptime: 0 } }
		]);

		const store = makeStore({ ...baseEntity, status: 'running' });
		const promise = store.action('stop');
		await vi.waitFor(() => expect(promise).resolves.toBeUndefined());

		expect(store.entity?.status).toBe('stopped');
		expect(store.actionInFlight).toBe(false);
	});

	it('survives intermediate read errors during convergence', async () => {
		const calls: string[] = [];
		const queue: Array<Response | Error> = [
			jsonResponse(200, { status: 'ok' }),
			new Error('network error'),
			jsonResponse(200, { status: 'running', uptime: 0 })
		];
		const fetchMock = vi.fn().mockImplementation((url: string) => {
			calls.push(url);
			const item = queue.shift()!;
			if (item instanceof Error) return Promise.reject(item);
			return Promise.resolve(item);
		});
		vi.stubGlobal('fetch', fetchMock);

		const store = makeStore();
		const promise = store.action('start');

		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);
		await vi.advanceTimersByTimeAsync(ACTION_POLL_MS);
		await vi.waitFor(() => expect(promise).resolves.toBeUndefined());

		expect(store.entity?.status).toBe('running');
		expect(store.actionInFlight).toBe(false);
	});

	it('does nothing when actionInFlight is already true', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);

		const store = makeStore();
		store.actionInFlight = true;

		await store.action('start');

		expect(fetchMock).not.toHaveBeenCalled();
		expect(store.entity?.status).toBe('stopped');
		expect(store.actionInFlight).toBe(true);
	});

	it('does nothing when entity is null', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);

		const store = new VmDetailStore('default', 100);
		store.entity = null;

		await store.action('start');

		expect(fetchMock).not.toHaveBeenCalled();
		expect(store.actionInFlight).toBe(false);
	});
});

describe('VmDetailStore.bootFromCdrom', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		vi.useRealTimers();
	});

	it('boots directly when the VM is stopped and converges to running', async () => {
		const { calls } = stubFetchSequence([
			{ status: 200, body: { status: 'accepted' } },
			{ status: 200, body: { status: 'running', uptime: 0 } }
		]);

		const store = makeStore({ ...baseEntity, status: 'stopped' });
		const promise = store.bootFromCdrom();
		await vi.waitFor(() => expect(promise).resolves.toBe(true));

		expect(calls[0]).toBe('/api/v1/vms/default/100/boot-cdrom');
		expect(store.entity?.status).toBe('running');
		expect(store.bootCdromInFlight).toBe(false);
		expect(store.bootCdromError).toBeNull();
	});

	it('shuts a running VM down first, then boots from the CD-ROM', async () => {
		const { calls } = stubFetchSequence([
			// shutdown action
			{ status: 200, body: { status: 'ok' } },
			{ status: 200, body: { status: 'stopped', uptime: 0 } },
			// boot-cdrom
			{ status: 200, body: { status: 'accepted' } },
			{ status: 200, body: { status: 'running', uptime: 0 } }
		]);

		const store = makeStore({ ...baseEntity, status: 'running' });
		const promise = store.bootFromCdrom();
		await vi.waitFor(() => expect(promise).resolves.toBe(true));

		expect(calls[0]).toBe('/api/v1/vms/default/100/actions');
		expect(calls).toContain('/api/v1/vms/default/100/boot-cdrom');
		expect(store.entity?.status).toBe('running');
	});

	it('aborts when the shutdown does not converge to stopped', async () => {
		const { calls } = stubFetchSequence([
			// shutdown action accepted, but the live status stays running
			{ status: 200, body: { status: 'ok' } },
			{ status: 200, body: { status: 'running', uptime: 0 } }
		]);

		const store = makeStore({ ...baseEntity, status: 'running' });
		const promise = store.bootFromCdrom();
		// Convergence timeout is 30s of fake time.
		await vi.advanceTimersByTimeAsync(31_000);
		await vi.waitFor(() => expect(promise).resolves.toBe(false));

		expect(calls).not.toContain('/api/v1/vms/default/100/boot-cdrom');
		expect(store.bootCdromError).not.toBeNull();
	});

	it('reports the server error when boot-cdrom fails', async () => {
		stubFetchSequence([
			{ status: 409, body: { code: 'no_cdrom', message: 'no iso mounted' } }
		]);

		const store = makeStore({ ...baseEntity, status: 'stopped' });
		const promise = store.bootFromCdrom();
		await vi.waitFor(() => expect(promise).resolves.toBe(false));

		expect(store.bootCdromError).not.toBeNull();
		expect(store.bootCdromInFlight).toBe(false);
	});
});

describe('VmDetailStore.refreshIfStale', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		vi.useRealTimers();
	});

	it('records lastLoadedAt when load() completes', async () => {
		stubFetchSequence([
			{ status: 200, body: baseEntity },
			{ status: 200, body: {} }
		]);
		const store = new VmDetailStore('default', 100);

		expect(store.lastLoadedAt).toBe(0);
		await store.load();
		expect(store.lastLoadedAt).toBeGreaterThan(0);
	});

	it('reloads when the entity data is stale', async () => {
		vi.useFakeTimers();
		const { fetchMock, calls } = stubFetchSequence([
			{ status: 200, body: baseEntity },
			{ status: 200, body: {} }
		]);
		const store = new VmDetailStore('default', 100);

		// First load fetches the entity plus hardware-options.
		await store.load();
		expect(fetchMock).toHaveBeenCalledTimes(2);

		vi.setSystemTime(Date.now() + 31_000);
		await store.refreshIfStale();

		// hardwareOptions is cached, so the stale reload re-reads the entity only.
		expect(fetchMock).toHaveBeenCalledTimes(3);
		expect(calls.filter((call) => call === '/api/v1/vms/default/100')).toHaveLength(2);
	});

	it('does not reload while the entity data is still fresh', async () => {
		vi.useFakeTimers();
		const { fetchMock } = stubFetchSequence([
			{ status: 200, body: baseEntity },
			{ status: 200, body: {} }
		]);
		const store = new VmDetailStore('default', 100);

		await store.load();
		vi.setSystemTime(Date.now() + 29_000);
		await store.refreshIfStale();

		expect(fetchMock).toHaveBeenCalledTimes(2);
	});

	it('does not reload while a load is in flight', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
		const store = new VmDetailStore('default', 100);

		store.loading = true;
		await store.refreshIfStale(0);

		expect(fetchMock).not.toHaveBeenCalled();
	});

	it('does not reload while a power action is in flight', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
		const store = makeStore();

		store.actionInFlight = true;
		await store.refreshIfStale(0);

		expect(fetchMock).not.toHaveBeenCalled();
	});

	it('does not reload while a CD-ROM boot converge is in flight', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
		const store = makeStore();

		store.bootCdromInFlight = true;
		await store.refreshIfStale(0);

		expect(fetchMock).not.toHaveBeenCalled();
	});

	it('does not reload while a delete is in flight', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
		const store = makeStore();

		store.deleteInFlight = true;
		await store.refreshIfStale(0);

		expect(fetchMock).not.toHaveBeenCalled();
	});
});

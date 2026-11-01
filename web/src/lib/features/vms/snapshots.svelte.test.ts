import { afterEach, describe, expect, it, vi } from 'vitest';
import { TaskTrayStore } from '#lib/features/tasks/tasks.svelte.js';
import { VmSnapshotsStore } from './snapshots.svelte';
import { m } from '#lib/paraglide/messages.js';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

afterEach(() => vi.unstubAllGlobals());

describe('VmSnapshotsStore', () => {
	it('loads snapshots and the server-provided gabarit', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, {
			snapshots: [{ name: 'before-upgrade', description: 'pre-migration', createdAt: '2026-08-01T10:00:00Z', vmstate: false }],
			maxSnapshots: 5
		})));
		const store = new VmSnapshotsStore('default', 101, new TaskTrayStore());

		await store.load();

		expect(store.snapshots).toHaveLength(1);
		expect(store.snapshots[0]?.name).toBe('before-upgrade');
		expect(store.maxSnapshots).toBe(5);
	});

	it('registers accepted creation tasks with the shared tray', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(202, { cluster: 'default', vmid: 101, name: 'before-upgrade', upid: 'UPID:test' }));
		vi.stubGlobal('fetch', fetchMock);
		const tray = new TaskTrayStore();
		const store = new VmSnapshotsStore('default', 101, tray);

		const created = await store.create('before-upgrade', 'pre-migration', false);

		expect(created).toBe(true);
		expect(tray.tasks).toHaveLength(1);
		expect(tray.tasks[0]).toMatchObject({ upid: 'UPID:test', kind: 'vm_snapshot_create', vmid: 101, name: 'before-upgrade', cluster: 'default' });
		expect(typeof tray.tasks[0]?.deadline).toBe('number');
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
		expect(JSON.parse(init?.body as string)).toEqual({ name: 'before-upgrade', description: 'pre-migration', vmstate: false });
		tray.destroy();
	});

	it('loads the automatic schedule with the snapshot list', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, {
			snapshots: [],
			maxSnapshots: 5,
			schedule: { frequency: 'daily', retain: 3, lastRunAt: '' }
		})));
		const store = new VmSnapshotsStore('default', 101, new TaskTrayStore());

		await store.load();

		expect(store.schedule).toEqual({ frequency: 'daily', retain: 3, lastRunAt: '' });
	});

	it('saves the schedule with PUT and keeps the stored result', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { frequency: 'weekly', retain: 2, lastRunAt: '' }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new VmSnapshotsStore('default', 101, new TaskTrayStore());

		const saved = await store.saveSchedule('weekly', 2);

		expect(saved).toBe(true);
		expect(store.schedule?.frequency).toBe('weekly');
		const call = fetchMock.mock.calls[0] as [string, RequestInit];
		expect(call[0]).toBe('/api/v1/vms/default/101/snapshot-schedule');
		expect(call[1].method).toBe('PUT');
		expect(JSON.parse(call[1].body as string)).toEqual({ frequency: 'weekly', retain: 2 });
	});

	it('clears the schedule with DELETE', async () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new VmSnapshotsStore('default', 101, new TaskTrayStore());
		store.schedule = { frequency: 'daily', retain: 3, lastRunAt: '' };

		const cleared = await store.clearSchedule();

		expect(cleared).toBe(true);
		expect(store.schedule).toBeNull();
		expect((fetchMock.mock.calls[0] as [string, RequestInit])[1].method).toBe('DELETE');
	});

	it('reports a rejected schedule save without losing the stored one', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(400, { code: 'retain_exceeds_max_snapshots', message: 'retain 9 must be below the limit of 5' })));
		const store = new VmSnapshotsStore('default', 101, new TaskTrayStore());
		store.schedule = { frequency: 'daily', retain: 3, lastRunAt: '' };

		const saved = await store.saveSchedule('daily', 9);

		expect(saved).toBe(false);
		expect(store.scheduleError).toBe(m['error.retain_exceeds_max_snapshots']());
		expect(store.schedule?.retain).toBe(3);
	});

	it('surfaces server validation errors without registering a task', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(400, { code: 'max_snapshots_reached', message: 'maximum reached' })));
		const tray = new TaskTrayStore();
		const store = new VmSnapshotsStore('default', 101, tray);

		const created = await store.create('sixth', '', false);

		expect(created).toBe(false);
		expect(store.error).toBe(m['error.max_snapshots_reached']());
		expect(tray.tasks).toHaveLength(0);
		tray.destroy();
	});
});

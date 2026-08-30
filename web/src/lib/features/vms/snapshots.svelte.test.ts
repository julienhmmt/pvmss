import { afterEach, describe, expect, it, vi } from 'vitest';
import { TaskTrayStore } from '$lib/features/tasks/tasks.svelte';
import { VmSnapshotsStore } from './snapshots.svelte';

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
		expect(JSON.parse(fetchMock.mock.calls[0]?.[1]?.body as string)).toEqual({ name: 'before-upgrade', description: 'pre-migration', vmstate: false });
		tray.destroy();
	});

	it('surfaces server validation errors without registering a task', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(400, { code: 'max_snapshots_reached', message: 'maximum reached' })));
		const tray = new TaskTrayStore();
		const store = new VmSnapshotsStore('default', 101, tray);

		const created = await store.create('sixth', '', false);

		expect(created).toBe(false);
		expect(store.error).toBe('maximum reached');
		expect(tray.tasks).toHaveLength(0);
		tray.destroy();
	});
});

import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { setLocale } from '$lib/paraglide/runtime.js';
import { ActivityStore, activityMessage, mergeActivity } from './activity.svelte';

beforeAll(() => setLocale('en', { reload: false }));
afterEach(() => vi.unstubAllGlobals());

function entry(id: number, action: string, timestamp: string) {
	return { id, actor: 'alice', cluster: 'default', vmid: 0, action, timestamp };
}

describe('mergeActivity', () => {
	it('merges machines newest first and caps the result', () => {
		const rows = mergeActivity(
			[
				{ machine: { cluster: 'default', vmid: 100, name: 'web-01' }, items: [entry(1, 'start', '2026-09-01T10:00:00Z'), entry(3, 'stop', '2026-09-03T10:00:00Z')] },
				{ machine: { cluster: 'b', vmid: 101, name: 'db-01' }, items: [entry(2, 'reboot', '2026-09-02T10:00:00Z')] }
			],
			2
		);
		expect(rows.map((row) => `${row.machineName}:${row.action}`)).toEqual(['web-01:stop', 'db-01:reboot']);
		expect(rows[1]).toMatchObject({ cluster: 'b', vmid: 101, id: 'b:2' });
	});
});

describe('activityMessage', () => {
	it('names known actions and keeps unknown ones visible', () => {
		expect(activityMessage('shutdown')).toBe('Shut down');
		expect(activityMessage('mystery_op')).toContain('mystery_op');
	});
});

describe('ActivityStore', () => {
	it('keeps the timeline when one machine audit fails', async () => {
		const fetchMock = vi.fn(async (url: string) => {
			if (url.startsWith('/api/v1/vms?')) {
				return new Response(
					JSON.stringify({ items: [{ cluster: 'default', vmid: 100, name: 'web-01' }, { cluster: 'default', vmid: 101, name: 'db-01' }], total: 2, page: 1, pageSize: 20, availableNodes: [] }),
					{ status: 200 }
				);
			}
			if (url.includes('/100/audit')) {
				return new Response(JSON.stringify({ items: [entry(1, 'start', '2026-09-01T10:00:00Z')] }), { status: 200 });
			}
			return new Response(JSON.stringify({ code: 'forbidden', message: 'no' }), { status: 403 });
		});
		vi.stubGlobal('fetch', fetchMock);
		const store = new ActivityStore();
		await store.load();
		expect(store.error).toBeNull();
		expect(store.entries?.map((row) => row.machineName)).toEqual(['web-01']);
	});
});

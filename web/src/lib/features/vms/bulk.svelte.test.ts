import { describe, it, expect, vi, afterEach } from 'vitest';
import { VmBulkSelection, type BulkTarget, type BulkActionResult } from './bulk.svelte';
import type { VmListItem } from './list.svelte';

function target(cluster: string, vmid: number): BulkTarget {
	return { cluster, vmid };
}

function vmItem(cluster: string, vmid: number, name: string, status: VmListItem['status']): VmListItem {
	return {
		cluster,
		vmid,
		name,
		node: 'n1',
		status,
		pool: 'pool-alice',
		tags: ['pvmss'],
		cpuCores: 2,
		memoryTotal: 4096
	};
}

describe('VmBulkSelection', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('starts empty with no selection', () => {
		const sel = new VmBulkSelection();
		expect(sel.selectedCount).toBe(0);
		expect(sel.selectedTargets).toEqual([]);
		expect(sel.hasSelection).toBe(false);
	});

	it('toggle adds a target when not selected', () => {
		const sel = new VmBulkSelection();
		sel.toggle(target('default', 101));
		expect(sel.selectedCount).toBe(1);
		expect(sel.selectedTargets).toEqual([target('default', 101)]);
		expect(sel.hasSelection).toBe(true);
	});

	it('toggle removes a target when already selected', () => {
		const sel = new VmBulkSelection();
		sel.toggle(target('default', 101));
		sel.toggle(target('default', 101));
		expect(sel.selectedCount).toBe(0);
		expect(sel.selectedTargets).toEqual([]);
	});

	it('isSelected reports the composite identity correctly', () => {
		const sel = new VmBulkSelection();
		sel.toggle(target('default', 101));
		expect(sel.isSelected('default', 101)).toBe(true);
		expect(sel.isSelected('default', 102)).toBe(false);
		expect(sel.isSelected('secondary', 101)).toBe(false);
	});

	it('selection is keyed by (cluster, vmid), not bare vmid', () => {
		const sel = new VmBulkSelection();
		sel.toggle(target('default', 101));
		sel.toggle(target('secondary', 101));
		expect(sel.selectedCount).toBe(2);
		expect(sel.selectedTargets).toEqual([target('default', 101), target('secondary', 101)]);
	});

	it('selectPage adds all items on the current page, preserving existing selection', () => {
		const sel = new VmBulkSelection();
		sel.toggle(target('default', 100));
		sel.selectPage([
			vmItem('default', 101, 'web-02', 'stopped'),
			vmItem('default', 102, 'db-01', 'running')
		]);
		expect(sel.selectedCount).toBe(3);
		expect(sel.isSelected('default', 100)).toBe(true);
		expect(sel.isSelected('default', 101)).toBe(true);
		expect(sel.isSelected('default', 102)).toBe(true);
	});

	it('clearPage removes only items matching the given page', () => {
		const sel = new VmBulkSelection();
		sel.toggle(target('default', 100));
		sel.toggle(target('default', 101));
		sel.toggle(target('secondary', 200));
		sel.clearPage([
			vmItem('default', 100, 'web-01', 'running'),
			vmItem('default', 101, 'web-02', 'stopped')
		]);
		expect(sel.selectedCount).toBe(1);
		expect(sel.isSelected('secondary', 200)).toBe(true);
		expect(sel.isSelected('default', 100)).toBe(false);
	});

	it('clear empties the selection', () => {
		const sel = new VmBulkSelection();
		sel.toggle(target('default', 101));
		sel.clear();
		expect(sel.selectedCount).toBe(0);
		expect(sel.hasSelection).toBe(false);
	});

	it('selectPage then clearPage on the same page empties selection', () => {
		const sel = new VmBulkSelection();
		const items = [vmItem('default', 101, 'web-02', 'stopped')];
		sel.selectPage(items);
		expect(sel.selectedCount).toBe(1);
		sel.clearPage(items);
		expect(sel.selectedCount).toBe(0);
	});

	it('pageAllSelected reports true when every page item is selected', () => {
		const sel = new VmBulkSelection();
		const items = [
			vmItem('default', 101, 'web-02', 'stopped'),
			vmItem('default', 102, 'db-01', 'running')
		];
		expect(sel.pageAllSelected(items)).toBe(false);
		sel.selectPage(items);
		expect(sel.pageAllSelected(items)).toBe(true);
	});

	it('pageAllSelected is false for an empty page', () => {
		const sel = new VmBulkSelection();
		expect(sel.pageAllSelected([])).toBe(false);
	});

	it('submitBulkAction posts to /api/v1/vms/bulk-action and stores per-target results', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			new Response(JSON.stringify({ results: [
				{ cluster: 'default', vmid: 101, status: 'ok' },
				{ cluster: 'default', vmid: 103, status: 'error', message: 'not your VM' }
			] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
		);
		vi.stubGlobal('fetch', fetchMock);

		const sel = new VmBulkSelection();
		sel.toggle(target('default', 101));
		sel.toggle(target('default', 103));

		const result = await sel.submitBulkAction('start');
		expect(fetchMock).toHaveBeenCalledTimes(1);
		const call = fetchMock.mock.calls[0];
		expect(call).toBeDefined();
		const [url, init] = call as [string, RequestInit];
		expect(url).toBe('/api/v1/vms/bulk-action');
		expect(init?.method).toBe('POST');
		const body = JSON.parse(init?.body as string);
		expect(body.action).toBe('start');
		expect(body.targets).toEqual([{ cluster: 'default', vmid: 101 }, { cluster: 'default', vmid: 103 }]);

		expect(result.results).toHaveLength(2);
		expect(result.results[0]!.status).toBe('ok');
		expect(result.results[1]!.status).toBe('error');
		expect(result.results[1]!.message).toBe('not your VM');
		expect(sel.lastResult).not.toBeNull();
		expect(sel.submitting).toBe(false);
	});

	it('submitBulkAction throws ApiRequestError on 400 and does not store a result', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			new Response(JSON.stringify({ code: 'too_many_targets', message: 'targets exceeds the maximum of 100' }), { status: 400, headers: { 'Content-Type': 'application/json' } })
		);
		vi.stubGlobal('fetch', fetchMock);

		const sel = new VmBulkSelection();
		sel.toggle(target('default', 101));

		await expect(sel.submitBulkAction('start')).rejects.toThrow('targets exceeds the maximum of 100');
		expect(sel.lastResult).toBeNull();
		expect(sel.submitting).toBe(false);
	});

	it('clearResult clears the last result', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			new Response(JSON.stringify({ results: [{ cluster: 'default', vmid: 101, status: 'ok' }] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
		);
		vi.stubGlobal('fetch', fetchMock);

		const sel = new VmBulkSelection();
		sel.toggle(target('default', 101));
		await sel.submitBulkAction('start');
		expect(sel.lastResult).not.toBeNull();
		sel.clearResult();
		expect(sel.lastResult).toBeNull();
	});

	it('resultForTarget finds the entry matching a composite identity', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			new Response(JSON.stringify({ results: [
				{ cluster: 'default', vmid: 101, status: 'ok' },
				{ cluster: 'secondary', vmid: 101, status: 'error', message: 'vm not found' }
			] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
		);
		vi.stubGlobal('fetch', fetchMock);

		const sel = new VmBulkSelection();
		sel.toggle(target('default', 101));
		sel.toggle(target('secondary', 101));
		await sel.submitBulkAction('start');

		const r0 = sel.resultForTarget('default', 101);
		const r1 = sel.resultForTarget('secondary', 101);
		expect(r0?.status).toBe('ok');
		expect(r1?.status).toBe('error');
		expect(r1?.message).toBe('vm not found');
	});

	it('resultForTarget returns undefined when no result is stored', () => {
		const sel = new VmBulkSelection();
		expect(sel.resultForTarget('default', 101)).toBeUndefined();
	});

	it('resultSummary counts ok and error entries', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			new Response(JSON.stringify({ results: [
				{ cluster: 'default', vmid: 101, status: 'ok' },
				{ cluster: 'default', vmid: 102, status: 'ok' },
				{ cluster: 'default', vmid: 103, status: 'error', message: 'not your VM' }
			] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
		);
		vi.stubGlobal('fetch', fetchMock);

		const sel = new VmBulkSelection();
		sel.toggle(target('default', 101));
		sel.toggle(target('default', 102));
		sel.toggle(target('default', 103));
		await sel.submitBulkAction('start');

		const summary = sel.resultSummary;
		expect(summary.ok).toBe(2);
		expect(summary.error).toBe(1);
		expect(summary.total).toBe(3);
	});

	it('resultSummary returns zeros when no result is stored', () => {
		const sel = new VmBulkSelection();
		expect(sel.resultSummary).toEqual({ ok: 0, error: 0, total: 0 });
	});
});

describe('VmBulkSelection type exports', () => {
	it('BulkTarget shape', () => {
		const t: BulkTarget = { cluster: 'default', vmid: 101 };
		expect(t.cluster).toBe('default');
		expect(t.vmid).toBe(101);
	});

	it('BulkActionResult shape', () => {
		const r: BulkActionResult = {
			results: [
				{ cluster: 'default', vmid: 101, status: 'ok' },
				{ cluster: 'default', vmid: 103, status: 'error', message: 'not your VM' }
			]
		};
		expect(r.results).toHaveLength(2);
	});
});

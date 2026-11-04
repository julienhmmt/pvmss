import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import { TaskTrayStore } from '#lib/features/tasks/tasks.svelte.js';
import { m } from '#lib/paraglide/messages.js';
import { ToastRegion } from '#lib/shared/ui/toast.svelte.js';
import VmSnapshotsTab from './VmSnapshotsTab.svelte';

const toast = new ToastRegion();
const tray = new TaskTrayStore();

vi.mock('./detail.svelte', async (importOriginal) => ({
	...(await importOriginal<typeof import('./detail.svelte')>()),
	getVmDetailContext: () => ({ cluster: 'default', vmid: 101, entity: { status: 'running' }, load: vi.fn() })
}));
vi.mock('#lib/features/tasks/tasks.svelte.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/features/tasks/tasks.svelte.js')>()),
	getTaskTrayContext: () => tray
}));
vi.mock('#lib/shared/ui/toast.svelte.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/shared/ui/toast.svelte.js')>()),
	getToastContext: () => toast
}));

function json(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

/** Mounts the tab with the given stored schedule; `write` answers the PUT/DELETE. */
async function setup(schedule: unknown, write: Response) {
	vi.stubGlobal('fetch', vi.fn((_url: string, init?: RequestInit) =>
		Promise.resolve((init?.method ?? 'GET') === 'GET' ? json(200, { snapshots: [], maxSnapshots: 5, schedule }) : write)
	));
	const target = document.createElement('div');
	document.body.append(target);
	const tab = mount(VmSnapshotsTab, { target });
	await vi.waitFor(() => expect(fetch).toHaveBeenCalled());
	await vi.dynamicImportSettled();
	await new Promise((resolve) => setTimeout(resolve));
	flushSync();
	const pick = (id: string) => target.querySelector<HTMLInputElement>(`[data-testid="${id}"]`)!;
	const submit = async () => {
		target.querySelector('form')!.requestSubmit();
		await new Promise((resolve) => setTimeout(resolve));
		flushSync();
	};
	return { pick, submit, tab, target };
}

describe('VmSnapshotsTab schedule feedback', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		document.body.innerHTML = '';
	});

	it('toasts when a schedule is saved', async () => {
		const { pick, submit, tab } = await setup(null, json(200, { frequency: 'daily', retain: 3, lastRunAt: '' }));
		pick('schedule-daily').click();
		flushSync();
		await submit();
		expect(toast.items.at(-1)).toMatchObject({ variant: 'success', message: m['toast.snapshotScheduleSaved']() });
		await unmount(tab);
	});

	it('toasts when a schedule is turned off', async () => {
		const { pick, submit, tab } = await setup({ frequency: 'daily', retain: 3, lastRunAt: '' }, new Response(null, { status: 204 }));
		pick('schedule-off').click();
		flushSync();
		await submit();
		expect(toast.items.at(-1)).toMatchObject({ variant: 'success', message: m['toast.snapshotScheduleCleared']() });
		await unmount(tab);
	});

	it('shows the error without a success toast when the save fails', async () => {
		const before = toast.items.length;
		const { pick, submit, target, tab } = await setup(null, json(500, { error: 'boom' }));
		pick('schedule-daily').click();
		flushSync();
		await submit();
		expect(toast.items).toHaveLength(before);
		expect(target.querySelector('[data-testid="schedule-error"]')).not.toBeNull();
		await unmount(tab);
	});
});

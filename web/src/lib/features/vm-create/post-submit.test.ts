import { afterEach, describe, expect, it, vi } from 'vitest';
import { handleAccepted, type PostSubmitDeps } from './post-submit';
import { TaskTrayStore } from '#lib/features/tasks/tasks.svelte.js';
import { TaskOutcomeLedger } from '#lib/features/tasks/task-outcome-ledger.svelte.js';
import { ToastRegion } from '#lib/shared/ui/toast.svelte.js';
import type { VmCreateAccepted } from './create.svelte';


function accepted(): VmCreateAccepted {
	return { cluster: 'default', vmid: 101, name: 'web-01', node: 'pve', upid: 'UPID:pve:1' };
}

function deps(): PostSubmitDeps & { toast: ToastRegion } {
	return { tray: new TaskTrayStore(), toast: new ToastRegion(), outcomeLedger: new TaskOutcomeLedger() };
}

afterEach(() => vi.unstubAllGlobals());

describe('handleAccepted', () => {
	it.each(['running', 'stopped'] as const)('passes the expected %s status to task-completion listeners', async (expectedStatus) => {
		const d: PostSubmitDeps = deps();
		await handleAccepted({ ...accepted(), expectedStatus }, d);
		expect(d.tray.tasks[0]?.expectedStatus).toBe(expectedStatus);
		d.tray.destroy();
	});

	it('tracks the task and shows the queued toast', async () => {
		const d = deps();
		const trackSpy = vi.spyOn(d.tray, 'track');
		const infoSpy = vi.spyOn(d.toast, 'info');

		await handleAccepted(accepted(), d);

		expect(trackSpy).toHaveBeenCalledTimes(1);
		expect(infoSpy).toHaveBeenCalled();
	});
});

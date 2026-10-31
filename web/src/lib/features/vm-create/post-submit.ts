import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { m } from '#lib/paraglide/messages.js';
import type { TaskTrayStore } from '#lib/features/tasks/tasks.svelte.js';
import type { TaskOutcomeLedger } from '#lib/features/tasks/task-outcome-ledger.svelte.js';
import type { ToastRegion } from '#lib/shared/ui/toast.svelte.js';
import type { VmCreateAccepted } from './create.svelte';

/** Dependencies the post-submit helper needs from its caller. Grouped so
 *  the signature stays stable as new side-effects are added. */
export interface PostSubmitDeps {
	tray: TaskTrayStore;
	toast: ToastRegion;
	outcomeLedger: TaskOutcomeLedger;
}

/**
 * Shared post-submit handling for both creation wizards (SimpleWizard and
 * DetailedWizard → StepReview). Called after `form.submit()` returns a
 * non-null `VmCreateAccepted`.
 *
 * - Registers the task with the tray (polling → terminal toast).
 * - Surfaces the queued toast.
 * - Navigates back to the machine list.
 *
 * Extracting this prevents the two wizards from drifting - a previous
 * version had SimpleWizard record `partial` but StepReview silently skip it,
 * breaking the no-duplicate safety for the detailed path.
 */
export async function handleAccepted(accepted: VmCreateAccepted, deps: PostSubmitDeps): Promise<void> {
	deps.tray.track({
		upid: accepted.upid,
		kind: 'vm_create',
		vmid: accepted.vmid,
		name: accepted.name,
		cluster: accepted.cluster,
		...(accepted.expectedStatus !== undefined && { expectedStatus: accepted.expectedStatus })
	});
	deps.toast.info(m['toast.vmCreateQueued']());
	await goto(resolve('/vms'));
}

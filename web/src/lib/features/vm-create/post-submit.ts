import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { m } from '$lib/paraglide/messages.js';
import type { TaskTrayStore } from '$lib/features/tasks/tasks.svelte';
import type { TaskOutcomeLedger } from '$lib/features/tasks/task-outcome-ledger.svelte';
import type { ToastRegion } from '$lib/shared/ui/toast.svelte';
import type { VmCreateAccepted } from './create.svelte';
import type { DraftStore } from './draft.svelte';

/** Dependencies the post-submit helper needs from its caller. Grouped so
 *  the signature stays stable as new side-effects are added. */
export interface PostSubmitDeps {
	tray: TaskTrayStore;
	toast: ToastRegion;
	draft: DraftStore;
	outcomeLedger: TaskOutcomeLedger;
}

/**
 * Shared post-submit handling for both creation wizards (SimpleWizard and
 * DetailedWizard → StepReview). Called after `form.submit()` returns a
 * non-null `VmCreateAccepted`.
 *
 * - Clears the draft (the request is in flight).
 * - Registers the task with the tray (polling → terminal toast).
 * - Records a `partial` outcome in the session ledger when cloud-init push
 *   failed, so the list / detail can show the "do not create a duplicate"
 *   safety until the VM is reconfigured or deleted (issue 09).
 * - Surfaces the right toast (sticky error for cloud-init, info otherwise).
 * - Navigates back to the machine list.
 *
 * Extracting this prevents the two wizards from drifting - a previous
 * version had SimpleWizard record `partial` but StepReview silently skip it,
 * breaking the no-duplicate safety for the detailed path.
 */
export async function handleAccepted(accepted: VmCreateAccepted, deps: PostSubmitDeps): Promise<void> {
	deps.draft.clear();
	deps.tray.track({
		upid: accepted.upid,
		kind: 'vm_create',
		vmid: accepted.vmid,
		name: accepted.name,
		cluster: accepted.cluster
	});
	if (accepted.cloudInitPushError) {
		// The VM was created (task queued) but cloud-init could not be
		// applied - record a `partial` outcome so the list / detail can
		// show the "do not create a duplicate" safety until the VM is
		// reconfigured or deleted (issue 09). The tray will still poll
		// the vm_create task to completion; the ledger is the persistent
		// signal the tray cannot hold once the task ends.
		deps.outcomeLedger.record(accepted.cluster, accepted.vmid, 'partial');
		// Sticky (duration 0) error toast: cloudInitPushError used to be
		// dead data on this type, silently hiding the failure from the user.
		deps.toast.error(m['toast.vmCreateCloudInitWarning']({ error: accepted.cloudInitPushError }), 0);
	} else {
		deps.toast.info(m['toast.vmCreateQueued']());
	}
	// cloud-image-console issue 05: a cloud-image VM ships without a
	// password - SSH is the only way in until a console password is set
	// on the console page. Surface the hint after the standard toast so
	// the operator knows where to go.
	if (accepted.fromImage) {
		deps.toast.info(m['toast.vmCreateImageSshOnlyHint']());
	}
	await goto(resolve('/vms'));
}

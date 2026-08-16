<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getVmCreateContext } from '../create.svelte';
	import { getDraftContext } from '../draft.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';

	// Review step (V09): shows the exact request the server will receive —
	// there is no second, expert-only endpoint (FR-001) and no hidden fields.
	const form = getVmCreateContext();
	const tray = getTaskTrayContext();
	const toast = getToastContext();
	const draft = getDraftContext();

	const outgoing = $derived(form.buildRequest());

	async function submit(): Promise<void> {
		const accepted = await form.submit();
		if (accepted === null) {
			if (form.submitError) toast.error(m['toast.vmCreateFailed']({ error: form.submitError }));
			return;
		}
		draft.clear();
		tray.track({ upid: accepted.upid, kind: 'vm_create', vmid: accepted.vmid, name: accepted.name });
		toast.info(m['toast.vmCreateQueued']());
		await goto(resolve('/vms'));
	}
</script>

<div class="grid gap-4">
	<h2 class="text-sm font-medium">{m['vms.create.reviewHeading']()}</h2>
	<pre
		class="overflow-x-auto rounded-md border border-border bg-muted p-3 text-xs"
		data-testid="review-request">{JSON.stringify(outgoing, null, 2)}</pre>

	{#if form.submitError}<p role="alert" class="text-sm text-destructive">{form.submitError}</p>{/if}
	<button
		type="button"
		class="rounded-md bg-primary px-3 py-2 font-medium text-primary-foreground disabled:opacity-50"
		disabled={form.submitting}
		onclick={() => void submit()}
	>
		{form.submitting ? m['common.creating']() : m['vms.create.submit']()}
	</button>
</div>

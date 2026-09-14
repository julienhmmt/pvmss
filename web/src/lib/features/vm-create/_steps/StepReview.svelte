<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { getDraftContext } from '../draft.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getTaskOutcomeLedgerContext } from '$lib/features/tasks/task-outcome-ledger.svelte';
	import { handleAccepted } from '../post-submit';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';

	// Review step (V09): shows the exact request the server will receive - 
	// there is no second, expert-only endpoint (FR-001) and no hidden fields.
	const form = getVmCreateContext();
	const tray = getTaskTrayContext();
	const toast = getToastContext();
	const draft = getDraftContext();
	const outcomeLedger = getTaskOutcomeLedgerContext();

	const outgoing = $derived(form.buildRequest());

	// The review shows the chosen document's label - the raw request JSON
	// only carries the id.
	const cloudInitDocumentLabel = $derived(
		form.cloudInitTemplateId !== ''
			? (form.catalog?.cloudInitTemplates.find((t) => t.id === form.cloudInitTemplateId)?.label ?? form.cloudInitTemplateId)
			: form.cloudInitFileId !== ''
				? (form.myCloudInitFiles.find((f) => f.id === form.cloudInitFileId)?.label ?? form.cloudInitFileId)
				: null
	);

	// Image mode: block the submit until the image's disk floor and the
	// mandatory cloud-init are satisfied (mirrored by store.submit).
	const imageBlocker = $derived(form.imageModeBlocker());

	async function submit(): Promise<void> {
		const accepted = await form.submit();
		if (accepted === null) {
			if (form.submitError) toast.error(m['toast.vmCreateFailed']({ error: form.submitError }));
			return;
		}
		await handleAccepted(accepted, { tray, toast, draft, outcomeLedger });
	}
</script>

<div class="grid gap-4">
	<h2 class="text-sm font-medium">{m['vms.create.reviewHeading']()}</h2>
	<p class="text-sm text-muted-foreground">
		{m['vms.create.reviewCloudinitDocument']()}: {cloudInitDocumentLabel ?? m['vms.create.cloudinitNone']()}
	</p>
	<pre
		class="overflow-x-auto rounded-lg border border-border bg-muted p-4 font-mono text-xs"
		data-testid="review-request">{JSON.stringify(outgoing, null, 2)}</pre>

	{#if form.submitError}<Alert>{form.submitError}</Alert>{/if}
	{#if imageBlocker !== null}
		<p class="text-sm text-muted-foreground" data-testid="image-blocker">{imageBlocker}</p>
	{/if}
	<Button
		type="button"
		loading={form.submitting}
		disabled={imageBlocker !== null}
		onclick={() => void submit()}
	>
		{form.submitting ? m['common.creating']() : m['vms.create.submit']()}
	</Button>
</div>

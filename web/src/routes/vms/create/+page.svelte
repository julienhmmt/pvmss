<script lang="ts">
	import { onMount } from 'svelte';
	import { setVmCreateContext } from '$lib/features/vm-create/create.svelte';
	import { setDraftContext } from '$lib/features/vm-create/draft.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import SimpleWizard from '$lib/features/vm-create/SimpleWizard.svelte';
	import DetailedWizard from '$lib/features/vm-create/DetailedWizard.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const form = setVmCreateContext();
	const draft = setDraftContext();
	const toast = getToastContext();

	onMount(() => {
		void form.loadCatalog();

		const stored = draft.load();
		if (stored !== null) {
			form.applyDraft(stored.values);
			if (draft.consumeRestoreToast()) {
				toast.success(m['toast.draftRestored']({ savedAt: stored.savedAt }));
			}
		}
	});

	$effect(() => {
		draft.scheduleSave(form.snapshotValues());
	});
</script>

<svelte:head>
	<title>{m['vms.create.title']()}</title>
</svelte:head>

<section class="mx-auto w-full max-w-2xl px-4 py-8">
	<h1 class="mb-4 text-2xl font-semibold tracking-tight">{m['vms.create.heading']()}</h1>

	<div role="tablist" aria-label={m['vms.create.modeLabel']()} class="mb-6 flex gap-2">
		<button
			role="tab"
			aria-selected={form.mode === 'simple'}
			class="rounded-md px-3 py-1.5 text-sm font-medium {form.mode === 'simple'
				? 'bg-primary text-primary-foreground'
				: 'bg-muted text-muted-foreground'}"
			onclick={() => (form.mode = 'simple')}
		>
			{m['vms.create.simple']()}
		</button>
		<button
			role="tab"
			aria-selected={form.mode === 'detailed'}
			class="rounded-md px-3 py-1.5 text-sm font-medium {form.mode === 'detailed'
				? 'bg-primary text-primary-foreground'
				: 'bg-muted text-muted-foreground'}"
			onclick={() => (form.mode = 'detailed')}
		>
			{m['vms.create.detailed']()}
		</button>
	</div>

	{#if form.mode === 'simple'}
		<SimpleWizard />
	{:else}
		<DetailedWizard />
	{/if}
</section>

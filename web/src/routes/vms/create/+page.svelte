<script lang="ts">
	import { onMount } from 'svelte';
	import { setVmCreateContext } from '$lib/features/vm-create/create.svelte';
	import { setDraftContext } from '$lib/features/vm-create/draft.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import SimpleWizard from '$lib/features/vm-create/SimpleWizard.svelte';
	import DetailedWizard from '$lib/features/vm-create/DetailedWizard.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const form = setVmCreateContext();
	const draft = setDraftContext();
	const toast = getToastContext();
	const session = getSessionContext();

	onMount(() => {
		void form.loadClusters().then(() => form.loadCatalog());

		const stored = draft.load();
		if (stored !== null) {
			form.applyDraft(stored.values);
			if (draft.consumeRestoreToast()) {
				toast.success(m['toast.draftRestored']({ savedAt: stored.savedAt }));
			}
		}

		function beforeUnload(event: BeforeUnloadEvent): void {
			if (draft.hasDraft()) {
				event.preventDefault();
				event.returnValue = '';
			}
		}
		window.addEventListener('beforeunload', beforeUnload);
		return () => window.removeEventListener('beforeunload', beforeUnload);
	});

	$effect(() => {
		draft.scheduleSave(form.snapshotValues());
	});
</script>

<svelte:head>
	<title>{m['vms.create.title']()}</title>
</svelte:head>

{#if session.isAdmin}
	<section class="mx-auto w-full max-w-2xl px-4 py-8">
		<Alert>{m['vms.create.adminBlocked']()}</Alert>
	</section>
{:else}
<section class="mx-auto w-full max-w-2xl px-4 py-8">
	<div class="mb-4 flex items-center justify-between gap-4">
		<h1 class="text-2xl font-semibold tracking-tight">{m['vms.create.heading']()}</h1>
		<ClusterSelector options={form.clusterOptions} value={form.cluster} onChange={(value) => form.setCluster(value)} id="vm-create-cluster" />
		{#if draft.hasDraft()}
			<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground" title={m['vms.create.draftSaved']()}>
				<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-3.5 w-3.5" aria-hidden="true">
					<path d="M4 10l4 4 8-8" />
				</svg>
				{m['vms.create.draftSaved']()}
			</span>
		{/if}
	</div>

	<div role="tablist" aria-label={m['vms.create.modeLabel']()} class="mb-6 flex gap-2">
		<button
			role="tab"
			aria-selected={form.mode === 'simple'}
			class="inline-flex items-center rounded-lg px-3 py-1.5 text-sm font-medium transition-colors pv-focus {form.mode === 'simple'
				? 'bg-primary text-primary-foreground'
				: 'bg-muted text-muted-foreground hover:bg-muted/80'}"
			onclick={() => (form.mode = 'simple')}
		>
			{m['vms.create.simple']()}
		</button>
		<button
			role="tab"
			aria-selected={form.mode === 'detailed'}
			class="inline-flex items-center rounded-lg px-3 py-1.5 text-sm font-medium transition-colors pv-focus {form.mode === 'detailed'
				? 'bg-primary text-primary-foreground'
				: 'bg-muted text-muted-foreground hover:bg-muted/80'}"
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
{/if}

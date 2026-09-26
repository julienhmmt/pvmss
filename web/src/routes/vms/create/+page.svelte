<script lang="ts">
	import { onMount } from 'svelte';
	import { setVmCreateContext, type CreateMode } from '$lib/features/vm-create/create.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import SimpleWizard from '$lib/features/vm-create/SimpleWizard.svelte';
	import DetailedWizard from '$lib/features/vm-create/DetailedWizard.svelte';
	import ModeChooser from '$lib/features/vm-create/ModeChooser.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import ButtonLink from '$lib/shared/ui/ButtonLink.svelte';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';

	const form = setVmCreateContext();
	const session = getSessionContext();

	/** The mode gate: null until a card is picked on the chooser. */
	let chosen = $state<CreateMode | null>(null);

	onMount(() => {
		void form.loadClusters().then(() => {
			void form.loadCatalog();
			void form.loadExistingMachines();
		});
	});

	// Blocked states replace the form (DESIGN.md §6.2): nothing to offer
	// yet, the allowance is used up, or the catalog could not be loaded.
	const catalog = $derived(form.catalog);
	const unavailable = $derived(form.catalogError !== null && catalog === null);
	const quotaReached = $derived(
		catalog?.quota !== undefined && catalog.quota.allowed >= 0 && catalog.quota.used >= catalog.quota.allowed
	);
	const noCatalog = $derived(
		catalog !== null && catalog.profiles.length === 0 && catalog.templates.length === 0 && catalog.images.length === 0
	);

	/** Enters a mode's wizard. */
	function chooseMode(mode: CreateMode): void {
		form.mode = mode;
		chosen = mode;
	}

	/** Returns to the chooser, keeping every entered value. */
	function changeMode(): void {
		chosen = null;
	}
</script>

<svelte:head>
	<title>{m['vms.create.title']()}</title>
</svelte:head>

<section class="mx-auto w-full max-w-5xl">
	<PageHeader
		back={{ href: resolve('/vms'), label: m['vms.create.back']() }}
		eyebrow={m['vms.create.eyebrow']()}
		title={m['vms.create.heading']()}
		description={m['vms.create.calmDescription']()}
		focusTarget
		divider={false}
	>
		{#snippet actions()}
			<span class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
				<svg viewBox="0 0 24 24" class="h-3.5 w-3.5 text-success" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
					<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
					<polyline points="9 12 11 14 15 10" />
				</svg>
				{m['vms.create.teamApproved']()}
			</span>
		{/snippet}
	</PageHeader>

	{#if session.isAdmin}
		<Alert>{m['vms.create.adminBlocked']()}</Alert>
	{:else}
		{#if form.clusterOptions.length > 1}
			<div class="mb-6 flex flex-col gap-4 rounded-xl border border-border bg-card p-4 shadow-card">
				<ClusterSelector options={form.clusterOptions} value={form.cluster} onChange={(value) => form.setCluster(value)} id="vm-create-cluster" />
			</div>
		{/if}

		{#if unavailable}
			<div class="rounded-xl border border-border bg-card shadow-card">
				<EmptyState title={m['vms.create.blocked.unavailableTitle']()} description={m['vms.create.blocked.unavailableBody']()} tone="error" dataTestid="vm-create-unavailable">
					{#snippet actions()}
						<Button onclick={() => void form.loadCatalog()}>{m['vms.create.blocked.retry']()}</Button>
					{/snippet}
				</EmptyState>
			</div>
		{:else if quotaReached}
			<div class="rounded-xl border border-border bg-card shadow-card">
				<EmptyState title={m['vms.create.blocked.quotaTitle']()} description={m['vms.create.blocked.quotaBody']()} dataTestid="vm-create-quota-reached">
					{#snippet actions()}
						<ButtonLink href={resolve('/vms')} variant="secondary">{m['vms.create.back']()}</ButtonLink>
					{/snippet}
				</EmptyState>
			</div>
		{:else if noCatalog}
			<div class="rounded-xl border border-border bg-card shadow-card">
				<EmptyState title={m['vms.create.blocked.noCatalogTitle']()} description={m['vms.create.blocked.noCatalogBody']()} dataTestid="vm-create-no-catalog" />
			</div>
		{:else if chosen === null}
			<div class="mx-auto flex w-full max-w-2xl flex-col gap-4">
				<p class="text-sm text-muted-foreground">{m['vms.create.modePrompt']()}</p>
				<ModeChooser onSelect={chooseMode} />
			</div>
		{:else}
			<div class="mb-5 flex flex-wrap items-center justify-between gap-3">
				<button
					type="button"
					onclick={changeMode}
					data-testid="vm-create-change-mode"
					class="pv-focus inline-flex items-center gap-1.5 rounded-lg text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
				>
					<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4" aria-hidden="true">
						<path d="M12 5l-5 5 5 5" />
					</svg>
					{m['vms.create.changeMode']()}
				</button>
				<div class="flex items-center gap-3 text-sm text-muted-foreground">
					<span class="rounded-full bg-muted px-2.5 py-0.5 text-xs font-medium">
						{chosen === 'simple' ? m['vms.create.simple']() : m['vms.create.detailed']()}
					</span>
					<span>{form.clusterDisplayName()}</span>
				</div>
			</div>

			{#if chosen === 'simple'}
				<SimpleWizard />
			{:else}
				<DetailedWizard />
			{/if}
		{/if}
	{/if}
</section>

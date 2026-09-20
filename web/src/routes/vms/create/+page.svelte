<script lang="ts">
	import { onMount } from 'svelte';
	import { setVmCreateContext, type CreateMode } from '$lib/features/vm-create/create.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import SimpleWizard from '$lib/features/vm-create/SimpleWizard.svelte';
	import DetailedWizard from '$lib/features/vm-create/DetailedWizard.svelte';
	import ModeChooser from '$lib/features/vm-create/ModeChooser.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const form = setVmCreateContext();
	const session = getSessionContext();

	/** The mode gate: null until a card is picked on the chooser. */
	let chosen = $state<CreateMode | null>(null);

	onMount(() => {
		void form.loadClusters().then(() => form.loadCatalog());
		void form.loadMyCloudInitFiles();
	});

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

{#if session.isAdmin}
	<section class="mx-auto w-full max-w-2xl px-4 py-8">
		<Alert>{m['vms.create.adminBlocked']()}</Alert>
	</section>
{:else if chosen === null}
	<section class="mx-auto w-full max-w-4xl px-4 py-8">
		<div class="mx-auto flex w-full max-w-2xl flex-col gap-6">
			<div class="flex flex-col gap-1">
				<h1 class="text-2xl font-semibold tracking-tight">{m['vms.create.heading']()}</h1>
				<p class="text-sm text-muted-foreground">{m['vms.create.modePrompt']()}</p>
			</div>

			<div class="flex flex-col gap-4 rounded-xl border border-border bg-card p-4 shadow-card">
				<ClusterSelector
					options={form.clusterOptions}
					value={form.cluster}
					onChange={(value) => form.setCluster(value)}
					id="vm-create-cluster"
				/>
			</div>

			<ModeChooser onSelect={chooseMode} />
		</div>
	</section>
{:else}
	<section class="mx-auto w-full max-w-4xl px-4 py-8">
		<div class="mb-6 flex flex-col gap-3">
			<div class="flex flex-wrap items-center justify-between gap-3">
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
					<span>{form.clusterDisplayName()}</span>
				</div>
			</div>

			<div class="flex flex-wrap items-center gap-3">
				<h1 class="text-2xl font-semibold tracking-tight">{m['vms.create.heading']()}</h1>
				<span class="rounded-full bg-muted px-2.5 py-0.5 text-xs font-medium text-muted-foreground">
					{chosen === 'simple' ? m['vms.create.simple']() : m['vms.create.detailed']()}
				</span>
			</div>
		</div>

		{#if chosen === 'simple'}
			<SimpleWizard />
		{:else}
			<DetailedWizard />
		{/if}
	</section>
{/if}

<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { getVmDetailContext } from '../detail.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';

	const store = getVmDetailContext();

	let socketsDraft = $state('1');
	let coresDraft = $state('1');
	let memoryDraft = $state('1024');
	const selectedTags = new SvelteSet<string>();

	$effect(() => {
		const entity = store.entity;
		if (entity === null) return;
		socketsDraft = String(entity.sockets ?? 1);
		coresDraft = String(entity.cores ?? entity.cpuCores);
		memoryDraft = String(Math.round(entity.memoryTotal / 1024 / 1024));
		selectedTags.clear();
		for (const tag of entity.tags) selectedTags.add(tag);
	});

	const catalogTags = $derived(store.hardwareOptions?.tags ?? []);

	const hardwareWillRestart = $derived.by(() => {
		const entity = store.entity;
		if (entity === null || entity.status !== 'running') return false;
		const sockets = Number(socketsDraft);
		const cores = Number(coresDraft);
		const memoryMB = Number(memoryDraft);
		return (
			sockets !== (entity.sockets ?? 1) ||
			cores !== (entity.cores ?? entity.cpuCores) ||
			memoryMB !== Math.round(entity.memoryTotal / 1024 / 1024)
		);
	});

	function toggleTag(name: string): void {
		if (selectedTags.has(name)) {
			selectedTags.delete(name);
		} else {
			selectedTags.add(name);
		}
	}

	async function saveHardware(): Promise<void> {
		const sockets = Number(socketsDraft);
		const cores = Number(coresDraft);
		const memoryMB = Number(memoryDraft);
		if (![sockets, cores, memoryMB].every(Number.isInteger)) return;
		await store.updateHardware({
			sockets,
			cores,
			memoryMB,
			tags: [...selectedTags]
		});
	}
</script>

<section class="rounded-xl border border-border bg-card p-6 shadow-card" aria-labelledby="hardware-heading" data-testid="vm-hardware">
	<h2 id="hardware-heading" class="text-lg font-semibold">{m['vms.hardware.heading']()}</h2>
	<div class="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<label class="grid gap-1.5 text-sm font-medium">
			{m['vms.hardware.sockets']()}
			<input
				class="pv-input"
				type="number"
				min="1"
				bind:value={socketsDraft}
				data-testid="vm-hardware-sockets"
			/>
		</label>
		<label class="grid gap-1.5 text-sm font-medium">
			{m['vms.hardware.cores']()}
			<input
				class="pv-input"
				type="number"
				min="1"
				bind:value={coresDraft}
				data-testid="vm-hardware-cores"
			/>
		</label>
		<label class="grid gap-1.5 text-sm font-medium">
			{m['vms.hardware.memory']()}
			<input
				class="pv-input"
				type="number"
				min="1"
				bind:value={memoryDraft}
				data-testid="vm-hardware-memory"
			/>
		</label>
		<div class="grid gap-1.5 text-sm font-medium">
			{m['vms.hardware.tags']()}
			{#if catalogTags.length === 0}
				<p class="text-sm font-normal text-muted-foreground" data-testid="vm-hardware-tags-empty">
					{m['vms.create.tagsNoneAvailable']()}
				</p>
			{:else}
				<div class="flex flex-wrap gap-2" role="group" aria-label={m['vms.hardware.tags']()} data-testid="vm-hardware-tags">
					{#each catalogTags as tag (tag.name)}
						{@const isSelected = selectedTags.has(tag.name)}
						<button
							type="button"
							aria-pressed={isSelected}
							onclick={() => toggleTag(tag.name)}
							data-testid="vm-hardware-tag-{tag.name}"
							class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background {isSelected
								? 'border-transparent bg-primary text-primary-foreground'
								: 'border-border bg-muted text-muted-foreground hover:bg-muted/80'}"
						>
							<span class="h-2 w-2 rounded-full" style="background-color: {tag.color}" aria-hidden="true"></span>
							{tag.name}
						</button>
					{/each}
				</div>
				<p class="text-xs font-normal text-muted-foreground">{m['vms.hardware.tagsHint']()}</p>
			{/if}
		</div>
	</div>
	{#if hardwareWillRestart}
		<p class="mt-4 rounded-lg bg-warning-soft px-3 py-2 text-sm text-warning-soft-foreground" data-testid="vm-hardware-restart-notice">
			{m['vms.hardware.restartNotice']()}
		</p>
	{/if}
	<Button
		class="mt-5"
		loading={store.hardwareInFlight}
		onclick={() => saveHardware()}
		data-testid="vm-hardware-save"
	>
		{store.hardwareInFlight ? m['common.saving']() : m['vms.hardware.saveButton']()}
	</Button>
	<p class="sr-only" role="status" aria-live="polite">
		{store.hardwareInFlight
			? hardwareWillRestart
				? m['vms.hardware.applyingRestart']()
				: m['vms.hardware.applying']()
			: ''}
	</p>
	{#if store.writeError}
		<p class="mt-3 text-sm text-destructive" role="alert">{store.writeError}</p>
	{/if}
</section>

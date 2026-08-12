<script lang="ts">
	import { getVmDetailContext } from '../detail.svelte';

	const store = getVmDetailContext();

	let socketsDraft = $state('1');
	let coresDraft = $state('1');
	let memoryDraft = $state('1024');
	let tagsDraft = $state('');

	$effect(() => {
		const entity = store.entity;
		if (entity === null) return;
		socketsDraft = String(entity.sockets ?? 1);
		coresDraft = String(entity.cores ?? entity.cpuCores);
		memoryDraft = String(Math.round(entity.memoryTotal / 1024 / 1024));
		tagsDraft = entity.tags.join(', ');
	});

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

	async function saveHardware(): Promise<void> {
		const sockets = Number(socketsDraft);
		const cores = Number(coresDraft);
		const memoryMB = Number(memoryDraft);
		if (![sockets, cores, memoryMB].every(Number.isInteger)) return;
		await store.updateHardware({
			sockets,
			cores,
			memoryMB,
			tags: tagsDraft
				.split(',')
				.map((tag) => tag.trim())
				.filter(Boolean)
		});
	}
</script>

<section aria-labelledby="hardware-heading" data-testid="vm-hardware">
	<h2 id="hardware-heading" class="text-lg font-semibold">Hardware</h2>
	<div class="mt-4 grid gap-3 sm:grid-cols-4">
		<label class="grid gap-1 text-sm">
			Sockets
			<input
				class="rounded-md border border-border bg-background px-2 py-2"
				type="number"
				min="1"
				bind:value={socketsDraft}
				data-testid="vm-hardware-sockets"
			/>
		</label>
		<label class="grid gap-1 text-sm">
			Cores
			<input
				class="rounded-md border border-border bg-background px-2 py-2"
				type="number"
				min="1"
				bind:value={coresDraft}
				data-testid="vm-hardware-cores"
			/>
		</label>
		<label class="grid gap-1 text-sm">
			Memory (MB)
			<input
				class="rounded-md border border-border bg-background px-2 py-2"
				type="number"
				min="1"
				bind:value={memoryDraft}
				data-testid="vm-hardware-memory"
			/>
		</label>
		<label class="grid gap-1 text-sm">
			Tags
			<input
				class="rounded-md border border-border bg-background px-2 py-2"
				bind:value={tagsDraft}
				data-testid="vm-hardware-tags"
			/>
		</label>
	</div>
	{#if hardwareWillRestart}
		<p class="mt-3 text-sm text-warning" data-testid="vm-hardware-restart-notice">
			This VM is running — saving sockets, cores, or memory changes will stop and restart it.
		</p>
	{/if}
	<button
		type="button"
		class="mt-4 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
		disabled={store.hardwareInFlight}
		onclick={() => saveHardware()}
		data-testid="vm-hardware-save"
	>
		{store.hardwareInFlight ? 'Saving…' : 'Save hardware'}
	</button>
	<p class="sr-only" role="status" aria-live="polite">
		{store.hardwareInFlight
			? hardwareWillRestart
				? 'Applying hardware change — VM is restarting…'
				: 'Applying hardware change…'
			: ''}
	</p>
	{#if store.writeError}
		<p class="mt-3 text-sm text-destructive" role="alert">{store.writeError}</p>
	{/if}
</section>

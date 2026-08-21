<script lang="ts">
	import { getVmDetailContext } from '../detail.svelte';
	import { m } from '$lib/paraglide/messages.js';

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
		<label class="grid gap-1.5 text-sm font-medium">
			{m['vms.hardware.tags']()}
			<input
				class="pv-input"
				bind:value={tagsDraft}
				data-testid="vm-hardware-tags"
			/>
		</label>
	</div>
	{#if hardwareWillRestart}
		<p class="mt-4 rounded-lg bg-warning-soft px-3 py-2 text-sm text-warning-soft-foreground" data-testid="vm-hardware-restart-notice">
			{m['vms.hardware.restartNotice']()}
		</p>
	{/if}
	<button
		type="button"
		class="mt-5 rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
		disabled={store.hardwareInFlight}
		onclick={() => saveHardware()}
		data-testid="vm-hardware-save"
	>
		{store.hardwareInFlight ? m['common.saving']() : m['vms.hardware.saveButton']()}
	</button>
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

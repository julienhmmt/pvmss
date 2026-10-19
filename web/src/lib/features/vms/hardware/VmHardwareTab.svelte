<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { getVmDetailContext } from '../detail.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';

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

	function limitCeiling(configured: number | undefined, fallback: number): number {
		return configured !== undefined && configured > 0 ? configured : fallback;
	}

	const maxSockets = $derived(limitCeiling(store.hardwareOptions?.limits.maxSockets, 4));
	const maxCores = $derived(limitCeiling(store.hardwareOptions?.limits.maxCores, 32));
	const maxMemoryMB = $derived(limitCeiling(store.hardwareOptions?.limits.maxMemoryMB, 65536));

	const socketsNum = $derived(Number(socketsDraft));
	const coresNum = $derived(Number(coresDraft));
	const memoryNum = $derived(Number(memoryDraft));

	const socketsError = $derived(
		Number.isInteger(socketsNum) && socketsNum >= 1 && socketsNum <= maxSockets
			? null
			: m['vms.create.socketsOutOfRange']({ min: 1, max: maxSockets })
	);
	const coresError = $derived(
		Number.isInteger(coresNum) && coresNum >= 1 && coresNum <= maxCores
			? null
			: m['vms.create.vcpuOutOfRange']({ min: 1, max: maxCores })
	);
	const memoryError = $derived(
		Number.isInteger(memoryNum) && memoryNum >= 128 && memoryNum <= maxMemoryMB
			? null
			: m['vms.create.memoryOutOfRange']({ min: 128, max: maxMemoryMB })
	);

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
		if (socketsError !== null || coresError !== null || memoryError !== null) return;
		await store.updateHardware({
			sockets: socketsNum,
			cores: coresNum,
			memoryMB: memoryNum,
			tags: [...selectedTags]
		});
	}
</script>

<Card pad="lg" aria-labelledby="hardware-heading" data-testid="vm-hardware">
	<h2 id="hardware-heading" class="text-lg font-semibold">{m['vms.hardware.heading']()}</h2>
	<div class="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<FormField
			label={m['vms.hardware.sockets']()}
			hint={m['vms.create.socketsLimitHint']({ min: 1, max: maxSockets })}
			error={socketsError}
			required
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					type="number"
					min={1}
					max={maxSockets}
					bind:value={socketsDraft}
					required
					data-testid="vm-hardware-sockets"
				/>
			{/snippet}
		</FormField>
		<FormField
			label={m['vms.hardware.cores']()}
			hint={m['vms.create.vcpuLimitHint']({ min: 1, max: maxCores })}
			error={coresError}
			required
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					type="number"
					min={1}
					max={maxCores}
					bind:value={coresDraft}
					required
					data-testid="vm-hardware-cores"
				/>
			{/snippet}
		</FormField>
		<FormField
			label={m['vms.hardware.memory']()}
			hint={m['vms.create.memoryLimitHint']({ min: 128, max: maxMemoryMB })}
			error={memoryError}
			required
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					type="number"
					min={128}
					max={maxMemoryMB}
					bind:value={memoryDraft}
					required
					data-testid="vm-hardware-memory"
				/>
			{/snippet}
		</FormField>
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
							class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-sm font-medium transition-colors pv-focus {isSelected
								? 'border-transparent bg-primary-solid text-primary-foreground'
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
		<Alert tone="warning" role="status" class="mt-4" data-testid="vm-hardware-restart-notice">
			{m['vms.hardware.restartNotice']()}
		</Alert>
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
		<Alert class="mt-3">{store.writeError}</Alert>
	{/if}
</Card>

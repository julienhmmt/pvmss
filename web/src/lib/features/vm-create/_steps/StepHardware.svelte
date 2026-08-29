<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Checkbox from '$lib/shared/ui/Checkbox.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';

	// Hardware step: vCPU and memory. Client-side bounds are a convenience
	// only — the server re-checks against the same ceiling (constitution VI).
	const form = getVmCreateContext();

	const maxSockets = $derived(form.catalog?.gabarit?.maxSockets ?? 4);
	const maxCores = $derived(form.catalog?.gabarit?.maxCores ?? 32);
	const maxMemoryMB = $derived(form.catalog?.gabarit?.maxMemoryMB ?? 65536);

	const socketsError = $derived(
		Number.isInteger(form.sockets) && form.sockets >= 1 && form.sockets <= maxSockets
			? null
			: m['vms.create.socketsOutOfRange']({ min: 1, max: maxSockets })
	);
	const cpuError = $derived(
		Number.isInteger(form.cpuCores) && form.cpuCores >= 1 && form.cpuCores <= maxCores
			? null
			: m['vms.create.vcpuOutOfRange']({ min: 1, max: maxCores })
	);
	const memoryError = $derived(
		Number.isInteger(form.memoryMB) && form.memoryMB >= 128 && form.memoryMB <= maxMemoryMB
			? null
			: m['vms.create.memoryOutOfRange']({ min: 128, max: maxMemoryMB })
	);

	const quota = $derived(form.catalog?.quota);
	const capacity = $derived(form.node !== '' ? form.nodeCapacity(form.node) : undefined);
</script>

<div class="grid gap-4">
	{#if quota || capacity}
		<div class="grid gap-1 rounded-lg border border-border bg-muted/40 p-3 text-sm">
			<p class="font-medium text-foreground">{m['vms.create.yourCapacity']()}</p>
			{#if quota}
				<p class="text-muted-foreground">
					{quota.allowed < 0 ? m['vms.create.quotaUnlimited']() : m['vms.create.quotaUsage']({ used: quota.used, allowed: quota.allowed })}
				</p>
			{/if}
			{#if capacity}
				{#if capacity.maxVCPUs > 0}
					<p class="text-muted-foreground">
						{m['vms.create.nodeCapacityVcpu']({ used: capacity.usedVCPUs, max: capacity.maxVCPUs, node: capacity.node })}
					</p>
				{/if}
				{#if capacity.maxRAMGB > 0}
					<p class="text-muted-foreground">
						{m['vms.create.nodeCapacityRam']({ used: capacity.usedRAMGB, max: capacity.maxRAMGB, node: capacity.node })}
					</p>
				{/if}
			{/if}
		</div>
	{/if}

	{#if form.mode === 'detailed'}
		<FormField
			label={m['vms.create.sockets']()}
			hint={m['vms.create.socketsLimitHint']({ min: 1, max: maxSockets })}
			error={socketsError}
			required
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} type="number" min={1} max={maxSockets} bind:value={form.sockets} required />
			{/snippet}
		</FormField>
	{/if}

	<FormField label={m['vms.create.vcpuCores']()} hint={m['vms.create.vcpuLimitHint']({ min: 1, max: maxCores })} error={cpuError} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} type="number" min={1} max={maxCores} bind:value={form.cpuCores} required />
		{/snippet}
	</FormField>

	<FormField
		label={m['vms.create.memory']()}
		hint={m['vms.create.memoryLimitHint']({ min: 128, max: maxMemoryMB })}
		error={memoryError}
		required
	>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} type="number" min={128} max={maxMemoryMB} step={128} bind:value={form.memoryMB} required />
		{/snippet}
	</FormField>

	{#if form.mode === 'detailed'}
		<div class="grid gap-2 rounded-lg border border-border p-4">
			<p class="text-sm font-medium text-foreground">{m['vms.create.firmwareGroup']()}</p>
			<Checkbox
				label={m['vms.create.uefi']()}
				hint={m['vms.create.uefiHint']()}
				checked={form.uefi}
				onToggle={(checked) => {
					form.uefi = checked;
					if (!checked) form.tpm = false;
				}}
			/>
			<Checkbox
				label={m['vms.create.tpm']()}
				hint={m['vms.create.tpmHint']()}
				checked={form.tpm}
				onToggle={(checked) => { form.tpm = checked; }}
				disabled={!form.uefi}
			/>
		</div>
	{/if}
</div>

<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';

	// Hardware step: vCPU and memory. Client-side bounds are a convenience
	// only — the server re-checks against the same ceiling (constitution VI).
	const form = getVmCreateContext();
</script>

<div class="grid gap-4">
	<FormField label={m['vms.create.vcpuCores']()} hint={m['vms.create.vcpuRange']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} type="number" min={1} max={32} bind:value={form.cpuCores} required />
		{/snippet}
	</FormField>

	<FormField label={m['vms.create.memory']()} hint={m['vms.create.memoryRange']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} type="number" min={128} max={65536} step={128} bind:value={form.memoryMB} required />
		{/snippet}
	</FormField>
</div>

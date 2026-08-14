<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';

	// Network step: one initial NIC (multi-NIC is T07) — an approved bridge
	// plus a device model.
	const form = getVmCreateContext();
	const inputClass = 'rounded-md border border-input bg-background px-3 py-2';
</script>

<div class="grid gap-4">
	<label class="grid gap-1 text-sm font-medium">
		{m['vms.create.bridge']()}
		<select class={inputClass} bind:value={form.bridge} required>
			<option value="" disabled>{m['vms.create.chooseBridge']()}</option>
			{#each form.catalog?.bridges ?? [] as bridge (bridge)}
				<option value={bridge}>{bridge}</option>
			{/each}
		</select>
	</label>

	<label class="grid gap-1 text-sm font-medium">
		{m['vms.create.model']()}
		<select class={inputClass} bind:value={form.networkModel}>
			<option value="virtio">virtio</option>
			<option value="e1000">e1000</option>
			<option value="rtl8139">rtl8139</option>
		</select>
	</label>
</div>

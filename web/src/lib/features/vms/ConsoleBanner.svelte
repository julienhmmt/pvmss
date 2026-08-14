<script lang="ts">
	import { resolve } from '$app/paths';
	import { getVmDetailContext } from '$lib/features/vms/detail.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getVmDetailContext();
</script>

{#if store.entity?.status === 'running'}
	<a
		href={resolve('/vms/[cluster]/[vmid]/console', { cluster: store.cluster, vmid: String(store.vmid) })}
		class="inline-flex items-center rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90"
		data-testid="vm-console-open"
	>
		{m['vms.console.open']()}
	</a>
{/if}

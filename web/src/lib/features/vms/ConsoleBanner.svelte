<script lang="ts">
	import { resolve } from '$app/paths';
	import { getVmDetailContext } from './detail.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getVmDetailContext();
</script>

{#if store.entity?.status === 'running'}
	<a
		href={resolve('/vms/[cluster]/[vmid]/console', { cluster: store.cluster, vmid: String(store.vmid) })}
		class="inline-flex items-center rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
		data-testid="vm-console-open"
	>
		{m['vms.console.open']()}
	</a>
{/if}

<script lang="ts">
	import { resolve } from '$app/paths';
	import { getVmDetailContext } from './detail.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import ButtonLink from '$lib/shared/ui/ButtonLink.svelte';

	const store = getVmDetailContext();
</script>

{#if store.entity?.status === 'running'}
	<ButtonLink
		href={resolve('/vms/[cluster]/[vmid]/console', { cluster: store.cluster, vmid: String(store.vmid) })}
		data-testid="vm-console-open"
	>
		{m['vms.console.open']()}
	</ButtonLink>
{/if}

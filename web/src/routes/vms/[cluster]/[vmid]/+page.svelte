<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setVmDetailContext } from '$lib/features/vms/detail.svelte';
	import { onVisibleRefresh } from '$lib/shared/visibility-refresh';
	import VmDetail from '$lib/features/vms/VmDetail.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const cluster = page.params.cluster ?? '';
	const vmid = Number(page.params.vmid);

	const store = setVmDetailContext(cluster, vmid);

	$effect(() => {
		if (store.deleted) {
			void goto(resolve('/vms'));
		}
	});

	onMount(() => {
		// The projection can be ~30s stale (VM just stopped from the console):
		// overlay the live status once loaded.
		void store.load().then(() => store.refreshLiveStatus());
		return onVisibleRefresh(() => void store.refreshIfStale());
	});
</script>

<svelte:head>
	<title>{m['vms.detail.title']({ name: store.entity?.name || `VM ${vmid}` })}</title>
</svelte:head>

<section class="mx-auto w-full max-w-reading">
	<VmDetail />
</section>

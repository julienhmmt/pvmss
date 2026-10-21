<script lang="ts">
	/**
	 * Admin view of one VM - the same detail surface the owner gets, reached
	 * from the admin machine collection. The server's vm.Resolve() lets an
	 * admin through regardless of pool, so no separate endpoint is involved.
	 */
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setVmDetailContext } from '#lib/features/vms/detail.svelte.js';
	import { onVisibleRefresh } from '#lib/shared/visibility-refresh.js';
	import VmDetail from '#lib/features/vms/VmDetail.svelte';
	import { m } from '#lib/paraglide/messages.js';

	const cluster = page.params.cluster ?? '';
	const vmid = Number(page.params.vmid);

	const store = setVmDetailContext(cluster, vmid);

	$effect(() => {
		if (store.deleted) {
			void goto(resolve('/admin/vms'));
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

<section class="mx-auto w-full max-w-6xl">
	<VmDetail backHref={resolve('/admin/vms')} backLabel={m['admin.vms.back']()} />
</section>

<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setVmDetailContext } from '$lib/features/vms/detail.svelte';
	import VmDetail from '$lib/features/vms/VmDetail.svelte';
	import Breadcrumb from '$lib/shared/ui/Breadcrumb.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const cluster = page.params.cluster ?? 'default';
	const vmid = Number(page.params.vmid);

	const store = setVmDetailContext(cluster, vmid);

	$effect(() => {
		if (store.deleted) {
			void goto(resolve('/vms'));
		}
	});

	onMount(() => {
		void store.load();
	});
</script>

<svelte:head>
	<title>{m['vms.detail.title']({ vmid: String(vmid) })}</title>
</svelte:head>

<section class="mx-auto w-full max-w-5xl px-4 py-8">
	<Breadcrumb items={[{ label: m['vms.list.heading'](), href: resolve('/vms') }, { label: m['vms.detail.breadcrumb']({ vmid: String(vmid) }) }]} />
	<VmDetail />
</section>

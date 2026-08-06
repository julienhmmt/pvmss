<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { page } from '$app/state';
	import { setVmDetailContext } from '$lib/features/vms/detail.svelte';
	import VmDetail from '$lib/features/vms/VmDetail.svelte';

	const cluster = page.params.cluster ?? 'default';
	const vmid = Number(page.params.vmid);

	const store = setVmDetailContext(cluster, vmid);

	// Navigate back to the list after a successful delete.
	$effect(() => {
		if (store.deleted) {
			void goto(`${base}/vms`);
		}
	});

	onMount(() => {
		void store.load();
	});
</script>

<svelte:head>
	<title>VM {vmid} — PVMSS</title>
</svelte:head>

<section class="mx-auto w-full max-w-4xl px-4 py-8">
	<a
		href={`${base}/vms`}
		class="mb-4 inline-flex items-center text-sm text-muted-foreground hover:text-foreground"
	>
		← Back to VMs
	</a>
	<VmDetail />
</section>

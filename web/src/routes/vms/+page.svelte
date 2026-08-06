<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { page } from '$app/state';
	import { setVmListContext } from '$lib/features/vms/list.svelte';
	import VmList from '$lib/features/vms/VmList.svelte';

	// Wiring only: the list state, URL sync, and rendering all live in
	// $lib/features/vms (FR-010) — this page just picks the scope.
	const vmListStore = setVmListContext({
		scope: 'mine',
		initialQuery: page.url.search,
		navigate: (queryString: string) => {
			void goto(`${base}/vms${queryString === '' ? '' : `?${queryString}`}`, {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		}
	});

	onMount(() => {
		void vmListStore.load();
	});
</script>

<svelte:head>
	<title>My VMs — PVMSS</title>
</svelte:head>

<section class="mx-auto w-full max-w-5xl px-4 py-8">
	<h1 class="mb-4 text-2xl font-semibold tracking-tight">My VMs</h1>

	{#if vmListStore.loading && vmListStore.result === null}
		<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
	{:else}
		<VmList />
	{/if}
</section>

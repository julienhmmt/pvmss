<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setVmListContext } from '$lib/features/vms/list.svelte';
	import { setVmBulkContext } from '$lib/features/vms/bulk.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import VmList from '$lib/features/vms/VmList.svelte';
	import VmBulkActionBar from '$lib/features/vms/VmBulkActionBar.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';

	// Wiring only: the list state, URL sync, and rendering all live in
	// $lib/features/vms (FR-010) — this page just picks the scope.
	let clusterOptions = $state<ClusterOption[]>([]);

	const vmListStore = setVmListContext({
		scope: 'mine',
		initialQuery: page.url.search,
		navigate: (queryString: string) => {
			// resolve() only accepts route literals, so the query string is
			// appended after the typed route resolution — no `as '/vms'` cast.
			const base = resolve('/vms');
			const target = queryString === '' ? base : `${base}?${queryString}`;
			void goto(target, {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		}
	});
	const vmBulk = setVmBulkContext();

	let offTaskOk: (() => void) | null = null;

	async function loadPage(): Promise<void> {
		try {
			clusterOptions = await fetchClusterOptions();
		} catch {
			clusterOptions = [];
		}
		await vmListStore.load();
	}

	onMount(() => {
		void loadPage();
		offTaskOk = getTaskTrayContext().onTaskOk(() => {
			void vmListStore.load();
			vmBulk.clearResult();
		});
	});
	onDestroy(() => {
		offTaskOk?.();
	});
</script>

<svelte:head>
	<title>My VMs — PVMSS</title>
</svelte:head>

<section class="mx-auto w-full max-w-5xl px-4 py-8">
	<div class="mb-4 flex flex-wrap items-center justify-between gap-3">
		<h1 class="text-2xl font-semibold tracking-tight">My VMs</h1>
		<ClusterSelector options={clusterOptions} value={vmListStore.cluster} onChange={(value) => vmListStore.setCluster(value)} includeAll id="vm-cluster-filter" />
		<a
			href={resolve('/vms/create')}
			class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground"
		>
			Create a VM
		</a>
	</div>

	{#if vmListStore.loading && vmListStore.result === null}
		<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
	{:else}
		<VmBulkActionBar />
		<VmList />
	{/if}
</section>

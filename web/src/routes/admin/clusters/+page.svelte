<script lang="ts">
	import { onMount } from 'svelte';
	import ClustersPage from '$lib/features/admin-clusters/ClustersPage.svelte';
	import { AdminClustersStore } from '$lib/features/admin-clusters/clusters.svelte';

	const store = new AdminClustersStore();

	onMount(() => {
		void store.load();
	});
</script>

<svelte:head>
	<title>Admin — Clusters — PVMSS</title>
</svelte:head>

<ClustersPage {store} />

<script lang="ts">
	import { onMount } from 'svelte';
	import ClustersPage from '$lib/features/admin-clusters/ClustersPage.svelte';
	import { AdminClustersStore } from '$lib/features/admin-clusters/clusters.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = new AdminClustersStore();

	onMount(() => {
		void store.load();
	});
</script>

<svelte:head>
	<title>{m['admin.clusters.title']()}</title>
</svelte:head>

<ClustersPage {store} />

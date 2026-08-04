<script lang="ts">
	import { onMount } from 'svelte';
	import { setNodesContext } from '$lib/features/cluster/nodes.svelte';
	import NodeList from '$lib/features/cluster/NodeList.svelte';

	const nodesStore = setNodesContext();

	onMount(() => {
		void nodesStore.load();
	});
</script>

<svelte:head>
	<title>Nodes — PVMSS</title>
</svelte:head>

<section class="mx-auto w-full max-w-4xl px-4 py-8">
	<h1 class="mb-4 text-2xl font-semibold tracking-tight">Cluster nodes</h1>

	{#if nodesStore.loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
	{:else if nodesStore.error}
		<p role="alert" class="text-destructive">{nodesStore.error}</p>
	{:else}
		<div role="status" aria-live="polite" class="sr-only">
			{nodesStore.nodes.length} nodes loaded
		</div>
		<NodeList nodes={nodesStore.nodes} />
	{/if}
</section>

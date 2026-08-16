<script lang="ts">
	import { onMount } from 'svelte';
	import { setNodesContext } from '$lib/features/cluster/nodes.svelte';
	import AuthRequired from '$lib/features/auth/AuthRequired.svelte';
	import NodeList from '$lib/features/cluster/NodeList.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const nodesStore = setNodesContext();

	onMount(() => {
		void nodesStore.load();
	});

	function handleRefresh(): void {
		void nodesStore.refresh();
	}
</script>

<svelte:head>
	<title>{m['nodes.title']()}</title>
</svelte:head>

{#if nodesStore.errorCode === 'unauthenticated'}
	<AuthRequired />
{:else}
	<section class="mx-auto w-full max-w-4xl px-4 py-8">
		<h1 class="mb-4 text-2xl font-semibold tracking-tight">{m['nodes.heading']()}</h1>

		{#if nodesStore.loading}
			<p role="status" aria-live="polite" class="text-muted-foreground">{m['common.loading']()}</p>
		{:else if nodesStore.error}
			<p role="alert" class="text-destructive">{nodesStore.error}</p>
		{:else}
			<div role="status" aria-live="polite" class="sr-only">
				{m['nodes.nodesLoaded']({ count: nodesStore.nodes.length })}
			</div>
			<NodeList
				nodes={nodesStore.nodes}
				refreshedAt={nodesStore.refreshedAt}
				refreshing={nodesStore.refreshing}
				refreshDisabled={nodesStore.refreshDisabled}
				refreshError={nodesStore.refreshError}
				onRefresh={handleRefresh}
			/>
		{/if}
	</section>
{/if}

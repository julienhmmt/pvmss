<script lang="ts">
	import { page } from '$app/state';
	import ClusterDetailPage from '#lib/features/admin-clusters/ClusterDetailPage.svelte';
	import { m } from '#lib/paraglide/messages.js';

	const name: string = $derived(page.params.name ?? '');
</script>

<svelte:head>
	<title>{m['admin.clusterDetail.pageTitle']({ name })}</title>
</svelte:head>

{#key name}
	<ClusterDetailPage {name} />
{/key}

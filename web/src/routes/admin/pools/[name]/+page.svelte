<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import PoolDetailPage from '$lib/features/admin-pools/PoolDetailPage.svelte';
	import { setAdminPoolDetailContext } from '$lib/features/admin-pools/pool-detail.svelte';

	const store = setAdminPoolDetailContext();
	const poolName: string = $derived(page.params.name ?? '');
	const clusterHint: string = $derived(page.url.searchParams.get('cluster') ?? '');

	onMount(() => {
		void store.load(poolName, clusterHint);
	});
</script>

<PoolDetailPage
	detail={store.detail}
	loading={store.loading}
	error={store.error}
	notFound={store.notFound}
	deleting={store.deleting}
	deleteError={store.deleteError}
	onRefresh={() => void store.refresh()}
	onDelete={() => store.remove()}
	onDeleted={() => void goto(resolve('/admin/pools'))}
/>

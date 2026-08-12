<script lang="ts">
	import { onMount } from 'svelte';
	import PoolsPage from '$lib/features/admin-pools/PoolsPage.svelte';
	import { setAdminPoolsContext } from '$lib/features/admin-pools/pools.svelte';

	const store = setAdminPoolsContext();

	onMount(() => {
		void store.load();
	});
</script>

<PoolsPage
	pools={store.pools}
	loading={store.loading}
	error={store.error}
	saving={store.saving}
	saveError={store.saveError}
	deleting={store.deleting}
	deleteError={store.deleteError}
	announce={store.announce}
	onSearch={(value) => store.applySearch(value)}
	onCreate={(name, password, comment) => store.create(name, password, comment)}
	onDelete={(name) => store.remove(name)}
/>

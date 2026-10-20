<script lang="ts">
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import PoolsPage from '#lib/features/admin-pools/PoolsPage.svelte';
	import { setAdminPoolsContext } from '#lib/features/admin-pools/pools.svelte.js';

	const store = setAdminPoolsContext();

	onMount(() => {
		// Set by the detail page when the pool was deleted but its Proxmox account was not.
		const failed = page.url.searchParams.get('userDeleteFailed');
		if (failed) store.warnUserDeleteFailed(failed);
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
	userDeleteWarning={store.userDeleteWarning}
	onDismissUserDeleteWarning={() => store.dismissUserDeleteWarning()}
	credentials={store.createdCredentials}
	clusterOptions={store.clusterOptions}
	cluster={store.cluster}
	onClusterChange={(v) => store.setCluster(v)}
	onSearch={(value) => store.applySearch(value)}
	onCreate={(name, comment) => store.create(name, comment)}
	onDelete={(name) => store.remove(name)}
	onDismissCredentials={() => store.dismissCredentials()}
/>

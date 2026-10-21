<script lang="ts">
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { replaceState } from '$app/navigation';
	import PoolsPage from '#lib/features/admin-pools/PoolsPage.svelte';
	import { setAdminPoolsContext } from '#lib/features/admin-pools/pools.svelte.js';

	const store = setAdminPoolsContext();
	const startCreating = page.url.searchParams.has('create');

	onMount(() => {
		// Set by the detail page when the pool was deleted but its Proxmox account was not.
		const failed = page.url.searchParams.get('userDeleteFailed');
		if (failed) store.warnUserDeleteFailed(failed);
		// One-shot: Back from a pool page must not reopen the dialog.
		if (startCreating) replaceState(page.url.pathname, page.state);
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
	{startCreating}
/>

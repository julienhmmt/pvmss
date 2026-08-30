<script lang="ts">
	import { onMount } from 'svelte';
	import PolicyNodesPage from '$lib/features/admin-policy-nodes/PolicyNodesPage.svelte';
	import { setAdminPolicyNodesContext } from '$lib/features/admin-policy-nodes/policyNodes.svelte';

	const store = setAdminPolicyNodesContext();

	onMount(() => {
		void store.load();
	});
</script>

<PolicyNodesPage
	nodes={store.sortedNodes}
	loading={store.loading}
	error={store.error}
	errorCode={store.errorCode}
	saving={store.saving}
	saveError={store.saveError}
	clusterOptions={store.clusterOptions}
	cluster={store.cluster}
	onClusterChange={(v) => store.setCluster(v)}
	onLoad={() => void store.load()}
	onRetry={() => void store.retryConnection()}
	onSave={(node, patch) => void store.save(node, patch)}
	sortBy={store.sortBy}
	sortDir={store.sortDir}
	onSort={(column) => store.setSort(column)}
/>

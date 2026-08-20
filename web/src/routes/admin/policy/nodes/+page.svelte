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
	saving={store.saving}
	saveError={store.saveError}
	onLoad={() => void store.load()}
	onSave={(node, patch) => void store.save(node, patch)}
	sortBy={store.sortBy}
	sortDir={store.sortDir}
	onSort={(column) => store.setSort(column)}
/>

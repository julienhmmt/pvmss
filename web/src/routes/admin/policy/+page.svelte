<script lang="ts">
	import { onMount } from 'svelte';
	import PolicyPage from '$lib/features/admin-policy/PolicyPage.svelte';
	import { setAdminPolicyContext } from '$lib/features/admin-policy/policy.svelte';

	const store = setAdminPolicyContext();

	onMount(() => {
		void store.load();
	});
</script>

<PolicyPage
	policy={store.policy}
	loading={store.loading}
	error={store.error}
	saving={store.saving}
	saveError={store.saveError}
	saved={store.saved}
	clusterOptions={store.clusterOptions}
	cluster={store.cluster}
	onClusterChange={(v) => store.setCluster(v)}
	onLoad={() => void store.load()}
	onSave={(patch) => void store.save(patch)}
/>

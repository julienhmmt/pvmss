<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import PolicyPage from '$lib/features/admin-policy/PolicyPage.svelte';
	import { setAdminPolicyContext } from '$lib/features/admin-policy/policy.svelte';
	import { saveAdminCluster } from '$lib/shared/clusters';
	import { m } from '$lib/paraglide/messages.js';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';

	const store = setAdminPolicyContext();
	const toast = getToastContext();

	onMount(() => {
		// Deep links from dashboard alerts carry a ?cluster= hint; assigning it
		// before load keeps the first (and only) fetch.
		const clusterHint = page.url.searchParams.get('cluster');
		if (clusterHint) {
			saveAdminCluster(clusterHint);
			store.cluster = clusterHint;
		}
		void store.load();
	});
</script>

<PolicyPage
	policy={store.policy}
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
	// One success toast per save (autosave or manual). Failures show inline via store.saveError.
	onSave={(patch) => store.save(patch).then(() => toast.success(m['policy.saved']()), () => undefined)}
/>

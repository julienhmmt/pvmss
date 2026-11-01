<script lang="ts">
	import { onMount } from 'svelte';
	import RecoveryPage from '#lib/features/admin-recovery/RecoveryPage.svelte';
	import { setAdminRecoveryContext } from '#lib/features/admin-recovery/recovery.svelte.js';

	const store = setAdminRecoveryContext();

	onMount(() => {
		void store.load();
	});
</script>

<RecoveryPage
	report={store.report}
	loading={store.loading}
	rescanning={store.rescanning}
	error={store.error}
	adopting={store.adopting}
	adoptError={store.adoptError}
	announce={store.announce}
	clusterOptions={store.clusterOptions}
	cluster={store.cluster}
	selectedPools={store.selectedPools}
	selectedTags={store.selectedTags}
	onClusterChange={(v) => store.setCluster(v)}
	onRescan={() => store.rescan()}
	onTogglePool={(name) => store.togglePool(name)}
	onToggleTag={(name) => store.toggleTag(name)}
	onAdopt={() => store.adopt()}
/>

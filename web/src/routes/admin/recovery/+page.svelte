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
	pools={store.filteredPools}
	tags={store.filteredTags}
	looseVMs={store.filteredLooseVMs}
	search={store.search}
	allPoolsSelected={store.allPoolsSelected}
	allTagsSelected={store.allTagsSelected}
	allVMsSelected={store.allVMsSelected}
	loading={store.loading}
	rescanning={store.rescanning}
	error={store.error}
	adopting={store.adopting}
	adoptError={store.adoptError}
	hiding={store.hiding}
	hideError={store.hideError}
	announce={store.announce}
	clusterOptions={store.clusterOptions}
	cluster={store.cluster}
	selectedPools={store.selectedPools}
	selectedTags={store.selectedTags}
	selectedVMs={store.selectedVMs}
	onClusterChange={(v) => store.setCluster(v)}
	onRescan={() => store.rescan()}
	onSearchChange={(v) => (store.search = v)}
	onToggleAllPools={() => store.toggleAllPools()}
	onToggleAllTags={() => store.toggleAllTags()}
	onToggleAllVMs={() => store.toggleAllVMs()}
	onTogglePool={(name) => store.togglePool(name)}
	onToggleTag={(name) => store.toggleTag(name)}
	onToggleVM={(vmid) => store.toggleVM(vmid)}
	onAdopt={() => store.adopt()}
	onHideSelected={() => store.hideSelected()}
	onHide={(items) => store.hideItems(items)}
	onUnhide={(items) => store.unhideItems(items)}
	onAdoptHidden={(item) => store.adoptHiddenItem(item)}
/>

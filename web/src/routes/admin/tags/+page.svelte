<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminTagsContext } from '#lib/features/admin-tags/tags.svelte.js';
	import TagsPage from '#lib/features/admin-tags/TagsPage.svelte';

	const store = setAdminTagsContext();

	onMount(() => {
		void store.load();
	});
</script>

<TagsPage
	tags={store.tags}
	filteredTags={store.filteredTags}
	loading={store.loading}
	error={store.error}
	saving={store.saving}
	saveError={store.saveError}
	clusterOptions={store.clusterOptions}
	cluster={store.cluster}
	onClusterChange={(v) => store.setCluster(v)}
	search={store.search}
	sortBy={store.sortBy}
	sortDir={store.sortDir}
	onSearchChange={(v) => (store.search = v)}
	onSort={(column) => store.setSort(column)}
	onResetFilters={() => store.resetFilters()}
	onCreate={(name, color) => store.create(name, color)}
	onUpdateColor={(name, color) => store.updateColor(name, color)}
	onDelete={(name) => store.remove(name)}
	onClearSaveError={() => (store.saveError = null)}
/>

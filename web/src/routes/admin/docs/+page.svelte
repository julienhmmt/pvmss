<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminDocsContext } from '$lib/features/admin-docs/docs.svelte';
	import DocsAdminPage from '$lib/features/admin-docs/DocsAdminPage.svelte';

	const store = setAdminDocsContext();

	onMount(() => {
		void store.load();
	});
</script>

<DocsAdminPage
	pages={store.pages}
	filteredPages={store.filteredPages}
	loading={store.loading}
	error={store.error}
	saving={store.saving}
	saveError={store.saveError}
	search={store.search}
	categoryFilter={store.categoryFilter}
	langFilter={store.langFilter}
	audienceFilter={store.audienceFilter}
	categoryOptions={store.categoryOptions}
	langOptions={store.langOptions}
	sortBy={store.sortBy}
	sortDir={store.sortDir}
	onSearchChange={(v) => (store.search = v)}
	onCategoryFilterChange={(v) => (store.categoryFilter = v)}
	onLangFilterChange={(v) => (store.langFilter = v)}
	onAudienceFilterChange={(v) => (store.audienceFilter = v)}
	onSort={(column) => store.setSort(column)}
	onResetFilters={() => store.resetFilters()}
	onCreate={(input) => store.create(input)}
	onUpdate={(id, lang, input) => store.update(id, lang, input)}
	onDelete={(id, lang) => void store.remove(id, lang)}
	onToggle={(id, lang, enabled) => void store.toggle(id, lang, enabled)}
/>

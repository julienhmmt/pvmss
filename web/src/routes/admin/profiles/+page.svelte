<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminProfilesContext } from '$lib/features/admin-profiles/profiles.svelte';
	import ProfilesPage from '$lib/features/admin-profiles/ProfilesPage.svelte';

	const store = setAdminProfilesContext();

	onMount(() => {
		void store.load();
	});
</script>

<ProfilesPage
	profiles={store.profiles}
	filteredProfiles={store.filteredProfiles}
	loading={store.loading}
	error={store.error}
	saving={store.saving}
	saveError={store.saveError}
	search={store.search}
	busFilter={store.busFilter}
	enabledFilter={store.enabledFilter}
	busOptions={store.busOptions}
	sortBy={store.sortBy}
	sortDir={store.sortDir}
	onSearchChange={(v) => (store.search = v)}
	onBusFilterChange={(v) => (store.busFilter = v)}
	onEnabledFilterChange={(v) => (store.enabledFilter = v)}
	onSort={(column) => store.setSort(column)}
	onResetFilters={() => store.resetFilters()}
	onCreate={(label, cpuCores, memoryMB, diskGB, bus) => void store.create(label, cpuCores, memoryMB, diskGB, bus)}
	onUpdate={(id, label, cpuCores, memoryMB, diskGB, bus) => void store.update(id, label, cpuCores, memoryMB, diskGB, bus)}
	onDelete={(id) => void store.remove(id)}
	onToggle={(id, enabled) => void store.toggle(id, enabled)}
/>

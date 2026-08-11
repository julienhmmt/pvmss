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
	loading={store.loading}
	error={store.error}
	saving={store.saving}
	saveError={store.saveError}
	onCreate={(label, cpuCores, memoryMB, diskGB, bus) => void store.create(label, cpuCores, memoryMB, diskGB, bus)}
	onUpdate={(id, label, cpuCores, memoryMB, diskGB, bus) => void store.update(id, label, cpuCores, memoryMB, diskGB, bus)}
	onDelete={(id) => void store.remove(id)}
	onToggle={(id, enabled) => void store.toggle(id, enabled)}
/>

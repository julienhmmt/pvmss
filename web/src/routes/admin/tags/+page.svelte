<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminTagsContext } from '$lib/features/admin-tags/tags.svelte';
	import TagsPage from '$lib/features/admin-tags/TagsPage.svelte';

	const store = setAdminTagsContext();

	onMount(() => {
		void store.load();
	});
</script>

<TagsPage
	tags={store.tags}
	loading={store.loading}
	error={store.error}
	saving={store.saving}
	saveError={store.saveError}
	onCreate={(name, color) => void store.create(name, color)}
	onUpdateColor={(name, color) => void store.updateColor(name, color)}
	onDelete={(name) => void store.remove(name)}
/>

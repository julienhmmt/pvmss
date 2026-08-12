<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminCloudInitTemplatesContext } from '$lib/features/admin-cloudinit-templates/cloudInitTemplates.svelte';
	import CloudInitTemplatesPage from '$lib/features/admin-cloudinit-templates/CloudInitTemplatesPage.svelte';

	const store = setAdminCloudInitTemplatesContext();

	onMount(() => {
		void store.load();
	});
</script>

<CloudInitTemplatesPage
	templates={store.templates}
	loading={store.loading}
	error={store.error}
	saving={store.saving}
	saveError={store.saveError}
	onCreate={(label, content) => void store.create(label, content)}
	onUpdate={(id, label, content) => void store.update(id, label, content)}
	onDelete={(id) => void store.remove(id)}
	onToggle={(id, enabled) => void store.toggle(id, enabled)}
/>

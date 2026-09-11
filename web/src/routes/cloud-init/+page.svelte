<script lang="ts">
	import { onMount } from 'svelte';
	import { setCloudInitFilesContext } from '$lib/features/cloudinit-files/cloudInitFiles.svelte';
	import CloudInitFilesPage from '$lib/features/cloudinit-files/CloudInitFilesPage.svelte';

	const store = setCloudInitFilesContext();

	onMount(() => {
		void store.load();
	});
</script>

<CloudInitFilesPage
	files={store.files}
	loading={store.loading}
	error={store.error}
	saving={store.saving}
	saveError={store.saveError}
	getFile={(id) => store.getFile(id)}
	onCreate={(label, content) => void store.create(label, content)}
	onUpdate={(id, label, content) => void store.update(id, label, content)}
	onDelete={(id) => void store.remove(id)}
/>

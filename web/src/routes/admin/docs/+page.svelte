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
	loading={store.loading}
	error={store.error}
	saving={store.saving}
	saveError={store.saveError}
	onCreate={(input) => store.create(input)}
	onUpdate={(id, lang, input) => store.update(id, lang, input)}
	onDelete={(id, lang) => void store.remove(id, lang)}
	onToggle={(id, lang, enabled) => void store.toggle(id, lang, enabled)}
/>

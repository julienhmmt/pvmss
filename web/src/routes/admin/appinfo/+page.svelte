<script lang="ts">
	import { onMount } from 'svelte';
	import { setAppInfoContext } from '$lib/features/admin-appinfo/appinfo.svelte';
	import AppInfoPage from '$lib/features/admin-appinfo/AppInfoPage.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = setAppInfoContext();

	onMount(() => {
		void store.load();
	});
</script>

<svelte:head>
	<title>{m['admin.appinfo.title']()}</title>
</svelte:head>

<AppInfoPage />

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

<section class="mx-auto w-full max-w-4xl px-4 py-8">
	<AppInfoPage />
</section>
